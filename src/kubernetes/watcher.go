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

func SetStoreIfNeeded(kind string, namespace string, name string, obj *unstructured.Unstructured) {
	if kind == "Deployment" || kind == "ReplicaSet" || kind == "CronJob" || kind == "Pod" || kind == "Job" || kind == "Event" || kind == "DaemonSet" || kind == "StatefulSet" {
		err := store.GlobalStore.Set(obj, kind, namespace, name)
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
		return
	}

	if kind == "Namespace" {
		err := store.GlobalStore.Set(obj, kind, name)
		if err != nil {
			k8sLogger.Error("Error setting object in store", "error", err)
		}
		return
	}

	if kind == "NetworkPolicy" {
		err := store.GlobalStore.Set(obj, kind, namespace, name)
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
}

func DeleteFromStoreIfNeeded(kind string, namespace string, name string, obj *unstructured.Unstructured) {
	if kind == "Deployment" || kind == "ReplicaSet" || kind == "CronJob" || kind == "Pod" || kind == "Job" || kind == "Event" || kind == "DaemonSet" || kind == "StatefulSet" {
		err := store.GlobalStore.Delete(kind, namespace, name)
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
		return
	}

	if kind == "Namespace" {
		err := store.GlobalStore.Delete(kind, name)
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
		err := store.GlobalStore.Delete(kind, namespace, name)
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
}

func GetUnstructuredResourceList(group, version, name string, namespace *string) (*unstructured.UnstructuredList, error) {
	provider, err := NewKubeProviderDynamic()
	if provider == nil || err != nil {
		k8sLogger.Error("Error creating provider for GetUnstructuredResourceList. Cannot continue.", "error", err)
		return nil, err
	}

	if namespace != nil {
		result, err := provider.DynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Namespace(*namespace).List(context.TODO(), metav1.ListOptions{})
		return removeManagedFieldsFromList(result), err
	} else {
		result, err := provider.DynamicClient.Resource(CreateGroupVersionResource(group, version, name)).List(context.TODO(), metav1.ListOptions{})
		return removeManagedFieldsFromList(result), err
	}
}

func GetUnstructuredResource(group, version, name string, namespace, resourceName string) (*unstructured.Unstructured, error) {
	provider, err := NewKubeProviderDynamic()
	if provider == nil || err != nil {
		k8sLogger.Error("Error creating provider for GetUnstructuredResource. Cannot continue.", "error", err)
		return nil, err
	}

	if namespace != "" {
		result, err := provider.DynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Namespace(namespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := provider.DynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Get(context.TODO(), resourceName, metav1.GetOptions{})
		return removeManagedFields(result), err
	}
}

func CreateUnstructuredResource(group, version, name string, namespace *string, yamlData string) (*unstructured.Unstructured, error) {
	provider, err := NewKubeProviderDynamic()
	if provider == nil || err != nil {
		k8sLogger.Error("Error creating provider for CreateUnstructuredResource. Cannot continue.", "error", err)
		return nil, err
	}

	obj := &unstructured.Unstructured{}
	err = yaml.Unmarshal([]byte(yamlData), obj)
	if err != nil {
		return nil, err
	}

	if namespace != nil {
		result, err := provider.DynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Namespace(obj.GetNamespace()).Create(context.TODO(), obj, metav1.CreateOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := provider.DynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Create(context.TODO(), obj, metav1.CreateOptions{})
		return removeManagedFields(result), err
	}
}

func UpdateUnstructuredResource(group, version, name string, namespace *string, yamlData string) (*unstructured.Unstructured, error) {
	provider, err := NewKubeProviderDynamic()
	if provider == nil || err != nil {
		k8sLogger.Error("Error creating provider for UpdatedUnstructuredResource. Cannot continue.", "error", err)
		return nil, err
	}

	obj := &unstructured.Unstructured{}
	err = yaml.Unmarshal([]byte(yamlData), obj)
	if err != nil {
		return nil, err
	}

	if namespace != nil {
		result, err := provider.DynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Namespace(obj.GetNamespace()).Update(context.TODO(), obj, metav1.UpdateOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := provider.DynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Update(context.TODO(), obj, metav1.UpdateOptions{})
		return removeManagedFields(result), err
	}
}

func DeleteUnstructuredResource(group, version, name string, namespace string, resourceName string) error {
	provider, err := NewKubeProviderDynamic()
	if provider == nil || err != nil {
		k8sLogger.Error("Error creating provider for watcher. Cannot continue.", "error", err)
		return err
	}

	if namespace != "" {
		return provider.DynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Namespace(namespace).Delete(context.TODO(), resourceName, metav1.DeleteOptions{})
	} else {
		return provider.DynamicClient.Resource(CreateGroupVersionResource(group, version, name)).Delete(context.TODO(), resourceName, metav1.DeleteOptions{})
	}
}

func DescribeUnstructuredResource(group, version, name string, namespace, resourceName string) (string, error) {
	provider, err := NewKubeProvider()
	if provider == nil || err != nil {
		k8sLogger.Error("Error creating provider for watcher. Cannot continue.", "error", err)
		return "", err
	}

	restMapping := &meta.RESTMapping{
		Resource: CreateGroupVersionResource(group, version, name),
		GroupVersionKind: schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    name,
		},
	}

	describer, ok := describe.GenericDescriberFor(restMapping, &provider.ClientConfig)
	if !ok {
		fmt.Printf("Failed to get describer: %v\n", err)
		return "", err
	}

	output, err := describer.Describe(namespace, resourceName, describe.DescriberSettings{ShowEvents: true})
	if err != nil {
		fmt.Printf("Failed to describe resource: %v\n", err)
		return "", err
	}

	return output, nil
}

func GetK8sObjectFor(file string, namespaced bool) (interface{}, error) {
	provider, err := NewKubeProviderDynamic()
	if provider == nil || err != nil {
		k8sLogger.Error("Error creating provider for watcher. Cannot continue.", "error", err)
		return nil, err
	}

	obj, err := GetObjectFromFile(file)
	if err != nil {
		return nil, err
	}

	resourceName, err := GetResourceNameForUnstructured(obj)
	if err != nil {
		return nil, err
	}

	if namespaced {
		res, err := provider.DynamicClient.Resource(
			CreateGroupVersionResource(obj.GroupVersionKind().Group, obj.GroupVersionKind().Version, resourceName)).Namespace(obj.GetNamespace()).Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
		if err != nil {
			k8sLogger.Error("Error querying resource", "error", err)
			return nil, err
		}
		return res.Object, nil
	} else {
		res, err := provider.DynamicClient.Resource(CreateGroupVersionResource(obj.GroupVersionKind().Group, obj.GroupVersionKind().Version, resourceName)).List(context.TODO(), metav1.ListOptions{})
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
	provider, err := NewKubeProvider()
	if provider == nil || err != nil {
		k8sLogger.Error("Error creating provider for watcher. Cannot continue.", "error", err)
		return nil, err
	}

	resources, err := provider.ClientSet.Discovery().ServerPreferredResources()
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
