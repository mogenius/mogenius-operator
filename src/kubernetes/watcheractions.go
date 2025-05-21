package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
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
	"k8s.io/kubectl/pkg/describe"
	"sigs.k8s.io/yaml"
)

const (
	VALKEY_RESOURCE_PREFIX = "resources"
)

type GetUnstructuredNamespaceResourceListRequest struct {
	Namespace string                     `json:"namespace" validate:"required"`
	Whitelist []*utils.SyncResourceEntry `json:"whitelist"`
	Blacklist []*utils.SyncResourceEntry `json:"blacklist"`
}

type GetUnstructuredLabeledResourceListRequest struct {
	Label     string                     `json:"label" validate:"required"`
	Whitelist []*utils.SyncResourceEntry `json:"whitelist"`
	Blacklist []*utils.SyncResourceEntry `json:"blacklist"`
}

var MirroredResourceKinds = []string{
	"Deployment",
	"ReplicaSet",
	"CronJob",
	"Pod",
	"Job",
	"Event",
	"DaemonSet",
	"StatefulSet",
	"Namespace",
	"NetworkPolicy",
	"PersistentVolume",
	"PersistentVolumeClaim",
	"Service",
	"Node",

	"StorageClass",
	"IngressClass",
	"Ingress",
	"Secret",
	"ConfigMap",
	"HorizontalPodAutoscaler",
	"ServiceAccount",
	"Service",
	"RoleBinding",
	"Role",
	"ClusterRoleBinding",
	"ClusterRole",
	"Workspace", // mogenius specific
	"User",      // mogenius specific
	"Grant",     // mogenius specific
	"Team",      // mogenius specific
}

func WatchStoreResources(watcher WatcherModule, eventClient websocket.WebsocketClient) error {
	start := time.Now()

	resources, err := GetAvailableResources()
	if err != nil {
		return err
	}
	for _, v := range resources {
		if !slices.Contains(MirroredResourceKinds, v.Kind) {
			k8sLogger.Debug("🚀 Skipping resource", "kind", v.Kind, "group", v.Group, "namespace", v.Namespace)
			continue
		}
		err := watcher.Watch(k8sLogger, WatcherResourceIdentifier{
			Name:         v.Name,
			Kind:         v.Kind,
			GroupVersion: v.Group,
		}, func(resource WatcherResourceIdentifier, obj *unstructured.Unstructured) {
			setStoreIfNeeded(resource.GroupVersion, obj.GetName(), resource.Kind, obj.GetNamespace(), obj)
			// suppress the add events for the first 10 seconds (because all resources are added initially)
			if time.Since(start) < 10*time.Second {
				return
			}
			sendEventServerEvent(eventClient, v.Group, resource.Version, obj.GetName(), resource.Kind, obj.GetNamespace(), resource.Name, "add")
		}, func(resource WatcherResourceIdentifier, oldObj, newObj *unstructured.Unstructured) {
			setStoreIfNeeded(resource.GroupVersion, newObj.GetName(), resource.Kind, newObj.GetNamespace(), newObj)
			sendEventServerEvent(eventClient, v.Group, resource.Version, newObj.GetName(), resource.Kind, newObj.GetNamespace(), resource.Name, "update")
		}, func(resource WatcherResourceIdentifier, obj *unstructured.Unstructured) {
			deleteFromStoreIfNeeded(resource.GroupVersion, obj.GetName(), resource.Kind, obj.GetNamespace(), obj)
			sendEventServerEvent(eventClient, v.Group, resource.Version, obj.GetName(), resource.Kind, obj.GetNamespace(), resource.Name, "delete")
		})
		if err != nil {
			k8sLogger.Error("failed to initialize watchhandler for resource", "groupVersion", v.Group, "kind", v.Kind, "version", v.Version, "error", err)
			return err
		} else {
			k8sLogger.Debug("🚀 Watching resource", "kind", v.Kind, "group", v.Group)
		}
	}
	return nil
}

func setStoreIfNeeded(groupVersion string, resourceName string, kind string, namespace string, obj *unstructured.Unstructured) {
	obj = removeUnusedFieds(obj)

	// store in valkey
	err := valkeyClient.SetObject(obj, 0, VALKEY_RESOURCE_PREFIX, groupVersion, kind, namespace, resourceName)
	if err != nil {
		k8sLogger.Error("Error setting object in store", "error", err)
	}
}

func sendEventServerEvent(eventClient websocket.WebsocketClient, group string, version string, resourceName string, kind string, namespace string, name string, eventType string) {
	datagram := structs.CreateDatagramForClusterEvent("ClusterEvent", group, version, kind, namespace, name, resourceName, eventType)

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
	err := valkeyClient.Delete(VALKEY_RESOURCE_PREFIX, groupVersion, kind, namespace, resourceName)
	if err != nil {
		k8sLogger.Error("Error deleting object in store", "error", err)
	}
}

func GetUnstructuredResourceList(group string, version string, name string, namespace *string) (*unstructured.UnstructuredList, error) {
	dynamicClient := clientProvider.DynamicClient()
	if namespace != nil {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Namespace(*namespace).List(context.TODO(), metav1.ListOptions{})
		return removeManagedFieldsFromList(result), err
	} else {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).List(context.TODO(), metav1.ListOptions{})
		return removeManagedFieldsFromList(result), err
	}
}

func GetUnstructuredResourceListFromStore(group string, kind string, version string, name string, namespace *string) (unstructured.UnstructuredList, error) {
	results := unstructured.UnstructuredList{}
	if namespace == nil {
		namespace = utils.Pointer("")
	}
	// try to get the data from the store (very fast)
	result := store.GetResourceByKindAndNamespace(valkeyClient, group, kind, *namespace)
	if result != nil {
		results.Items = result
	}

	// fallback: gather the data when the store is empty (can be slow)
	if len(result) == 0 {
		// dont bother kubernetes with mirrored resources
		if slices.Contains(MirroredResourceKinds, kind) {
			return results, nil
		}
		list, err := GetUnstructuredResourceList(group, version, name, namespace)
		if err != nil {
			return results, err
		}
		results.Items = list.Items
	}

	return results, nil
}

func GetUnstructuredNamespaceResourceList(namespace string, whitelist []*utils.SyncResourceEntry, blacklist []*utils.SyncResourceEntry) ([]unstructured.Unstructured, error) {
	results := []unstructured.Unstructured{}

	resources, err := GetAvailableResources()
	if err != nil {
		return results, err
	}

	if whitelist == nil {
		whitelist = []*utils.SyncResourceEntry{}
	}

	if blacklist == nil {
		blacklist = []*utils.SyncResourceEntry{}
	}

	for _, v := range resources {
		if v.Namespace != nil {
			if (len(whitelist) > 0 && !utils.ContainsResourceEntry(whitelist, v)) || (blacklist != nil && utils.ContainsResourceEntry(blacklist, v)) {
				continue
			}

			result := store.GetResourceByKindAndNamespace(valkeyClient, v.Group, v.Kind, namespace)
			if result != nil {
				results = append(results, result...)
			}
		}
	}
	return results, nil
}

func GetUnstructuredLabeledResourceList(label string, whitelist []*utils.SyncResourceEntry, blacklist []*utils.SyncResourceEntry) (unstructured.UnstructuredList, error) {
	results := unstructured.UnstructuredList{
		Object: map[string]interface{}{},
		Items:  []unstructured.Unstructured{},
	}

	resources, err := GetAvailableResources()
	if err != nil {
		return results, err
	}

	if whitelist == nil {
		whitelist = []*utils.SyncResourceEntry{}
	}

	if blacklist == nil {
		blacklist = []*utils.SyncResourceEntry{}
	}

	dynamicClient := clientProvider.DynamicClient()
	//// dynamicClient := clientProvider.DynamicClient()
	for _, v := range resources {
		if v.Namespace != nil {

			if (len(whitelist) > 0 && !utils.ContainsResourceEntry(whitelist, v)) || (blacklist != nil && utils.ContainsResourceEntry(blacklist, v)) {
				continue
			}
			result, err := dynamicClient.Resource(CreateGroupVersionResource(v.Group, v.Version, v.Name)).List(context.TODO(), metav1.ListOptions{LabelSelector: label})

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
		result, err := dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Namespace(namespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Get(context.TODO(), resourceName, metav1.GetOptions{})
		return removeManagedFields(result), err
	}
}

func CreateUnstructuredResource(group string, version string, name string, namespace *string, yamlData string) (*unstructured.Unstructured, error) {
	dynamicClient := clientProvider.DynamicClient()
	obj := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(yamlData), obj)
	if err != nil {
		return nil, err
	}

	if namespace != nil {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Namespace(obj.GetNamespace()).Create(context.TODO(), obj, metav1.CreateOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Create(context.TODO(), obj, metav1.CreateOptions{})
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
		result, err := dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Namespace(obj.GetNamespace()).Update(context.TODO(), obj, metav1.UpdateOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Update(context.TODO(), obj, metav1.UpdateOptions{})
		return removeManagedFields(result), err
	}
}

func DeleteUnstructuredResource(group string, version string, name string, namespace string, resourceName string) error {
	dynamicClient := clientProvider.DynamicClient()
	if namespace != "" {
		return dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Namespace(namespace).Delete(context.TODO(), resourceName, metav1.DeleteOptions{})
	} else {
		return dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Delete(context.TODO(), resourceName, metav1.DeleteOptions{})
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

		return dynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Namespace(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
	}
	return nil, fmt.Errorf("%s is a invalid resource for trigger. Only jobs or cronjobs can be triggert", name)
}

func GetK8sObjectFor(file string, namespaced bool) (interface{}, error) {
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
			CreateGroupVersionResource(obj.GroupVersionKind().Group, obj.GroupVersionKind().Version, resourceName)).Namespace(obj.GetNamespace()).Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
		if err != nil {
			k8sLogger.Error("Error querying resource", "error", err)
			return nil, err
		}
		return res.Object, nil
	} else {
		res, err := dynamicClient.Resource(CreateGroupVersionResource(obj.GroupVersionKind().Group, obj.GroupVersionKind().Version, resourceName)).List(context.TODO(), metav1.ListOptions{})
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
	availableResources []utils.SyncResourceEntry
}

var (
	resourceCache      availableResourceCacheEntry
	resourceCacheMutex sync.Mutex        // Mutex to ensure concurrent safe access to cache
	resourceCacheTTL   = 1 * time.Minute // Cache duration
)

func GetAvailableResources() ([]utils.SyncResourceEntry, error) {
	resourceCacheMutex.Lock()
	defer resourceCacheMutex.Unlock()

	// Check if we have cached resources and if they are still valid
	if time.Since(resourceCache.timestamp) < resourceCacheTTL {
		return resourceCache.availableResources, nil
	}

	// No valid cache, fetch resources from server
	clientset := clientProvider.K8sClientSet()
	resources, err := clientset.Discovery().ServerPreferredResources()
	if err != nil {
		k8sLogger.Error("Error discovering resources", "error", err)
		return nil, err
	}

	var availableResources []utils.SyncResourceEntry
	for _, resourceList := range resources {
		for _, resource := range resourceList.APIResources {
			if slices.Contains(resource.Verbs, "list") && slices.Contains(resource.Verbs, "watch") {
				var namespace *string
				if resource.Namespaced {
					namespace = utils.Pointer("")
				}
				availableResources = append(availableResources, utils.SyncResourceEntry{
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

func GetAvailableResourcesSerialized() string {
	resources, err := GetAvailableResources()
	if err != nil {
		k8sLogger.Error("Error updating available resources", "error", err)
		return ""
	}

	bytes, err := yaml.Marshal(resources)
	if err != nil {
		k8sLogger.Error("Error serializing available resources", "error", err)
		return ""
	}

	return string(bytes)
}

func GetSyncResourcesFromString(resourcesStr string) ([]utils.SyncResourceEntry, error) {
	var resources []utils.SyncResourceEntry
	err := yaml.Unmarshal([]byte(resourcesStr), &resources)
	if err != nil {
		return nil, err
	}

	return resources, nil
}

func CommaSeperatedStringToArray(str string) []string {
	return strings.Split(str, ",")
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
		delete(unstructuredContent["metadata"].(map[string]interface{}), "managedFields")
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
	if obj == nil {
		return obj
	}

	obj = removeManagedFields(obj)
	unstructuredContent := obj.Object
	delete(unstructuredContent, "data")

	return obj
}
