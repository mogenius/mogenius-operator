package kubernetes

import (
	"context"
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
	logger         *slog.Logger
}

func NewWatcher(logger *slog.Logger, clientProvider k8sclient.K8sClientProvider) *Watcher {
	return &Watcher{
		handlerMapLock: sync.Mutex{},
		activeHandlers: make(map[WatcherResourceIdentifier]resourceContext, 0),
		clientProvider: clientProvider,
		logger:         logger,
	}
}

type resourceContext struct {
	state     WatcherResourceState
	informer  cache.SharedIndexInformer
	handler   cache.ResourceEventHandlerRegistration
	cancelCtx context.CancelFunc
}

func (self *Watcher) Watch(logger *slog.Logger, resource WatcherResourceIdentifier, onAdd WatcherOnAdd, onUpdate WatcherOnUpdate, onDelete WatcherOnDelete) error {
	assert.Assert(logger != nil)
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
	go self.watchWithRetry(ctx, logger, resource, onAdd, onUpdate, onDelete)

	return nil
}

func (self *Watcher) watchWithRetry(ctx context.Context, logger *slog.Logger, resource WatcherResourceIdentifier, onAdd WatcherOnAdd, onUpdate WatcherOnUpdate, onDelete WatcherOnDelete) {
	backoff := time.Second
	maxBackoff := time.Minute * 2
	maxRetries := 20
	retryCount := 0

	for {
		select {
		case <-ctx.Done():
			logger.Info("Watcher context cancelled, stopping", "resource", resource)
			return
		default:
		}

		// Check if we've exceeded max retries
		if retryCount >= maxRetries {
			logger.Error("Max retry attempts reached, giving up on watcher",
				"resource", resource, "retries", retryCount)
			self.setWatcherState(resource, WatchingFailed)
			return
		}

		logger.Info("Starting watcher", "resource", resource, "attempt", retryCount+1)

		watcherDone := make(chan error, 1)
		go func() {
			err := self.startSingleWatcher(ctx, logger, resource, onAdd, onUpdate, onDelete)
			watcherDone <- err
		}()

		// Wait for watcher to complete or context to be cancelled
		select {
		case <-ctx.Done():
			logger.Info("Watcher context cancelled during execution", "resource", resource)
			return
		case err := <-watcherDone:
			if err != nil {
				retryCount++
				logger.Warn("Watcher failed, will retry",
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
				logger.Warn("Watcher completed successfully (this should not happen)", "resource", resource)
				return
			}
		}
	}
}

func (self *Watcher) startSingleWatcher(ctx context.Context, logger *slog.Logger, resource WatcherResourceIdentifier, onAdd WatcherOnAdd, onUpdate WatcherOnUpdate, onDelete WatcherOnDelete) error {
	dynamicClient := self.clientProvider.DynamicClient()
	gv, err := schema.ParseGroupVersion(resource.GroupVersion)
	if err != nil {
		return fmt.Errorf("invalid groupVersion: %s", err)
	}

	informerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClient, time.Minute*30, v1.NamespaceAll, nil)
	resourceInformer := informerFactory.ForResource(CreateGroupVersionResource(gv.Group, gv.Version, resource.Name)).Informer()

	// Enhanced error handler that can detect fatal errors
	err = resourceInformer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		if err == io.EOF {
			logger.Debug("Watch connection closed normally", "resource", resource)
			return // closed normally, its fine
		}
		logger.Error("Encountered error while watching resource",
			"resourceName", resource.Name,
			"resourceKind", resource.Kind,
			"resourceGroupVersion", resource.GroupVersion,
			"error", err)
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
				logger.Warn("failed to deserialize", "resourceJson", bodyString)
				return
			}
			if onAdd != nil {
				onAdd(resource, unstructuredObj)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldUnstructuredObj, ok := oldObj.(*unstructured.Unstructured)
			if !ok {
				body, _ := json.Marshal(oldObj)
				bodyString := string(body)
				logger.Warn("failed to deserialize old object", "resourceJson", bodyString)
				return
			}
			newUnstructuredObj, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				body, _ := json.Marshal(newObj)
				bodyString := string(body)
				logger.Warn("failed to deserialize new object", "resourceJson", bodyString)
				return
			}

			// Filter out resync updates - same resource version means no actual change
			if oldUnstructuredObj.GetResourceVersion() == newUnstructuredObj.GetResourceVersion() {
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
				logger.Warn("failed to deserialize", "resourceJson", bodyString)
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
	logger.Info("Watcher cache synced successfully", "resource", resource)
	self.setWatcherState(resource, Watching)

	// Keep the watcher running until context is cancelled
	<-ctx.Done()
	close(stopCh)

	logger.Info("Stopping watcher", "resource", resource)
	return nil
}

func (self *Watcher) setWatcherState(resource WatcherResourceIdentifier, state WatcherResourceState) {
	self.handlerMapLock.Lock()
	defer self.handlerMapLock.Unlock()

	if resourceContext, ok := self.activeHandlers[resource]; ok {
		resourceContext.state = state
		self.activeHandlers[resource] = resourceContext
	}
}

func (self *Watcher) updateResourceContext(resource WatcherResourceIdentifier, informer cache.SharedIndexInformer, handler cache.ResourceEventHandlerRegistration) {
	self.handlerMapLock.Lock()
	defer self.handlerMapLock.Unlock()

	if resourceContext, ok := self.activeHandlers[resource]; ok {
		resourceContext.informer = informer
		resourceContext.handler = handler
		self.activeHandlers[resource] = resourceContext
	}
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

	resources := []WatcherResourceIdentifier{}
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

func (self *Watcher) UnwatchAll() {
	for _, resource := range self.ListWatchedResources() {
		err := self.Unwatch(resource)
		if err != nil {
			self.logger.Error("failed to unwatch resource", "resource", resource, "error", err)
		}
	}
}
