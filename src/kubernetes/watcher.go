package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/k8sclient"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

type Watcher struct {
	handlerMapLock sync.Mutex
	activeHandlers map[WatcherResourceIdentifier]resourceContext
	clientProvider k8sclient.K8sClientProvider
}

func NewWatcher(clientProvider k8sclient.K8sClientProvider) *Watcher {
	return &Watcher{
		handlerMapLock: sync.Mutex{},
		activeHandlers: make(map[WatcherResourceIdentifier]resourceContext, 0),
		clientProvider: clientProvider,
	}
}

type resourceContext struct {
	state    WatcherResourceState
	informer cache.SharedIndexInformer
	handler  cache.ResourceEventHandlerRegistration
}

func (self *Watcher) Watch(logger *slog.Logger, resource WatcherResourceIdentifier, onAdd WatcherOnAdd, onUpdate WatcherOnUpdate, onDelete WatcherOnDelete) error {
	assert.Assert(logger != nil)
	self.handlerMapLock.Lock()
	defer self.handlerMapLock.Unlock()

	for r := range self.activeHandlers {
		if resource == r {
			return fmt.Errorf("resources is already being watched")
		}
	}

	dynamicClient := self.clientProvider.DynamicClient()
	gv, err := schema.ParseGroupVersion(resource.GroupVersion)
	if err != nil {
		return fmt.Errorf("invalid groupVersion: %s", err)
	}

	informerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClient, time.Minute*10, v1.NamespaceAll, nil)

	resourceInformer := informerFactory.ForResource(CreateGroupVersionResource(gv.Group, gv.Version, resource.Name)).Informer()

	err = resourceInformer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		if err == io.EOF {
			return // closed normally, its fine
		}
		logger.Error(`Encountered error while watching resource`, "resourceName", resource.Name, "resourceKind", resource.Kind, "resourceGroupVersion", resource.GroupVersion, "error", err)
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
				logger.Warn(`failed to deserialize`, "resourceJson", bodyString)
				return
			}
			if onAdd != nil {
				onAdd(resource, unstructuredObj)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldUnstructuredObj, ok := oldObj.(*unstructured.Unstructured)
			if !ok {
				body, _ := json.Marshal(newObj)
				bodyString := string(body)
				logger.Warn(`failed to deserialize`, "resourceJson", bodyString)
				return
			}
			newUnstructuredObj, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				body, _ := json.Marshal(newObj)
				bodyString := string(body)
				logger.Warn(`failed to deserialize`, "resourceJson", bodyString)
				return
			}
			if onUpdate != nil {
				onUpdate(resource, oldUnstructuredObj, newUnstructuredObj)
			}
		},
		DeleteFunc: func(obj interface{}) {
			unstructuredObj, ok := obj.(*unstructured.Unstructured)
			if !ok {
				body, _ := json.Marshal(obj)
				bodyString := string(body)
				logger.Warn(`failed to deserialize`, "resourceJson", bodyString)
				return
			}
			if onDelete != nil {
				onDelete(resource, unstructuredObj)
			}
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add eventhandler: %s", err.Error())
	}

	go func() {
		stopCh := make(chan struct{})
		go resourceInformer.Run(stopCh)

		if !cache.WaitForCacheSync(stopCh, resourceInformer.HasSynced) {
			self.handlerMapLock.Lock()
			defer self.handlerMapLock.Unlock()
			resourceContext, ok := self.activeHandlers[resource]
			if !ok {
				logger.Warn("Attempted to update resource state but resource has been removed from watcher", "resource", resource)
			}
			resourceContext.state = WatchingFailed
			self.activeHandlers[resource] = resourceContext
			return
		}

		self.handlerMapLock.Lock()
		defer self.handlerMapLock.Unlock()
		resourceContext, ok := self.activeHandlers[resource]
		if !ok {
			logger.Warn("Attempted to update resource state but resource has been removed from watcher", "resource", resource)
		}
		resourceContext.state = Watching
		self.activeHandlers[resource] = resourceContext
	}()

	self.activeHandlers[resource] = resourceContext{
		state:    WatcherInitializing,
		informer: resourceInformer,
		handler:  handler,
	}

	return nil
}

func (m *Watcher) Unwatch(resource WatcherResourceIdentifier) error {
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

func (m *Watcher) ListWatchedResources() []WatcherResourceIdentifier {
	m.handlerMapLock.Lock()
	defer m.handlerMapLock.Unlock()

	resources := make([]WatcherResourceIdentifier, len(m.activeHandlers))
	for r := range m.activeHandlers {
		resources = append(resources, r)
	}

	return resources
}

func (m *Watcher) State(resource WatcherResourceIdentifier) (WatcherResourceState, error) {
	m.handlerMapLock.Lock()
	defer m.handlerMapLock.Unlock()

	resourceContext, ok := m.activeHandlers[resource]
	if !ok {
		return Unknown, fmt.Errorf("resource is not being watched")
	}

	return resourceContext.state, nil
}
