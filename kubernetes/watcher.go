package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mogenius-k8s-manager/interfaces"
	"mogenius-k8s-manager/shutdown"
	"mogenius-k8s-manager/store"
	"mogenius-k8s-manager/utils"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	v1Net "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubectl/pkg/describe"
	"sigs.k8s.io/yaml"
)

var AvailableResources []utils.SyncResourceEntry

type Watcher struct {
	handlerMapLock sync.Mutex
	activeHandlers map[interfaces.WatcherResourceIdentifier]resourceContext
}

func NewWatcher() Watcher {
	return Watcher{
		handlerMapLock: sync.Mutex{},
		activeHandlers: make(map[interfaces.WatcherResourceIdentifier]resourceContext, 0),
	}
}

type resourceContext struct {
	state    interfaces.WatcherResourceState
	informer cache.SharedIndexInformer
	handler  cache.ResourceEventHandlerRegistration
}

func (m *Watcher) Watch(resource interfaces.WatcherResourceIdentifier, onAdd interfaces.WatcherOnAdd, onUpdate interfaces.WatcherOnUpdate, onDelete interfaces.WatcherOnDelete) error {
	m.handlerMapLock.Lock()
	defer m.handlerMapLock.Unlock()

	for r := range m.activeHandlers {
		if resource == r {
			return fmt.Errorf("resources is already being watched")
		}
	}

	provider, err := NewKubeProvider()
	if provider == nil || err != nil {
		return fmt.Errorf("failed to create provider for watcher: %s", err.Error())
	}

	gv, err := schema.ParseGroupVersion(resource.GroupVersion)
	if err != nil {
		return fmt.Errorf("invalid groupVersion: %s", err)
	}

	informerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(provider.DynamicClient, time.Minute*10, v1.NamespaceAll, nil)

	resourceInformer := informerFactory.ForResource(createResourceVersion(gv.Group, gv.Version, resource.Name)).Informer()

	err = resourceInformer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		if err == io.EOF {
			return // closed normally, its fine
		}
		k8sLogger.Error(`Encountered error while watching resource`, "resourceName", resource.Name, "resourceKind", resource.Kind, "resourceGroupVersion", resource.GroupVersion, "error", err)
	})
	if err != nil {
		return fmt.Errorf("failed to set error watch handler: %s", err)
	}
	handler, err := resourceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			unstructuredObj, ok := obj.(*unstructured.Unstructured)
			if !ok {
				body, _ := json.Marshal(obj)
				bodyString := string(body)
				k8sLogger.Warn(`failed to deserialize`, "resourceJson", bodyString)
				return
			}
			if onAdd != nil {
				onAdd(resource, unstructuredObj)
			}
			SetStoreIfNeeded(resource.Kind, unstructuredObj.GetNamespace(), unstructuredObj.GetName(), unstructuredObj)
			IacManagerWriteResourceYaml(resource.Kind, unstructuredObj.GetNamespace(), unstructuredObj.GetName(), unstructuredObj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldUnstructuredObj, ok := oldObj.(*unstructured.Unstructured)
			if !ok {
				body, _ := json.Marshal(newObj)
				bodyString := string(body)
				k8sLogger.Warn(`failed to deserialize`, "resourceJson", bodyString)
				return
			}
			newUnstructuredObj, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				body, _ := json.Marshal(newObj)
				bodyString := string(body)
				k8sLogger.Warn(`failed to deserialize`, "resourceJson", bodyString)
				return
			}
			if onUpdate != nil {
				onUpdate(resource, oldUnstructuredObj, newUnstructuredObj)
			}
			SetStoreIfNeeded(resource.Kind, newUnstructuredObj.GetNamespace(), newUnstructuredObj.GetName(), newUnstructuredObj)
			IacManagerWriteResourceYaml(resource.Kind, newUnstructuredObj.GetNamespace(), newUnstructuredObj.GetName(), newUnstructuredObj)
		},
		DeleteFunc: func(obj interface{}) {
			unstructuredObj, ok := obj.(*unstructured.Unstructured)
			if !ok {
				body, _ := json.Marshal(obj)
				bodyString := string(body)
				k8sLogger.Warn(`failed to deserialize`, "resourceJson", bodyString)
				return
			}
			if onDelete != nil {
				onDelete(resource, unstructuredObj)
			}
			DeleteFromStoreIfNeeded(resource.Kind, unstructuredObj.GetNamespace(), unstructuredObj.GetName(), unstructuredObj)
			IacManagerDeleteResourceYaml(resource.Kind, unstructuredObj.GetNamespace(), unstructuredObj.GetName(), obj)
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add eventhandler: %s", err.Error())
	}

	go func() {
		stopCh := make(chan struct{})
		go resourceInformer.Run(stopCh)

		if !cache.WaitForCacheSync(stopCh, resourceInformer.HasSynced) {
			m.handlerMapLock.Lock()
			defer m.handlerMapLock.Unlock()
			resourceContext, ok := m.activeHandlers[resource]
			if !ok {
				k8sLogger.Warn("Attempted to update resource state but resource has been removed from watcher", "resource", resource)
			}
			resourceContext.state = interfaces.WatchingFailed
			m.activeHandlers[resource] = resourceContext
			return
		}

		m.handlerMapLock.Lock()
		defer m.handlerMapLock.Unlock()
		resourceContext, ok := m.activeHandlers[resource]
		if !ok {
			k8sLogger.Warn("Attempted to update resource state but resource has been removed from watcher", "resource", resource)
		}
		resourceContext.state = interfaces.Watching
		m.activeHandlers[resource] = resourceContext
	}()

	m.activeHandlers[resource] = resourceContext{
		state:    interfaces.WatcherInitializing,
		informer: resourceInformer,
		handler:  handler,
	}

	return nil
}

func (m *Watcher) Unwatch(resource interfaces.WatcherResourceIdentifier) error {
	m.handlerMapLock.Lock()
	defer m.handlerMapLock.Unlock()

	resourceContext, ok := m.activeHandlers[resource]
	if !ok {
		return fmt.Errorf("resource is not being watched")
	}

	err := resourceContext.informer.RemoveEventHandler(resourceContext.handler)
	if err != nil {
		return fmt.Errorf("failed to remove event handler: %s", err.Error())
	}
	delete(m.activeHandlers, resource)

	return nil
}

func (m *Watcher) ListWatchedResources() []interfaces.WatcherResourceIdentifier {
	m.handlerMapLock.Lock()
	defer m.handlerMapLock.Unlock()

	resources := make([]interfaces.WatcherResourceIdentifier, len(m.activeHandlers))
	for r := range m.activeHandlers {
		resources = append(resources, r)
	}

	return resources
}

func (m *Watcher) State(resource interfaces.WatcherResourceIdentifier) (interfaces.WatcherResourceState, error) {
	m.handlerMapLock.Lock()
	defer m.handlerMapLock.Unlock()

	resourceContext, ok := m.activeHandlers[resource]
	if !ok {
		return interfaces.Unknown, fmt.Errorf("resource is not being watched")
	}

	return resourceContext.state, nil
}

func WatchAllResources(watcher interfaces.WatcherModule) {
	// Retry watching resources with exponential backoff in case of failures
	err := retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}, apierrors.IsServiceUnavailable, func() error {
		// TODO: this has to keep running and Watch/Unwatch resources instead of only registering on startup
		workloads := utils.CONFIG.Iac.SyncWorkloads
		for _, v := range workloads {
			err := watcher.Watch(interfaces.WatcherResourceIdentifier{
				Name:         v.Name,
				Kind:         v.Kind,
				GroupVersion: v.Group,
			}, nil, nil, nil)
			if err != nil {
				k8sLogger.Error("failed to initialize watchhandler for resource", "kind", v.Kind, "version", v.Version, "error", err)
			} else {
				k8sLogger.Info("🚀 Watching resource", "kind", v.Kind, "group", v.Group)
			}
		}
		return nil
	})
	if err != nil {
		k8sLogger.Error("Error watching resources", "error", err)
		shutdown.SendShutdownSignalAndBlockForever(true)
		select {}
	}
}

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
			return
		}

		HandleNetworkPolicyChange(&netPol, "Deleted")
		return
	}
}

func InitAllWorkloads() {
	utils.Assert(IacManagerShouldWatchResources != nil, "func IacManagerShouldWatchResources has to be initialized")
	allResources, err := GetAvailableResources()
	if err != nil {
		k8sLogger.Error("Error getting available resources", "error", err)
		return
	}
	for _, resource := range allResources {
		if IacManagerShouldWatchResources() {
			list, err := GetUnstructuredResourceList(resource.Group, resource.Version, resource.Kind, resource.Namespaced)
			if err != nil {
				k8sLogger.Error("Error getting resource list", "kind", resource.Kind, "error", err)
				continue
			}
			for _, res := range list.Items {
				IacManagerWriteResourceYaml(resource.Kind, res.GetNamespace(), res.GetName(), res.Object)
			}
		}
	}
}

func GetUnstructuredResourceList(group, version, name string, namespaced bool) (*unstructured.UnstructuredList, error) {
	provider, err := NewKubeProvider()
	if provider == nil || err != nil {
		k8sLogger.Error("Error creating provider for GetUnstructuredResourceList. Cannot continue.", "error", err)
		return nil, err
	}

	if namespaced {
		result, err := provider.DynamicClient.Resource(createResourceVersion(group, version, name)).Namespace("").List(context.TODO(), metav1.ListOptions{})
		return removeManagedFieldsFromList(result), err
	} else {
		result, err := provider.DynamicClient.Resource(createResourceVersion(group, version, name)).List(context.TODO(), metav1.ListOptions{})
		return removeManagedFieldsFromList(result), err
	}
}

func GetUnstructuredResource(group, version, name string, namespace, resourceName string) (*unstructured.Unstructured, error) {
	provider, err := NewKubeProvider()
	if provider == nil || err != nil {
		k8sLogger.Error("Error creating provider for GetUnstructuredResource. Cannot continue.", "error", err)
		return nil, err
	}

	if namespace != "" {
		result, err := provider.DynamicClient.Resource(createResourceVersion(group, version, name)).Namespace(namespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := provider.DynamicClient.Resource(createResourceVersion(group, version, name)).Get(context.TODO(), resourceName, metav1.GetOptions{})
		return removeManagedFields(result), err
	}
}

func CreateUnstructuredResource(group, version, name string, namespaced bool, yamlData string) (*unstructured.Unstructured, error) {
	provider, err := NewKubeProvider()
	if provider == nil || err != nil {
		k8sLogger.Error("Error creating provider for CreateUnstructuredResource. Cannot continue.", "error", err)
		return nil, err
	}

	obj := &unstructured.Unstructured{}
	err = yaml.Unmarshal([]byte(yamlData), obj)
	if err != nil {
		return nil, err
	}

	if namespaced {
		result, err := provider.DynamicClient.Resource(createResourceVersion(group, version, name)).Namespace(obj.GetNamespace()).Create(context.TODO(), obj, metav1.CreateOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := provider.DynamicClient.Resource(createResourceVersion(group, version, name)).Create(context.TODO(), obj, metav1.CreateOptions{})
		return removeManagedFields(result), err
	}
}

func UpdateUnstructuredResource(group, version, name string, namespaced bool, yamlData string) (*unstructured.Unstructured, error) {
	provider, err := NewKubeProvider()
	if provider == nil || err != nil {
		k8sLogger.Error("Error creating provider for UpdatedUnstructuredResource. Cannot continue.", "error", err)
		return nil, err
	}

	obj := &unstructured.Unstructured{}
	err = yaml.Unmarshal([]byte(yamlData), obj)
	if err != nil {
		return nil, err
	}

	if namespaced {
		result, err := provider.DynamicClient.Resource(createResourceVersion(group, version, name)).Namespace(obj.GetNamespace()).Update(context.TODO(), obj, metav1.UpdateOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := provider.DynamicClient.Resource(createResourceVersion(group, version, name)).Update(context.TODO(), obj, metav1.UpdateOptions{})
		return removeManagedFields(result), err
	}
}

func DeleteUnstructuredResource(group, version, name string, namespace string, resourceName string) error {
	provider, err := NewKubeProvider()
	if provider == nil || err != nil {
		k8sLogger.Error("Error creating provider for watcher. Cannot continue.", "error", err)
		return err
	}

	if namespace != "" {
		return provider.DynamicClient.Resource(createResourceVersion(group, version, name)).Namespace(namespace).Delete(context.TODO(), resourceName, metav1.DeleteOptions{})
	} else {
		return provider.DynamicClient.Resource(createResourceVersion(group, version, name)).Delete(context.TODO(), resourceName, metav1.DeleteOptions{})
	}
}

func DescribeUnstructuredResource(group, version, name string, namespace, resourceName string) (string, error) {
	provider, err := NewKubeProvider()
	if provider == nil || err != nil {
		k8sLogger.Error("Error creating provider for watcher. Cannot continue.", "error", err)
		return "", err
	}

	restMapping := &meta.RESTMapping{
		Resource: createResourceVersion(group, version, name),
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
	provider, err := NewKubeProvider()
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
			createResourceVersion(obj.GroupVersionKind().Group, obj.GroupVersionKind().Version, resourceName)).Namespace(obj.GetNamespace()).Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
		if err != nil {
			k8sLogger.Error("Error querying resource", "error", err)
			return nil, err
		}
		return res.Object, nil
	} else {
		res, err := provider.DynamicClient.Resource(createResourceVersion(obj.GroupVersionKind().Group, obj.GroupVersionKind().Version, resourceName)).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			k8sLogger.Error("Error listing resource", "error", err)
			return nil, err
		}
		return res.Object, nil
	}
}

func GetResourceNameForUnstructured(obj *unstructured.Unstructured) (string, error) {
	for _, v := range AvailableResources {
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
				availableResources = append(availableResources, utils.SyncResourceEntry{
					Group:      resourceList.GroupVersion,
					Name:       resource.Name,
					Kind:       resource.Kind,
					Version:    resource.Version,
					Namespaced: resource.Namespaced,
				})
			}
		}
	}

	AvailableResources = availableResources

	return availableResources, nil
}

func GetAvailableResourcesSerialized() string {
	resources, err := GetAvailableResources()
	if err != nil {
		k8sLogger.Error("Error getting available resources", "error", err)
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

func createResourceVersion(group, version, name string) schema.GroupVersionResource {
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
