package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mogenius-k8s-manager/interfaces"
	"mogenius-k8s-manager/store"
	"mogenius-k8s-manager/utils"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
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
	activeHandlers map[interfaces.ResourceIdentifier]resourceContext
}

func NewWatcher() Watcher {
	return Watcher{
		handlerMapLock: sync.Mutex{},
		activeHandlers: make(map[interfaces.ResourceIdentifier]resourceContext, 0),
	}
}

type resourceContext struct {
	state    interfaces.ResourceState
	informer cache.SharedIndexInformer
	handler  cache.ResourceEventHandlerRegistration
}

func (m *Watcher) Watch(resource interfaces.ResourceIdentifier, onAdd func(resource interfaces.ResourceIdentifier, obj *unstructured.Unstructured), onUpdate func(resource interfaces.ResourceIdentifier, oldObj *unstructured.Unstructured, newObj *unstructured.Unstructured), onDelete func(resource interfaces.ResourceIdentifier, obj *unstructured.Unstructured)) error {
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

	gvr := schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: resource.Name}
	resourceInformer := informerFactory.ForResource(gvr).Informer()

	err = resourceInformer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		if err == io.EOF {
			return // closed normally, its fine
		}
		K8sLogger.Errorf(`WatchError on Name('%s') Kind('%s') GroupVersion('%s'): %s`, resource.Name, resource.Kind, resource.GroupVersion, err)
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
				K8sLogger.Warnf(`failed to deserialize: %s`, bodyString)
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
				K8sLogger.Warnf(`failed to deserialize: %s`, bodyString)
				return
			}
			newUnstructuredObj, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				body, _ := json.Marshal(newObj)
				bodyString := string(body)
				K8sLogger.Warnf(`failed to deserialize: %s`, bodyString)
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
				K8sLogger.Warnf(`failed to deserialize: %s`, bodyString)
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
				K8sLogger.Warnf("Attempted to update resource state but resource has been removed from watcher: %+v", resource)
			}
			resourceContext.state = interfaces.WatchingFailed
			m.activeHandlers[resource] = resourceContext
			return
		}

		m.handlerMapLock.Lock()
		defer m.handlerMapLock.Unlock()
		resourceContext, ok := m.activeHandlers[resource]
		if !ok {
			K8sLogger.Warnf("Attempted to update resource state but resource has been removed from watcher: %+v", resource)
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

func (m *Watcher) Unwatch(resource interfaces.ResourceIdentifier) error {
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

func (m *Watcher) ListWatchedResources() []interfaces.ResourceIdentifier {
	m.handlerMapLock.Lock()
	defer m.handlerMapLock.Unlock()

	resources := make([]interfaces.ResourceIdentifier, len(m.activeHandlers))
	for r := range m.activeHandlers {
		resources = append(resources, r)
	}

	return resources
}

func (m *Watcher) State(resource interfaces.ResourceIdentifier) (interfaces.ResourceState, error) {
	m.handlerMapLock.Lock()
	defer m.handlerMapLock.Unlock()

	resourceContext, ok := m.activeHandlers[resource]
	if !ok {
		return interfaces.Unknown, fmt.Errorf("resource is not being watched")
	}

	return resourceContext.state, nil
}

func WatchAllResources(watcher interfaces.KubernetesWatcher) {
	// Retry watching resources with exponential backoff in case of failures
	err := retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}, apierrors.IsServiceUnavailable, func() error {
		for _, v := range utils.CONFIG.Iac.SyncWorkloads {
			err := watcher.Watch(interfaces.ResourceIdentifier{
				Name:         v.Name,
				Kind:         v.Kind,
				GroupVersion: v.Group,
			}, nil, nil, nil)
			if err != nil {
				K8sLogger.Errorf("failed to initialize watchhandler for resource %s %s: %s", v.Kind, v.Version, err.Error())
			} else {
				K8sLogger.Infof("🚀 Watching resource %s (%s)", v.Kind, v.Group)
			}
		}
		return nil
	})
	if err != nil {
		K8sLogger.Fatalf("Error watching resources: %s", err.Error())
	}
}

func SetStoreIfNeeded(kind string, namespace string, name string, obj *unstructured.Unstructured) {
	if kind == "Deployment" || kind == "ReplicaSet" || kind == "CronJob" || kind == "Pod" || kind == "Job" || kind == "Event" {
		err := store.GlobalStore.Set(obj, kind, namespace, name)
		if err != nil {
			K8sLogger.Errorf("Error setting object in store: %s", err.Error())
		}
		if kind == "Event" {
			var event v1.Event
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &event)
			if err != nil {
				return
			}
			processEvent(&event)
		}
	}
}

func DeleteFromStoreIfNeeded(kind string, namespace string, name string, obj *unstructured.Unstructured) {
	if kind == "Deployment" || kind == "ReplicaSet" || kind == "CronJob" || kind == "Pod" || kind == "Job" || kind == "Event" {
		err := store.GlobalStore.Delete(kind, namespace, name)
		if err != nil {
			K8sLogger.Errorf("Error deleting object in store: %s", err.Error())
		}
		if kind == "Event" {
			var event v1.Event
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &event)
			if err != nil {
				return
			}
			processEvent(&event)
		}
	}
	if kind == "PersistentVolume" {
		var pv v1.PersistentVolume
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &pv)
		if err != nil {
			return
		}
		handlePVDeletion(&pv)
	}
}

func InitAllWorkloads() {
	utils.Assert(IacManagerShouldWatchResources != nil, "func IacManagerShouldWatchResources has to be initialized")
	allResources, err := GetAvailableResources()
	if err != nil {
		K8sLogger.Errorf("Error getting available resources: %s", err.Error())
		return
	}
	for _, resource := range allResources {
		if IacManagerShouldWatchResources() {
			list, err := GetUnstructuredResourceList(resource.Group, resource.Version, resource.Kind, resource.Namespaced)
			if err != nil {
				K8sLogger.Errorf("Error getting resource list for %s: %s", resource.Kind, err.Error())
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
		K8sLogger.Errorf("Error creating provider for GetUnstructuredResourceList. Cannot continue: %s", err.Error())
		return nil, err
	}

	if namespaced {
		return provider.DynamicClient.Resource(schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: name,
		}).Namespace("").List(context.TODO(), metav1.ListOptions{})
	} else {
		return provider.DynamicClient.Resource(schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: name,
		}).List(context.TODO(), metav1.ListOptions{})
	}
}

func CreateUnstructuredResource(group, version, name string, namespaced bool, yamlData string) (*unstructured.Unstructured, error) {
	provider, err := NewKubeProvider()
	if provider == nil || err != nil {
		K8sLogger.Errorf("Error creating provider for CreateUnstructuredResource. Cannot continue: %s", err.Error())
		return nil, err
	}

	obj := &unstructured.Unstructured{}
	err = yaml.Unmarshal([]byte(yamlData), obj)
	if err != nil {
		return nil, err
	}

	if namespaced {
		return provider.DynamicClient.Resource(schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: name,
		}).Namespace(obj.GetNamespace()).Create(context.TODO(), obj, metav1.CreateOptions{})
	} else {
		return provider.DynamicClient.Resource(schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: name,
		}).Create(context.TODO(), obj, metav1.CreateOptions{})
	}
}

func UpdateUnstructuredResource(group, version, name string, namespaced bool, yamlData string) (*unstructured.Unstructured, error) {
	provider, err := NewKubeProvider()
	if provider == nil || err != nil {
		K8sLogger.Errorf("Error creating provider for UpdatedUnstructuredResource. Cannot continue: %s", err.Error())
		return nil, err
	}

	obj := &unstructured.Unstructured{}
	err = yaml.Unmarshal([]byte(yamlData), obj)
	if err != nil {
		return nil, err
	}

	if namespaced {
		return provider.DynamicClient.Resource(schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: name,
		}).Namespace(obj.GetNamespace()).Update(context.TODO(), obj, metav1.UpdateOptions{})
	} else {
		return provider.DynamicClient.Resource(schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: name,
		}).Update(context.TODO(), obj, metav1.UpdateOptions{})
	}
}

func DeleteUnstructuredResource(group, version, name string, namespaced bool, yamlData string) error {
	provider, err := NewKubeProvider()
	if provider == nil || err != nil {
		K8sLogger.Errorf("Error creating provider for watcher. Cannot continue: %s", err.Error())
		return err
	}

	obj := &unstructured.Unstructured{}
	err = yaml.Unmarshal([]byte(yamlData), obj)
	if err != nil {
		return err
	}

	if namespaced {
		return provider.DynamicClient.Resource(schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: name,
		}).Namespace(obj.GetNamespace()).Delete(context.TODO(), obj.GetName(), metav1.DeleteOptions{})
	} else {
		return provider.DynamicClient.Resource(schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: name,
		}).Delete(context.TODO(), obj.GetName(), metav1.DeleteOptions{})
	}
}

func DescribeUnstructuredResource(group, version, name string, namespaced bool, yamlData string) (string, error) {
	provider, err := NewKubeProvider()
	if provider == nil || err != nil {
		K8sLogger.Errorf("Error creating provider for watcher. Cannot continue: %s", err.Error())
		return "", err
	}

	restMapping := &meta.RESTMapping{
		Resource: schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: name,
		},
		GroupVersionKind: schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    name,
		},
	}

	obj := &unstructured.Unstructured{}
	err = yaml.Unmarshal([]byte(yamlData), obj)
	if err != nil {
		return "", err
	}

	describer, ok := describe.GenericDescriberFor(restMapping, &provider.ClientConfig)
	if !ok {
		fmt.Printf("Failed to get describer: %v\n", err)
		return "", err
	}

	output, err := describer.Describe(obj.GetNamespace(), obj.GetName(), describe.DescriberSettings{ShowEvents: true})
	if err != nil {
		fmt.Printf("Failed to describe resource: %v\n", err)
		return "", err
	}

	return output, nil
}

func GetK8sObjectFor(file string, namespaced bool) (interface{}, error) {
	provider, err := NewKubeProvider()
	if provider == nil || err != nil {
		K8sLogger.Errorf("Error creating provider for watcher. Cannot continue: %s", err.Error())
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
		res, err := provider.DynamicClient.Resource(schema.GroupVersionResource{
			Group:    obj.GroupVersionKind().Group,
			Version:  obj.GroupVersionKind().Version,
			Resource: resourceName,
		}).Namespace(obj.GetNamespace()).Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
		if err != nil {
			K8sLogger.Errorf("Error querying resource: %s", err.Error())
			return nil, err
		}
		return res.Object, nil
	} else {
		res, err := provider.DynamicClient.Resource(schema.GroupVersionResource{
			Group:    obj.GroupVersionKind().Group,
			Version:  obj.GroupVersionKind().Version,
			Resource: resourceName,
		}).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			K8sLogger.Errorf("Error listing resource: %s", err.Error())
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
		K8sLogger.Errorf("Error creating provider for watcher. Cannot continue: %s", err.Error())
		return nil, err
	}

	resources, err := provider.ClientSet.Discovery().ServerPreferredResources()
	if err != nil {
		K8sLogger.Errorf("Error discovering resources: %s", err.Error())
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
		K8sLogger.Errorf("Error getting available resources: %s", err.Error())
		return ""
	}

	bytes, err := yaml.Marshal(resources)
	if err != nil {
		K8sLogger.Errorf("Error serializing available resources: %s", err.Error())
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
