package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/src/ai"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/watcher"
	"mogenius-k8s-manager/src/websocket"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/kubectl/pkg/describe"
	"sigs.k8s.io/yaml"
)

const (
	VALKEY_RESOURCE_PREFIX = "resources"
)

type GetUnstructuredNamespaceResourceListRequest struct {
	Namespace string                      `json:"namespace" validate:"required"`
	Whitelist []*utils.ResourceDescriptor `json:"whitelist"`
	Blacklist []*utils.ResourceDescriptor `json:"blacklist"`
}

type GetUnstructuredLabeledResourceListRequest struct {
	Label     string                      `json:"label" validate:"required"`
	Whitelist []*utils.ResourceDescriptor `json:"whitelist"`
	Blacklist []*utils.ResourceDescriptor `json:"blacklist"`
}

var lastWatchCheckStart time.Time = time.Time{}

func WatchStoreResources(wm watcher.WatcherModule, aiManager ai.AiManager, eventClient websocket.WebsocketClient) error {
	start := time.Now()

	// function should not be called more often than every 5 seconds
	// to avoid too many calls to the k8s api server
	// which can lead to rate limiting
	if time.Since(lastWatchCheckStart) < 5*time.Second {
		return nil
	}
	lastWatchCheckStart = time.Now()

	resources, err := GetAvailableResources()
	if err != nil {
		return err
	}
	for _, v := range resources {
		err := wm.Watch(watcher.WatcherResourceIdentifier{
			Plural:     v.Plural,
			Kind:       v.Kind,
			ApiVersion: v.ApiVersion,
			Namespaced: v.Namespaced,
		}, func(resource watcher.WatcherResourceIdentifier, obj *unstructured.Unstructured) {
			handleCRDAddition(wm, aiManager, eventClient, resource)
			setStoreIfNeeded(resource.ApiVersion, obj.GetName(), resource.Kind, obj.GetNamespace(), obj)

			// suppress the add events for the first 10 seconds (because all resources are added initially)
			if time.Since(start) < 10*time.Second {
				return
			}
			sendEventServerEvent(eventClient, v.ApiVersion, resource.Kind, obj.GetName(), "add", obj)
		}, func(resource watcher.WatcherResourceIdentifier, oldObj, newObj *unstructured.Unstructured) {
			setStoreIfNeeded(resource.ApiVersion, newObj.GetName(), resource.Kind, newObj.GetNamespace(), newObj)

			// Filter out resync updates - same resource version means no actual change
			if oldObj.GetResourceVersion() == newObj.GetResourceVersion() {
				return
			}
			sendEventServerEvent(eventClient, v.ApiVersion, resource.Kind, newObj.GetName(), "update", newObj)
			aiManager.ProcessObject(newObj, "update")
		}, func(resource watcher.WatcherResourceIdentifier, obj *unstructured.Unstructured) {
			deleteFromStoreIfNeeded(resource.ApiVersion, obj.GetName(), resource.Kind, obj.GetNamespace(), obj)
			sendEventServerEvent(eventClient, v.ApiVersion, resource.Kind, obj.GetName(), "delete", obj)
			handleCRDDeletion(wm, resource, obj)
			aiManager.ProcessObject(obj, "delete")
		})
		if err != nil {
			if !strings.Contains(err.Error(), "resource is already being watched") {
				k8sLogger.Error("failed to initialize watchhandler for resource", "ApiVersion", v.ApiVersion, "kind", v.Kind, "error", err)
				return err
			}
		} else {
			k8sLogger.Info("🚀 Watching resource", "kind", v.Kind, "plural", v.Plural)
		}
	}
	return nil
}

var (
	crdDebounceTimer *time.Timer
	crdDebounceMutex sync.Mutex
)

// no matter how many CRD addition events we get in a short time frame
// this method will debounce them and only execute the logic once after 3 seconds
func handleCRDAddition(wm watcher.WatcherModule, aiManager ai.AiManager, eventClient websocket.WebsocketClient, resource watcher.WatcherResourceIdentifier) {
	if resource.Kind == "CustomResourceDefinition" {
		crdDebounceMutex.Lock()
		defer crdDebounceMutex.Unlock()

		// Cancel existing timer if it exists
		if crdDebounceTimer != nil {
			crdDebounceTimer.Stop()
		}

		// Create new timer that executes after 3 seconds
		crdDebounceTimer = time.AfterFunc(3*time.Second, func() {
			resetAvailableResourceCache()

			res, err := GetAvailableResources()
			if err != nil {
				k8sLogger.Error("Error getting available resources", "error", err)
				return
			}
			currentlyWatchedResources := wm.ListWatchedResources()
			if len(res) != len(currentlyWatchedResources) {
				err := WatchStoreResources(wm, aiManager, eventClient)
				if err != nil {
					k8sLogger.Error("Error watching store resources", "error", err)
				}
			}
		})
	}
}

func handleCRDDeletion(wm watcher.WatcherModule, resource watcher.WatcherResourceIdentifier, obj *unstructured.Unstructured) {
	if resource.Kind == "CustomResourceDefinition" {
		name, _, _ := unstructured.NestedString(obj.Object, "spec", "names", "plural")
		kind, _, _ := unstructured.NestedString(obj.Object, "spec", "names", "kind")

		if name == "" || kind == "" {
			k8sLogger.Error("Error parsing CRD for unwatching", "name", name, "kind", kind)
			return
		}
		resourceToDelete := watcher.WatcherResourceIdentifier{
			Plural:     resource.Plural,
			Kind:       resource.Kind,
			ApiVersion: resource.ApiVersion,
			Namespaced: resource.Namespaced,
		}
		err := wm.Unwatch(resourceToDelete)
		if err != nil {
			k8sLogger.Error("Error unwatching resource", "name", obj.GetName(), "error", err)
		} else {
			k8sLogger.Info("STOP Watching resource", "kind", obj.GetKind(), "name", obj.GetName())
		}
	}
}

func setStoreIfNeeded(apiVersion string, resourceName string, kind string, namespace string, obj *unstructured.Unstructured) {
	obj = removeUnusedFieds(obj)

	// store in valkey
	err := valkeyClient.SetObject(obj, utils.ResourceResyncTime*2, VALKEY_RESOURCE_PREFIX, apiVersion, kind, namespace, resourceName)
	if err != nil {
		k8sLogger.Error("Error setting object in store", "error", err)
	}
}

func sendEventServerEvent(eventClient websocket.WebsocketClient, apiVersion, kind, name, eventType string, obj *unstructured.Unstructured) {
	datagram := structs.CreateDatagramForClusterEvent("ClusterEvent", apiVersion, kind, name, eventType, obj)

	// send the datagram to the event server
	go func() {
		err := eventClient.WriteJSON(datagram)
		if err != nil {
			k8sLogger.Error("Error sending data to EventServer", "error", err)

		}
	}()
}

func deleteFromStoreIfNeeded(apiVersion string, resourceName string, kind string, namespace string, obj *unstructured.Unstructured) {
	if kind == "PersistentVolume" {
		var pv v1.PersistentVolume
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &pv)
		if err != nil {
			k8sLogger.Error("Error cannot cast from unstructured", "error", err)
			return
		}
		handlePVDeletion(&pv)
	}

	// other resources
	err := valkeyClient.DeleteSingle(VALKEY_RESOURCE_PREFIX, apiVersion, kind, namespace, resourceName)
	if err != nil {
		k8sLogger.Error("Error deleting object in store", "error", err)
	}
}

func GetUnstructuredResourceList(plural string, apiVersion string, namespace *string) (*unstructured.UnstructuredList, error) {
	dynamicClient := clientProvider.DynamicClient()
	resource := CreateGroupVersionResource(apiVersion, plural)

	var result *unstructured.UnstructuredList
	var err error
	if namespace == nil {
		result, err = dynamicClient.Resource(resource).List(context.Background(), metav1.ListOptions{})
	} else {
		result, err = dynamicClient.Resource(resource).Namespace(*namespace).List(context.Background(), metav1.ListOptions{})
	}

	result = removeManagedFieldsFromList(result)
	return result, err
}

func GetUnstructuredResourceListFromStore(apiVersion string, kind string, namespace *string, withData *bool) unstructured.UnstructuredList {
	selectedNamespace := ""
	if namespace != nil {
		selectedNamespace = *namespace
	}

	// try to get the data from the store (very fast)
	results := unstructured.UnstructuredList{}
	result := store.GetResourceByKindAndNamespace(valkeyClient, apiVersion, kind, selectedNamespace, k8sLogger)
	if result != nil {
		// delete data field to speed up transfer
		if withData == nil || !*withData {
			for i := range result {
				delete(result[i].Object, "data")
			}
		}
		results.Items = result
	}

	return results
}

func GetUnstructuredNamespaceResourceList(namespace string, whitelist []*utils.ResourceDescriptor, blacklist []*utils.ResourceDescriptor) ([]unstructured.Unstructured, error) {
	results := []unstructured.Unstructured{}
	resultsMutex := sync.Mutex{}

	resources, err := GetAvailableResources()
	if err != nil {
		return results, err
	}

	if whitelist == nil {
		whitelist = []*utils.ResourceDescriptor{}
	}

	if blacklist == nil {
		blacklist = []*utils.ResourceDescriptor{}
	}

	var wg sync.WaitGroup
	for _, v := range resources {
		if v.Namespaced {
			if len(whitelist) > 0 && !utils.ContainsResourceDescriptor(whitelist, v) {
				continue
			}
			if blacklist != nil && utils.ContainsResourceDescriptor(blacklist, v) {
				continue
			}
			wg.Go(func() {
				result := store.GetResourceByKindAndNamespace(valkeyClient, v.ApiVersion, v.Kind, namespace, k8sLogger)
				resultsMutex.Lock()
				results = append(results, result...)
				resultsMutex.Unlock()
			})
		}
	}
	wg.Wait()

	return results, nil
}

func GetUnstructuredLabeledResourceList(label string, whitelist []*utils.ResourceDescriptor, blacklist []*utils.ResourceDescriptor) (unstructured.UnstructuredList, error) {
	results := unstructured.UnstructuredList{
		Object: map[string]any{},
		Items:  []unstructured.Unstructured{},
	}

	resources, err := GetAvailableResources()
	if err != nil {
		return results, err
	}

	if whitelist == nil {
		whitelist = []*utils.ResourceDescriptor{}
	}

	if blacklist == nil {
		blacklist = []*utils.ResourceDescriptor{}
	}

	dynamicClient := clientProvider.DynamicClient()
	for _, v := range resources {
		if v.Namespaced {
			if (len(whitelist) > 0 && !utils.ContainsResourceDescriptor(whitelist, v)) || (blacklist != nil && utils.ContainsResourceDescriptor(blacklist, v)) {
				continue
			}

			result, err := dynamicClient.Resource(CreateGroupVersionResource(v.ApiVersion, v.Plural)).List(context.Background(), metav1.ListOptions{LabelSelector: label})

			if err != nil {
				if !os.IsNotExist(err) {
					k8sLogger.Error("Error querying resource", "error", err)
				}
				continue
			}
			// result := store.GetResourceByKindAndNamespace(v.Group, v.Kind, namespace)
			if result != nil {
				results.Items = append(results.Items, result.Items...)
			}
		}
	}
	return results, nil
}

func GetUnstructuredResource(apiVersion string, plural string, namespace, resourceName string) (*unstructured.Unstructured, error) {
	dynamicClient := clientProvider.DynamicClient()
	if namespace != "" {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Namespace(namespace).Get(context.Background(), resourceName, metav1.GetOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Get(context.Background(), resourceName, metav1.GetOptions{})
		return removeManagedFields(result), err
	}
}

func GetUnstructuredResourceFromStore(apiVersion string, kind string, namespace, resourceName string) (*unstructured.Unstructured, error) {
	return store.GetResource(valkeyClient, apiVersion, kind, namespace, resourceName, k8sLogger)
}

func CreateUnstructuredResource(apiVersion string, plural string, namespaced bool, yamlData string) (*unstructured.Unstructured, error) {
	dynamicClient := clientProvider.DynamicClient()
	obj := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(yamlData), obj)
	if err != nil {
		return nil, err
	}

	if namespaced {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Namespace(obj.GetNamespace()).Create(context.Background(), obj, metav1.CreateOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Create(context.Background(), obj, metav1.CreateOptions{})
		return removeManagedFields(result), err
	}
}

func UpdateUnstructuredResource(apiVersion string, plural string, namespaced bool, yamlData string) (*unstructured.Unstructured, error) {
	dynamicClient := clientProvider.DynamicClient()
	obj := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(yamlData), obj)
	if err != nil {
		return nil, err
	}

	if namespaced {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Namespace(obj.GetNamespace()).Update(context.Background(), obj, metav1.UpdateOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Update(context.Background(), obj, metav1.UpdateOptions{})
		return removeManagedFields(result), err
	}
}

func DeleteUnstructuredResource(apiVersion string, plural string, namespace string, resourceName string) error {
	dynamicClient := clientProvider.DynamicClient()
	if namespace != "" {
		return dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Namespace(namespace).Delete(context.Background(), resourceName, metav1.DeleteOptions{})
	} else {
		return dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Delete(context.Background(), resourceName, metav1.DeleteOptions{})
	}
}

func DescribeUnstructuredResource(apiVersion string, plural string, namespace, resourceName string) (string, error) {
	config := clientProvider.ClientConfig()

	restMapping := &meta.RESTMapping{
		Resource: CreateGroupVersionResource(apiVersion, plural),
	}

	describer, ok := describe.GenericDescriberFor(restMapping, config)
	if !ok {
		return "", fmt.Errorf("failed to get describer")

	}

	output, err := describer.Describe(namespace, resourceName, describe.DescriberSettings{ShowEvents: true})
	if err != nil {
		fmt.Printf("Failed to describe resource: %v\n", err)
		return "", err
	}

	return output, nil
}

func TriggerUnstructuredResource(apiVersion string, plural string, namespace string, resourceName string) (*unstructured.Unstructured, error) {
	dynamicClient := clientProvider.DynamicClient()

	if plural == "cronjobs" || plural == "jobs" {
		job, err := GetUnstructuredResource(apiVersion, plural, namespace, resourceName)
		if err != nil {
			return nil, err
		}

		// cleanup
		unstructured.RemoveNestedField(job.Object, "metadata", "uid")
		unstructured.RemoveNestedField(job.Object, "metadata", "resourceVersion")
		unstructured.RemoveNestedField(job.Object, "metadata", "creationTimestamp")
		unstructured.RemoveNestedField(job.Object, "metadata", "labels", "controller-uid")
		unstructured.RemoveNestedField(job.Object, "metadata", "labels", "batch.kubernetes.io/controller-uid")
		unstructured.RemoveNestedField(job.Object, "metadata", "labels", "batch.kubernetes.io/job-name")
		unstructured.RemoveNestedField(job.Object, "spec", "selector")
		unstructured.RemoveNestedField(job.Object, "spec", "template", "metadata", "labels", "controller-uid")
		unstructured.RemoveNestedField(job.Object, "spec", "template", "metadata", "labels", "job-name")
		unstructured.RemoveNestedField(job.Object, "spec", "template", "metadata", "labels", "batch.kubernetes.io/controller-uid")
		unstructured.RemoveNestedField(job.Object, "spec", "template", "metadata", "labels", "batch.kubernetes.io/job-name")
		unstructured.RemoveNestedField(job.Object, "status")

		// replace
		jobname := job.GetName() + "-" + utils.NanoIdSmallLowerCase()
		job.SetName(jobname)
		job.SetKind("Job")
		if plural == "cronjobs" {
			template, _, err := unstructured.NestedMap(job.Object, "spec", "jobTemplate", "spec", "template")
			if err != nil {
				return nil, fmt.Errorf("field jobTemplate not found")
			}
			_ = unstructured.SetNestedField(job.Object, template, "spec", "template")
			plural = "jobs"
		}

		return dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Namespace(namespace).Create(context.Background(), job, metav1.CreateOptions{})
	}
	return nil, fmt.Errorf("%s is a invalid resource for trigger. Only jobs or cronjobs can be triggert", plural)
}

type availableResourceCacheEntry struct {
	timestamp          time.Time
	availableResources []utils.ResourceDescriptor
}

var (
	resourceCache      availableResourceCacheEntry
	resourceCacheMutex sync.Mutex        // Mutex to ensure concurrent safe access to cache
	resourceCacheTTL   = 1 * time.Minute // Cache duration
)

func GetAvailableResources() ([]utils.ResourceDescriptor, error) {
	// Check if we have cached resources and if they are still valid
	resourceCacheMutex.Lock()
	defer resourceCacheMutex.Unlock()
	if time.Since(resourceCache.timestamp) < resourceCacheTTL {
		return resourceCache.availableResources, nil
	}

	// No valid cache, fetch resources from server
	clientset := clientProvider.K8sClientSet()
	resources, err := clientset.Discovery().ServerPreferredResources()
	if err != nil {
		if discovery.IsGroupDiscoveryFailedError(err) {
			k8sLogger.Error("Failed to discover group resources", "error", err)
		} else {
			k8sLogger.Error("Error discovering resources", "error", err)
			return nil, err
		}
	}

	var availableResources []utils.ResourceDescriptor
	for _, resourceList := range resources {
		for _, resource := range resourceList.APIResources {
			if slices.Contains(resource.Verbs, "list") && slices.Contains(resource.Verbs, "watch") {
				availableResources = append(availableResources, utils.ResourceDescriptor{
					Plural:     resource.Name,
					ApiVersion: resourceList.GroupVersion,
					Kind:       resource.Kind,
					Namespaced: resource.Namespaced,
				})
			}
		}
	}

	// Update the cache with the new data
	resourceCache.availableResources = availableResources
	resourceCache.timestamp = time.Now()

	return availableResources, nil
}

func resetAvailableResourceCache() {
	resourceCacheMutex.Lock()
	defer resourceCacheMutex.Unlock()
	resourceCache = availableResourceCacheEntry{}
}

func GetResourcesNameForKind(kind string) (name string, err error) {
	resources, err := GetAvailableResources()
	if err != nil {
		return "", err
	}

	for _, resource := range resources {
		if resource.Kind == kind {
			return resource.Plural, nil
		}
	}
	return "", fmt.Errorf("resource not found for name %s", name)
}

func CreateGroupVersionResource(apiVersion, plural string) schema.GroupVersionResource {
	gv, err := schema.ParseGroupVersion(apiVersion) // e.g., "apps/v1" or just "v1"
	if err != nil {
		k8sLogger.Error("invalid apiVersion", "apiVersion", apiVersion, "resourceName", plural, "error", err)
	}
	gvr := gv.WithResource(plural)
	return gvr
}

func removeManagedFields(obj *unstructured.Unstructured) *unstructured.Unstructured {
	if obj == nil {
		return obj
	}
	unstructuredContent := obj.Object
	delete(unstructuredContent, "managedFields")
	if unstructuredContent["metadata"] != nil {
		delete(unstructuredContent["metadata"].(map[string]any), "managedFields")
	}
	return obj
}

func removeManagedFieldsFromList(objList *unstructured.UnstructuredList) *unstructured.UnstructuredList {
	if objList == nil {
		return objList
	}
	for i := range objList.Items {
		removeManagedFields(&objList.Items[i])
	}

	return objList
}

func removeUnusedFieds(obj *unstructured.Unstructured) *unstructured.Unstructured {
	obj = removeManagedFields(obj)
	return obj
}
