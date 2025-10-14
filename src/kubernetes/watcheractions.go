package kubernetes

import (
	"context"
	"fmt"
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
	Namespace string                 `json:"namespace" validate:"required"`
	Whitelist []*utils.ResourceEntry `json:"whitelist"`
	Blacklist []*utils.ResourceEntry `json:"blacklist"`
}

type GetUnstructuredLabeledResourceListRequest struct {
	Label     string                 `json:"label" validate:"required"`
	Whitelist []*utils.ResourceEntry `json:"whitelist"`
	Blacklist []*utils.ResourceEntry `json:"blacklist"`
}

var lastWatchCheckStart time.Time = time.Time{}

func WatchStoreResources(wm watcher.WatcherModule, eventClient websocket.WebsocketClient) error {
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
			Name:         v.Name,
			Kind:         v.Kind,
			GroupVersion: v.Group,
		}, func(resource watcher.WatcherResourceIdentifier, obj *unstructured.Unstructured) {
			setStoreIfNeeded(resource.GroupVersion, obj.GetName(), resource.Kind, obj.GetNamespace(), obj)
			handleCRDAddition(wm, eventClient, resource)

			// suppress the add events for the first 10 seconds (because all resources are added initially)
			if time.Since(start) < 10*time.Second {
				return
			}
			sendEventServerEvent(eventClient, v.Group, resource.Version, resource.Kind, resource.Name, "add", obj)
		}, func(resource watcher.WatcherResourceIdentifier, oldObj, newObj *unstructured.Unstructured) {
			setStoreIfNeeded(resource.GroupVersion, newObj.GetName(), resource.Kind, newObj.GetNamespace(), newObj)

			// Filter out resync updates - same resource version means no actual change
			if oldObj.GetResourceVersion() == newObj.GetResourceVersion() {
				return
			}
			sendEventServerEvent(eventClient, v.Group, resource.Version, resource.Kind, resource.Name, "update", newObj)
		}, func(resource watcher.WatcherResourceIdentifier, obj *unstructured.Unstructured) {
			deleteFromStoreIfNeeded(resource.GroupVersion, obj.GetName(), resource.Kind, obj.GetNamespace(), obj)
			sendEventServerEvent(eventClient, v.Group, resource.Version, resource.Kind, resource.Name, "delete", obj)
			handleCRDDeletion(wm, resource, obj)
		})
		if err != nil {
			if !strings.Contains(err.Error(), "resource is already being watched") {
				k8sLogger.Error("failed to initialize watchhandler for resource", "groupVersion", v.Group, "kind", v.Kind, "version", v.Version, "error", err)
				return err
			}
		} else {
			k8sLogger.Info("🚀 Watching resource", "kind", v.Kind, "name", v.Name)
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
func handleCRDAddition(wm watcher.WatcherModule, eventClient websocket.WebsocketClient, resource watcher.WatcherResourceIdentifier) {
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
				err := WatchStoreResources(wm, eventClient)
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
		group, _, _ := unstructured.NestedString(obj.Object, "spec", "group")
		versions, _, _ := unstructured.NestedSlice(obj.Object, "spec", "versions")

		if name == "" || kind == "" || group == "" || len(versions) == 0 {
			k8sLogger.Error("Error parsing CRD for unwatching", "name", name, "kind", kind, "group", group, "versions", versions)
			return
		}
		if firstVersion, ok := versions[0].(map[string]interface{}); ok {
			if versionName, ok := firstVersion["name"].(string); ok {
				resourceToDelete := watcher.WatcherResourceIdentifier{
					Name:         name,
					Kind:         kind,
					GroupVersion: group + "/" + versionName,
				}
				err := wm.Unwatch(resourceToDelete)
				if err != nil {
					k8sLogger.Error("Error unwatching resource", "name", obj.GetName(), "error", err)
				} else {
					k8sLogger.Info("STOP Watching resource", "kind", obj.GetKind(), "name", obj.GetName())
				}
			}
		}
	}
}

func setStoreIfNeeded(groupVersion string, resourceName string, kind string, namespace string, obj *unstructured.Unstructured) {
	obj = removeUnusedFieds(obj)

	// store in valkey
	err := valkeyClient.SetObject(obj, utils.ResourceResyncTime*2, VALKEY_RESOURCE_PREFIX, groupVersion, kind, namespace, resourceName)
	if err != nil {
		k8sLogger.Error("Error setting object in store", "error", err)
	}
}

func sendEventServerEvent(eventClient websocket.WebsocketClient, group, version, kind, name, eventType string, obj *unstructured.Unstructured) {
	datagram := structs.CreateDatagramForClusterEvent("ClusterEvent", group, version, kind, name, eventType, obj)

	// send the datagram to the event server
	go func() {
		err := eventClient.WriteJSON(datagram)
		if err != nil {
			k8sLogger.Error("Error sending data to EventServer", "error", err)

		}
	}()
}

func deleteFromStoreIfNeeded(groupVersion string, resourceName string, kind string, namespace string, obj *unstructured.Unstructured) {
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
	err := valkeyClient.DeleteSingle(VALKEY_RESOURCE_PREFIX, groupVersion, kind, namespace, resourceName)
	if err != nil {
		k8sLogger.Error("Error deleting object in store", "error", err)
	}
}

func GetUnstructuredResourceList(group string, version string, name string, namespace *string) (*unstructured.UnstructuredList, error) {
	dynamicClient := clientProvider.DynamicClient()
	resource := CreateGroupVersionResource(group, version, name)

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

func GetUnstructuredResourceListFromStore(group string, kind string, version string, name string, namespace *string, withData *bool) unstructured.UnstructuredList {
	selectedNamespace := ""
	if namespace != nil {
		selectedNamespace = *namespace
	}

	// try to get the data from the store (very fast)
	results := unstructured.UnstructuredList{}
	result := store.GetResourceByKindAndNamespace(valkeyClient, group, kind, selectedNamespace, k8sLogger)
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

func GetUnstructuredNamespaceResourceList(namespace string, whitelist []*utils.ResourceEntry, blacklist []*utils.ResourceEntry) ([]unstructured.Unstructured, error) {
	results := []unstructured.Unstructured{}
	resultsMutex := sync.Mutex{}

	resources, err := GetAvailableResources()
	if err != nil {
		return results, err
	}

	if whitelist == nil {
		whitelist = []*utils.ResourceEntry{}
	}

	if blacklist == nil {
		blacklist = []*utils.ResourceEntry{}
	}

	var wg sync.WaitGroup
	for _, v := range resources {
		if v.Namespace != nil {
			if len(whitelist) > 0 && !utils.ContainsResourceEntry(whitelist, v) {
				continue
			}
			if blacklist != nil && utils.ContainsResourceEntry(blacklist, v) {
				continue
			}
			wg.Go(func() {
				result := store.GetResourceByKindAndNamespace(valkeyClient, v.Group, v.Kind, namespace, k8sLogger)
				resultsMutex.Lock()
				results = append(results, result...)
				resultsMutex.Unlock()
			})
		}
	}
	wg.Wait()

	return results, nil
}

func GetUnstructuredLabeledResourceList(label string, whitelist []*utils.ResourceEntry, blacklist []*utils.ResourceEntry) (unstructured.UnstructuredList, error) {
	results := unstructured.UnstructuredList{
		Object: map[string]any{},
		Items:  []unstructured.Unstructured{},
	}

	resources, err := GetAvailableResources()
	if err != nil {
		return results, err
	}

	if whitelist == nil {
		whitelist = []*utils.ResourceEntry{}
	}

	if blacklist == nil {
		blacklist = []*utils.ResourceEntry{}
	}

	dynamicClient := clientProvider.DynamicClient()
	for _, v := range resources {
		if v.Namespace != nil {

			if (len(whitelist) > 0 && !utils.ContainsResourceEntry(whitelist, v)) || (blacklist != nil && utils.ContainsResourceEntry(blacklist, v)) {
				continue
			}
			result, err := dynamicClient.Resource(CreateGroupVersionResource(v.Group, v.Version, v.Name)).List(context.Background(), metav1.ListOptions{LabelSelector: label})

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

func GetUnstructuredResource(group string, version string, name string, namespace, resourceName string) (*unstructured.Unstructured, error) {
	dynamicClient := clientProvider.DynamicClient()
	if namespace != "" {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Namespace(namespace).Get(context.Background(), resourceName, metav1.GetOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Get(context.Background(), resourceName, metav1.GetOptions{})
		return removeManagedFields(result), err
	}
}

func GetUnstructuredResourceFromStore(group string, kind string, namespace, resourceName string) (*unstructured.Unstructured, error) {
	return store.GetResource(valkeyClient, group, kind, namespace, resourceName, k8sLogger)
}

func CreateUnstructuredResource(group string, version string, name string, namespace *string, yamlData string) (*unstructured.Unstructured, error) {
	dynamicClient := clientProvider.DynamicClient()
	obj := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(yamlData), obj)
	if err != nil {
		return nil, err
	}

	if namespace != nil {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Namespace(obj.GetNamespace()).Create(context.Background(), obj, metav1.CreateOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Create(context.Background(), obj, metav1.CreateOptions{})
		return removeManagedFields(result), err
	}
}

func UpdateUnstructuredResource(group string, version string, name string, namespace *string, yamlData string) (*unstructured.Unstructured, error) {
	dynamicClient := clientProvider.DynamicClient()
	obj := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(yamlData), obj)
	if err != nil {
		return nil, err
	}

	if namespace != nil {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Namespace(obj.GetNamespace()).Update(context.Background(), obj, metav1.UpdateOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Update(context.Background(), obj, metav1.UpdateOptions{})
		return removeManagedFields(result), err
	}
}

func DeleteUnstructuredResource(group string, version string, name string, namespace string, resourceName string) error {
	dynamicClient := clientProvider.DynamicClient()
	if namespace != "" {
		return dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Namespace(namespace).Delete(context.Background(), resourceName, metav1.DeleteOptions{})
	} else {
		return dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Delete(context.Background(), resourceName, metav1.DeleteOptions{})
	}
}

func DescribeUnstructuredResource(group string, version string, name string, namespace, resourceName string) (string, error) {
	config := clientProvider.ClientConfig()

	restMapping := &meta.RESTMapping{
		Resource: CreateGroupVersionResource(group, version, name),
		GroupVersionKind: schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    name,
		},
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

func TriggerUnstructuredResource(group string, version string, name string, namespace string, resourceName string) (*unstructured.Unstructured, error) {
	dynamicClient := clientProvider.DynamicClient()

	if name == "cronjobs" || name == "jobs" {
		job, err := GetUnstructuredResource(group, version, name, namespace, resourceName)
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
		if name == "cronjobs" {
			template, _, err := unstructured.NestedMap(job.Object, "spec", "jobTemplate", "spec", "template")
			if err != nil {
				return nil, fmt.Errorf("field jobTemplate not found")
			}
			_ = unstructured.SetNestedField(job.Object, template, "spec", "template")
			name = "jobs"
		}

		return dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Namespace(namespace).Create(context.Background(), job, metav1.CreateOptions{})
	}
	return nil, fmt.Errorf("%s is a invalid resource for trigger. Only jobs or cronjobs can be triggert", name)
}

func GetK8sObjectFor(file string, namespaced bool) (any, error) {
	dynamicClient := clientProvider.DynamicClient()
	obj, err := GetObjectFromFile(file)
	if err != nil {
		return nil, err
	}

	resourceName, err := GetResourceNameForUnstructured(obj)
	if err != nil {
		return nil, err
	}

	if namespaced {
		res, err := dynamicClient.Resource(
			CreateGroupVersionResource(obj.GroupVersionKind().Group, obj.GroupVersionKind().Version, resourceName)).Namespace(obj.GetNamespace()).Get(context.Background(), obj.GetName(), metav1.GetOptions{})
		if err != nil {
			k8sLogger.Error("Error querying resource", "error", err)
			return nil, err
		}
		return res.Object, nil
	} else {
		res, err := dynamicClient.Resource(CreateGroupVersionResource(obj.GroupVersionKind().Group, obj.GroupVersionKind().Version, resourceName)).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			k8sLogger.Error("Error listing resource", "error", err)
			return nil, err
		}
		return res.Object, nil
	}
}

func GetResourceNameForUnstructured(obj *unstructured.Unstructured) (string, error) {
	resources, err := GetAvailableResources()
	if err != nil {
		return "", err
	}

	for _, v := range resources {
		if v.Kind == obj.GetKind() && v.Group == obj.GroupVersionKind().Group && v.Version == obj.GroupVersionKind().Version {
			return v.Name, nil
		}
	}
	return "", fmt.Errorf("resource not found for %s %s %s", obj.GetKind(), obj.GroupVersionKind().Group, obj.GroupVersionKind().Version)
}

func GetObjectFromFile(file string) (*unstructured.Unstructured, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var obj unstructured.Unstructured
	err = yaml.Unmarshal(data, &obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

type availableResourceCacheEntry struct {
	timestamp          time.Time
	availableResources []utils.ResourceEntry
}

var (
	resourceCache      availableResourceCacheEntry
	resourceCacheMutex sync.Mutex        // Mutex to ensure concurrent safe access to cache
	resourceCacheTTL   = 1 * time.Minute // Cache duration
)

func GetAvailableResources() ([]utils.ResourceEntry, error) {
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

	var availableResources []utils.ResourceEntry
	for _, resourceList := range resources {
		for _, resource := range resourceList.APIResources {
			if slices.Contains(resource.Verbs, "list") && slices.Contains(resource.Verbs, "watch") {
				var namespace *string
				if resource.Namespaced {
					namespace = utils.Pointer("")
				}
				availableResources = append(availableResources, utils.ResourceEntry{
					Group:     resourceList.GroupVersion,
					Name:      resource.Name,
					Kind:      resource.Kind,
					Version:   resource.Version,
					Namespace: namespace,
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
			return resource.Name, nil
		}
	}
	return "", fmt.Errorf("resource not found for name %s", name)
}

func CreateGroupVersionResource(group string, version string, name string) schema.GroupVersionResource {
	// for core apis we need change the group to empty string
	if group == "v1" && version == "" {
		return schema.GroupVersionResource{
			Group:    "",
			Version:  group,
			Resource: name,
		}
	}
	if strings.HasSuffix(group, version) {
		version = ""
	}

	return schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: name,
	}
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
