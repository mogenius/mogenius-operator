package watcher

import (
	"context"
	"encoding/json"
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

type watcher struct {
	handlerMapLock sync.RWMutex
	activeHandlers map[utils.ResourceDescriptor]resourceContext
	clientProvider k8sclient.K8sClientProvider
	logger         *slog.Logger
}

func NewWatcher(logger *slog.Logger, clientProvider k8sclient.K8sClientProvider) WatcherModule {
	self := &watcher{}
	self.handlerMapLock = sync.RWMutex{}
	self.activeHandlers = make(map[utils.ResourceDescriptor]resourceContext, 0)
	self.clientProvider = clientProvider
	self.logger = logger

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

	handler, err := resourceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			unstructuredObj, ok := obj.(*unstructured.Unstructured)
			if !ok {
				body, _ := json.Marshal(obj)
				bodyString := string(body)
				self.logger.Warn("failed to deserialize", "resourceJson", bodyString)
				return
			}
			if onAdd != nil {
				onAdd(resource, unstructuredObj)
			}
		},
		UpdateFunc: func(oldObj, newObj any) {
			oldUnstructuredObj, ok := oldObj.(*unstructured.Unstructured)
			if !ok {
				body, _ := json.Marshal(oldObj)
				bodyString := string(body)
				self.logger.Warn("failed to deserialize old object", "resourceJson", bodyString)
				return
			}
			newUnstructuredObj, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				body, _ := json.Marshal(newObj)
				bodyString := string(body)
				self.logger.Warn("failed to deserialize new object", "resourceJson", bodyString)
				return
			}

			if onUpdate != nil {
				onUpdate(resource, oldUnstructuredObj, newUnstructuredObj)
			}
		},
		DeleteFunc: func(obj any) {
			unstructuredObj, ok := obj.(*unstructured.Unstructured)
			if !ok {
				body, _ := json.Marshal(obj)
				bodyString := string(body)
				self.logger.Warn("failed to deserialize", "resourceJson", bodyString)
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

	err := resourceContext.informer.RemoveEventHandler(resourceContext.handler)
	if err != nil {
		return fmt.Errorf("failed to remove event handler: %s", err.Error())
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
