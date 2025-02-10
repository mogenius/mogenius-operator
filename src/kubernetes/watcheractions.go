package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/utils"
	"os"
	"slices"
	"strings"

	v1 "k8s.io/api/core/v1"
	v1Net "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubectl/pkg/describe"
	"sigs.k8s.io/yaml"
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

func containsResourceEntry(resources []*utils.SyncResourceEntry, target utils.SyncResourceEntry) bool {
	for _, r := range resources {
		if r.Kind == target.Kind && r.Group == target.Group {
			return true
		}
	}
	return false
}

func WatchStoreResources(watcher WatcherModule) error {
	resources, err := GetAvailableResources()
	if err != nil {
		return err
	}
	relevantResourceKinds := []string{
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
		"Node",
	}
	for _, v := range resources {
		//if !slices.Contains(relevantResourceKinds, v.Kind) {
		//	continue
		//}
		if v.Namespace == nil && !slices.Contains(relevantResourceKinds, v.Kind) {
			k8sLogger.Debug("🚀 Skipping resource", "kind", v.Kind, "group", v.Group, "namespace", v.Namespace)
			continue
		}
		err := watcher.Watch(k8sLogger, WatcherResourceIdentifier{
			Name:         v.Name,
			Kind:         v.Kind,
			GroupVersion: v.Group,
		}, func(resource WatcherResourceIdentifier, obj *unstructured.Unstructured) {
			SetStoreIfNeeded(resource.GroupVersion, resource.Kind, obj.GetNamespace(), obj.GetName(), obj)
		}, func(resource WatcherResourceIdentifier, oldObj, newObj *unstructured.Unstructured) {
			SetStoreIfNeeded(resource.GroupVersion, resource.Kind, newObj.GetNamespace(), newObj.GetName(), newObj)
		}, func(resource WatcherResourceIdentifier, obj *unstructured.Unstructured) {
			DeleteFromStoreIfNeeded(resource.GroupVersion, resource.Kind, obj.GetNamespace(), obj.GetName(), obj)
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

func SetStoreIfNeeded(groupVersion string, kind string, namespace string, name string, obj *unstructured.Unstructured) {
	//if kind == "Deployment" || kind == "ReplicaSet" || kind == "CronJob" || kind == "Pod" || kind == "Job" || kind == "Event" || kind == "DaemonSet" || kind == "StatefulSet" {
	//	err := store.GlobalStore.Set(obj, groupVersion, kind, namespace, name)
	//	if err != nil {
	//		k8sLogger.Error("Error setting object in store", "error", err)
	//	}
	//	if kind == "Event" {
	//		var event v1.Event
	//		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &event)
	//		if err != nil {
	//			k8sLogger.Error("Error cannot cast from unstructured", "error", err)
	//			return
	//		}
	//		processEvent(&event)
	//	}
	//	return
	//}

	if kind == "Namespace" {
		err := store.GlobalStore.Set(obj, groupVersion, kind, name)
		if err != nil {
			k8sLogger.Error("Error setting object in store", "error", err)
		}
		return
	}

	if kind == "NetworkPolicy" {
		err := store.GlobalStore.Set(obj, groupVersion, kind, namespace, name)
		if err != nil {
			k8sLogger.Error("Error setting object in store", "error", err)
		}

		var netPol v1Net.NetworkPolicy
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &netPol)
		if err != nil {
			k8sLogger.Error("Error cannot cast from unstructured", "error", err)
			return
		}

		HandleNetworkPolicyChange(&netPol, "Added/Updated")
		return
	}

	// other resources
	err := store.GlobalStore.Set(obj, groupVersion, kind, namespace, name)
	if err != nil {
		k8sLogger.Error("Error setting object in store", "error", err)
	}
	if kind == "Event" {
		var event v1.Event
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &event)
		if err != nil {
			k8sLogger.Error("Error cannot cast from unstructured", "error", err)
			return
		}
		processEvent(&event)
	}
}

func DeleteFromStoreIfNeeded(groupVersion string, kind string, namespace string, name string, obj *unstructured.Unstructured) {
	//if kind == "Deployment" || kind == "ReplicaSet" || kind == "CronJob" || kind == "Pod" || kind == "Job" || kind == "Event" || kind == "DaemonSet" || kind == "StatefulSet" {
	//	err := store.GlobalStore.Delete(groupVersion, kind, namespace, name)
	//	if err != nil {
	//		k8sLogger.Error("Error deleting object in store", "error", err)
	//	}
	//	if kind == "Event" {
	//		var event v1.Event
	//		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &event)
	//		if err != nil {
	//			k8sLogger.Error("Error cannot cast from unstructured", "error", err)
	//			return
	//		}
	//		processEvent(&event)
	//	}
	//	return
	//}

	if kind == "Namespace" {
		err := store.GlobalStore.Delete(groupVersion, kind, name)
		if err != nil {
			k8sLogger.Error("Error deleting object in store", "error", err)
		}
		return
	}

	if kind == "PersistentVolume" {
		var pv v1.PersistentVolume
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &pv)
		if err != nil {
			k8sLogger.Error("Error cannot cast from unstructured", "error", err)
			return
		}
		handlePVDeletion(&pv)
		return
	}

	if kind == "NetworkPolicy" {
		err := store.GlobalStore.Delete(groupVersion, kind, namespace, name)
		if err != nil {
			k8sLogger.Error("Error deleting object in store", "error", err)
		}

		var netPol v1Net.NetworkPolicy
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &netPol)
		if err != nil {
			k8sLogger.Error("Error cannot cast from unstructured", "error", err)
			return
		}

		HandleNetworkPolicyChange(&netPol, "Deleted")
		return
	}

	// other resources
	err := store.GlobalStore.Delete(groupVersion, kind, namespace, name)
	if err != nil {
		k8sLogger.Error("Error deleting object in store", "error", err)
	}
	if kind == "Event" {
		var event v1.Event
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &event)
		if err != nil {
			k8sLogger.Error("Error cannot cast from unstructured", "error", err)
			return
		}
		processEvent(&event)
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
	result := store.GetResourceByKindAndNamespace(group, kind, *namespace)
	if result != nil {
		results.Items = result
	}
	return results, nil
}

func GetUnstructuredNamespaceResourceList(namespace string, whitelist []*utils.SyncResourceEntry, blacklist []*utils.SyncResourceEntry) (*[]unstructured.Unstructured, error) {
	resources, err := GetAvailableResources()
	if err != nil {
		return nil, err
	}

	if whitelist == nil {
		whitelist = []*utils.SyncResourceEntry{}
	}

	if blacklist == nil {
		blacklist = []*utils.SyncResourceEntry{}
	}

	results := []unstructured.Unstructured{}

	// dynamicClient := clientProvider.DynamicClient()
	for _, v := range resources {
		if v.Namespace != nil {
			//result, err := dynamicClient.Resource(CreateGroupVersionResource(v.Group, v.Version, v.Name)).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
			//if err != nil {
			//	if !os.IsNotExist(err) {
			//		k8sLogger.Error("Error querying resource", "error", err)
			//	}
			//}
			if (len(whitelist) > 0 && !containsResourceEntry(whitelist, v)) || (blacklist != nil && containsResourceEntry(blacklist, v)) {
				continue
			}

			result := store.GetResourceByKindAndNamespace(v.Group, v.Kind, namespace)
			if result != nil {
				results = append(results, result...)
			}
		}
	}
	return &results, nil
}

func GetUnstructuredLabeledResourceList(label string, whitelist []*utils.SyncResourceEntry, blacklist []*utils.SyncResourceEntry) (*unstructured.UnstructuredList, error) {
	resources, err := GetAvailableResources()
	if err != nil {
		return nil, err
	}

	if whitelist == nil {
		whitelist = []*utils.SyncResourceEntry{}
	}

	if blacklist == nil {
		blacklist = []*utils.SyncResourceEntry{}
	}

	results := []unstructured.Unstructured{}

	dynamicClient := clientProvider.DynamicClient()
	//// dynamicClient := clientProvider.DynamicClient()
	for _, v := range resources {
		if v.Namespace != nil {

			if (len(whitelist) > 0 && !containsResourceEntry(whitelist, v)) || (blacklist != nil && containsResourceEntry(blacklist, v)) {
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
				results = append(results, result.Items...)
			}
		}
	}
	return &unstructured.UnstructuredList{Items: results}, nil
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
	return "", fmt.Errorf("Resource not found for %s %s %s", obj.GetKind(), obj.GroupVersionKind().Group, obj.GroupVersionKind().Version)
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

func GetAvailableResources() ([]utils.SyncResourceEntry, error) {
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
