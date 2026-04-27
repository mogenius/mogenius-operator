package watcher

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/utils"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// A generic kubernetes resource watcher
type WatcherModule interface {
	// Register a watcher for the given resource
	Watch(resource utils.ResourceDescriptor, onAdd WatcherOnAdd, onUpdate WatcherOnUpdate, onDelete WatcherOnDelete) error
	// Stop the watcher for the given resource
	Unwatch(resource utils.ResourceDescriptor) error
	// Query the status of the resource
	State(resource utils.ResourceDescriptor) (WatcherResourceState, error)
	// List all currently watched resources
	ListWatchedResources() []utils.ResourceDescriptor
	UnwatchAll()

	// OnObjectCreated registers a callback that fires when a specific object (by kind/namespace/name) is added.
	// Requires that the resource kind is already being watched via Watch.
	OnObjectCreated(kind, namespace, name string, cb func(*unstructured.Unstructured))
	// OnObjectUpdated registers a callback that fires when a specific object is updated.
	OnObjectUpdated(kind, namespace, name string, cb func(*unstructured.Unstructured))
	// OnObjectDeleted registers a callback that fires when a specific object is deleted.
	OnObjectDeleted(kind, namespace, name string, cb func(*unstructured.Unstructured))
}

type WatcherOnAdd func(resource utils.ResourceDescriptor, obj *unstructured.Unstructured)
type WatcherOnUpdate func(resource utils.ResourceDescriptor, oldObj *unstructured.Unstructured, newObj *unstructured.Unstructured)
type WatcherOnDelete func(resource utils.ResourceDescriptor, obj *unstructured.Unstructured)

type WatcherResourceState string

const (
	Unknown             WatcherResourceState = "Unknown"
	Watching            WatcherResourceState = "Watching"
	WatcherInitializing WatcherResourceState = "WatcherInitializing"
	WatchingFailed      WatcherResourceState = "WatchingFailed"
)

// objectSubscriptionKey identifies a specific Kubernetes object for targeted event subscriptions.
type objectSubscriptionKey struct {
	kind      string
	namespace string
	name      string
}

type watcher struct {
	handlerMapLock sync.RWMutex
	activeHandlers map[utils.ResourceDescriptor]resourceContext
	clientProvider k8sclient.K8sClientProvider
	logger         *slog.Logger

	objectSubsMu     sync.RWMutex
	objectSubsAdd    map[objectSubscriptionKey][]func(*unstructured.Unstructured)
	objectSubsUpdate map[objectSubscriptionKey][]func(*unstructured.Unstructured)
	objectSubsDelete map[objectSubscriptionKey][]func(*unstructured.Unstructured)
}

func NewWatcher(logger *slog.Logger, clientProvider k8sclient.K8sClientProvider) WatcherModule {
	self := &watcher{}
	self.handlerMapLock = sync.RWMutex{}
	self.activeHandlers = make(map[utils.ResourceDescriptor]resourceContext, 0)
	self.clientProvider = clientProvider
	self.logger = logger
	self.objectSubsAdd = make(map[objectSubscriptionKey][]func(*unstructured.Unstructured))
	self.objectSubsUpdate = make(map[objectSubscriptionKey][]func(*unstructured.Unstructured))
	self.objectSubsDelete = make(map[objectSubscriptionKey][]func(*unstructured.Unstructured))

	return self
}

type resourceContext struct {
	state     WatcherResourceState
	informer  cache.SharedIndexInformer
	handler   cache.ResourceEventHandlerRegistration
	cancelCtx context.CancelFunc
}

func (self *watcher) Watch(resource utils.ResourceDescriptor, onAdd WatcherOnAdd, onUpdate WatcherOnUpdate, onDelete WatcherOnDelete) error {
	assert.Assert(self.logger != nil)
	self.handlerMapLock.Lock()
	defer self.handlerMapLock.Unlock()

	for r := range self.activeHandlers {
		if resource == r {
			return fmt.Errorf("resource is already being watched")
		}
	}

	// Initialize the resource context early
	ctx, cancel := context.WithCancel(context.Background())
	resourceCtx := resourceContext{
		state:     WatcherInitializing,
		informer:  nil,    // Will be set when watcher starts
		handler:   nil,    // Will be set when watcher starts
		cancelCtx: cancel, // Store cancel function for cleanup
	}
	self.activeHandlers[resource] = resourceCtx

	// Start the watcher with retry logic in a goroutine
	go self.watchWithRetry(ctx, resource, onAdd, onUpdate, onDelete)

	return nil
}

func (self *watcher) watchWithRetry(ctx context.Context, resource utils.ResourceDescriptor, onAdd WatcherOnAdd, onUpdate WatcherOnUpdate, onDelete WatcherOnDelete) {
	backoff := time.Second
	maxBackoff := time.Minute * 2
	maxRetries := 20
	retryCount := 0

	for {
		select {
		case <-ctx.Done():
			self.logger.Info("Watcher context cancelled, stopping", "resource", resource)
			return
		default:
		}

		// Check if we've exceeded max retries
		if retryCount >= maxRetries {
			self.logger.Error("Max retry attempts reached, giving up on watcher",
				"resource", resource, "retries", retryCount)
			self.setWatcherState(resource, WatchingFailed)
			return
		}

		self.logger.Debug("Starting watcher", "resource", resource, "attempt", retryCount+1)

		watcherDone := make(chan error, 1)
		go func() {
			err := self.startSingleWatcher(ctx, resource, onAdd, onUpdate, onDelete)
			watcherDone <- err
		}()

		// Wait for watcher to complete or context to be cancelled
		select {
		case <-ctx.Done():
			self.logger.Info("Watcher context cancelled during execution", "resource", resource)
			return
		case err := <-watcherDone:
			if err != nil {
				retryCount++
				self.logger.Warn("Watcher failed, will retry",
					"resource", resource,
					"error", err,
					"attempt", retryCount,
					"backoff", backoff)

				self.setWatcherState(resource, WatchingFailed)

				// Exponential backoff before retry
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
					backoff = min(backoff*2, maxBackoff)
				}
			} else {
				// Successful completion (shouldn't normally happen unless context cancelled)
				self.logger.Warn("Watcher completed successfully (this should not happen)", "resource", resource)
				return
			}
		}
	}
}

func (self *watcher) startSingleWatcher(ctx context.Context, resource utils.ResourceDescriptor, onAdd WatcherOnAdd, onUpdate WatcherOnUpdate, onDelete WatcherOnDelete) error {
	dynamicClient := self.clientProvider.DynamicClient()

	informerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClient, utils.ResourceResyncTime, v1.NamespaceAll, nil)
	resourceInformer := informerFactory.ForResource(self.createGroupVersionResource(resource.ApiVersion, resource.Plural)).Informer()

	// Strip large metadata fields before caching to reduce in-process memory usage.
	// managedFields (server-side apply tracking) and last-applied-configuration
	// are never used by event handlers and can be several KB per object.
	resourceInformer.SetTransform(func(obj interface{}) (interface{}, error) {
		if u, ok := obj.(*unstructured.Unstructured); ok {
			u.SetManagedFields(nil)
			annotations := u.GetAnnotations()
			delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
			u.SetAnnotations(annotations)
		}
		return obj, nil
	})

	// Enhanced error handler that can detect fatal errors
	err := resourceInformer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		if err == io.EOF {
			self.logger.Debug("Watch connection closed normally", "resource", resource)
			return // closed normally, its fine
		}
		if strings.Contains(err.Error(), "the server could not find the requested resource") {
			return // Resource might have been deleted, no need to retry
		}
		self.logger.Error("Encountered error while watching resource",
			"resourceName", resource.Plural,
			"resourceKind", resource.Kind,
			"resourceGroupVersion", resource.ApiVersion,
			"error", err)
	})
	if err != nil {
		return fmt.Errorf("failed to set error watch handler: %s", err)
	}

	toUnstructured := func(obj any) (*unstructured.Unstructured, bool) {
		if u, ok := obj.(*unstructured.Unstructured); ok {
			return u, true
		}
		if d, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			if u, ok := d.Obj.(*unstructured.Unstructured); ok {
				return u, true
			}
		}
		return nil, false
	}

	handler, err := resourceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			unstructuredObj, ok := toUnstructured(obj)
			if !ok {
				self.logger.Warn("failed to deserialize", "type", fmt.Sprintf("%T", obj))
				return
			}
			if onAdd != nil {
				onAdd(resource, unstructuredObj)
			}
			self.fireObjectSubs(self.objectSubsAdd, unstructuredObj)
		},
		UpdateFunc: func(oldObj, newObj any) {
			oldUnstructuredObj, ok := toUnstructured(oldObj)
			if !ok {
				self.logger.Warn("failed to deserialize old object", "type", fmt.Sprintf("%T", oldObj))
				return
			}
			newUnstructuredObj, ok := toUnstructured(newObj)
			if !ok {
				self.logger.Warn("failed to deserialize new object", "type", fmt.Sprintf("%T", newObj))
				return
			}
			if onUpdate != nil {
				onUpdate(resource, oldUnstructuredObj, newUnstructuredObj)
			}
			self.fireObjectSubs(self.objectSubsUpdate, newUnstructuredObj)
		},
		DeleteFunc: func(obj any) {
			unstructuredObj, ok := toUnstructured(obj)
			if !ok {
				self.logger.Warn("failed to deserialize", "type", fmt.Sprintf("%T", obj))
				return
			}
			if onDelete != nil {
				onDelete(resource, unstructuredObj)
			}
			self.fireObjectSubs(self.objectSubsDelete, unstructuredObj)
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add eventhandler: %s", err.Error())
	}

	// Update the stored informer and handler
	self.updateResourceContext(resource, resourceInformer, handler)

	// Create stop channel that will be closed when context is cancelled
	stopCh := make(chan struct{})

	// Start the informer
	go resourceInformer.Run(stopCh)

	// Wait for cache sync with timeout
	syncTimeout := time.Second * 30
	syncCtx, syncCancel := context.WithTimeout(ctx, syncTimeout)
	defer syncCancel()

	syncDone := make(chan bool, 1)
	go func() {
		synced := cache.WaitForCacheSync(stopCh, resourceInformer.HasSynced)
		syncDone <- synced
	}()

	select {
	case <-syncCtx.Done():
		close(stopCh)
		self.setWatcherState(resource, WatchingFailed)
		return fmt.Errorf("cache sync timeout after %v", syncTimeout)
	case synced := <-syncDone:
		if !synced {
			close(stopCh)
			self.setWatcherState(resource, WatchingFailed)
			return fmt.Errorf("failed to sync cache")
		}
	}

	// Cache sync successful
	self.logger.Debug("Watcher cache synced successfully", "resource", resource)
	self.setWatcherState(resource, Watching)

	// Keep the watcher running until context is cancelled
	<-ctx.Done()
	close(stopCh)

	self.logger.Info("Stopping watcher", "resource", resource)
	return nil
}

func (self *watcher) setWatcherState(resource utils.ResourceDescriptor, state WatcherResourceState) {
	self.handlerMapLock.Lock()
	defer self.handlerMapLock.Unlock()

	if resourceContext, ok := self.activeHandlers[resource]; ok {
		resourceContext.state = state
		self.activeHandlers[resource] = resourceContext
	}
}

func (self *watcher) updateResourceContext(resource utils.ResourceDescriptor, informer cache.SharedIndexInformer, handler cache.ResourceEventHandlerRegistration) {
	self.handlerMapLock.Lock()
	defer self.handlerMapLock.Unlock()

	if resourceContext, ok := self.activeHandlers[resource]; ok {
		resourceContext.informer = informer
		resourceContext.handler = handler
		self.activeHandlers[resource] = resourceContext
	}
}

func (m *watcher) Unwatch(resource utils.ResourceDescriptor) error {
	m.handlerMapLock.Lock()
	defer m.handlerMapLock.Unlock()

	resourceContext, ok := m.activeHandlers[resource]
	if !ok {
		return fmt.Errorf("resource is not being watched")
	}

	// Cancel the context first to stop watchWithRetry and startSingleWatcher goroutines.
	resourceContext.cancelCtx()

	if resourceContext.informer != nil && resourceContext.handler != nil {
		err := resourceContext.informer.RemoveEventHandler(resourceContext.handler)
		if err != nil {
			return fmt.Errorf("failed to remove event handler: %s", err.Error())
		}
	}
	delete(m.activeHandlers, resource)

	return nil
}

func (m *watcher) ListWatchedResources() []utils.ResourceDescriptor {
	m.handlerMapLock.RLock()
	defer m.handlerMapLock.RUnlock()

	resources := []utils.ResourceDescriptor{}
	for r := range m.activeHandlers {
		resources = append(resources, r)
	}

	return resources
}

func (m *watcher) State(resource utils.ResourceDescriptor) (WatcherResourceState, error) {
	m.handlerMapLock.RLock()
	defer m.handlerMapLock.RUnlock()

	resourceContext, ok := m.activeHandlers[resource]
	if !ok {
		return Unknown, fmt.Errorf("resource is not being watched")
	}

	return resourceContext.state, nil
}

func (self *watcher) OnObjectCreated(kind, namespace, name string, cb func(*unstructured.Unstructured)) {
	key := objectSubscriptionKey{kind: kind, namespace: namespace, name: name}
	self.objectSubsMu.Lock()
	self.objectSubsAdd[key] = append(self.objectSubsAdd[key], cb)
	self.objectSubsMu.Unlock()
}

func (self *watcher) OnObjectUpdated(kind, namespace, name string, cb func(*unstructured.Unstructured)) {
	key := objectSubscriptionKey{kind: kind, namespace: namespace, name: name}
	self.objectSubsMu.Lock()
	self.objectSubsUpdate[key] = append(self.objectSubsUpdate[key], cb)
	self.objectSubsMu.Unlock()
}

func (self *watcher) OnObjectDeleted(kind, namespace, name string, cb func(*unstructured.Unstructured)) {
	key := objectSubscriptionKey{kind: kind, namespace: namespace, name: name}
	self.objectSubsMu.Lock()
	self.objectSubsDelete[key] = append(self.objectSubsDelete[key], cb)
	self.objectSubsMu.Unlock()
}

// fireObjectSubs fires any per-object subscriptions registered for this object's kind/namespace/name.
func (self *watcher) fireObjectSubs(subs map[objectSubscriptionKey][]func(*unstructured.Unstructured), obj *unstructured.Unstructured) {
	key := objectSubscriptionKey{kind: obj.GetKind(), namespace: obj.GetNamespace(), name: obj.GetName()}
	self.objectSubsMu.RLock()
	callbacks := subs[key]
	self.objectSubsMu.RUnlock()
	for _, cb := range callbacks {
		cb(obj)
	}
}

func (self *watcher) UnwatchAll() {
	for _, resource := range self.ListWatchedResources() {
		err := self.Unwatch(resource)
		if err != nil {
			self.logger.Error("failed to unwatch resource", "resource", resource, "error", err)
		}
	}
}

func (self *watcher) createGroupVersionResource(apiVersion string, plural string) schema.GroupVersionResource {
	gv, err := schema.ParseGroupVersion(apiVersion) // e.g., "apps/v1" or just "v1"
	if err != nil {
		self.logger.Error("invalid apiVersion", "apiVersion", apiVersion, "resourceName", plural, "error", err)
	}
	gvr := gv.WithResource(plural)
	return gvr
}
