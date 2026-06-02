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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// Helm stores up to 10 history revisions per release as Secrets of type
// helm.sh/release.v1. In clusters with many releases these dominate the
// Secret list (often thousands, several KB each) and push the initial
// cache sync past the 30s timeout below, so the Secret watcher never
// syncs and the cache stays empty. These secrets are never read from the
// operator's store (Helm uses its own storage driver, which talks to the
// API directly), so they are excluded server-side via a field selector on
// a dedicated informer factory for Secrets.
const helmReleaseSecretType = "helm.sh/release.v1"
const secretWatchFieldSelector = "type!=" + helmReleaseSecretType

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

	// Shared informer machinery. The previous code built a brand-new
	// DynamicSharedInformerFactory per Watch() call - so "Shared" in the
	// name shared nothing. With ~80 watched resource kinds (default API
	// server + CRDs) that was ~80 separate factories, each with its own
	// reflector goroutine and HTTP/2 watch stream to the API server.
	// One factory backs every Watch now; informers within it are looked
	// up by GVR (factory.ForResource returns the same instance on repeat
	// calls).
	factory dynamicinformer.DynamicSharedInformerFactory
	// secretFactory is a dedicated factory used only for the Secret GVR. It
	// carries a field selector that excludes Helm release-history secrets
	// (see secretWatchFieldSelector). It must be separate from `factory`
	// because the field selector references the `type` field, which only
	// Secrets expose - applying it to the shared factory would break the
	// List/Watch of every other resource kind.
	secretFactory dynamicinformer.DynamicSharedInformerFactory
	// factoryStopCh is allocated but intentionally never closed by this
	// package - the shared factory runs for the watcher's lifetime, the
	// OS reclaims its reflectors at process exit. Closing it from
	// UnwatchAll would break leader-election Stop/Start cycles (see the
	// comment in UnwatchAll for context).
	factoryStopCh chan struct{}
	// informersConfigured tracks which informers have had their Transform
	// and WatchErrorHandler set. Both can only be set before the informer
	// is started, and only need to be set once per GVR.
	informersConfigured map[schema.GroupVersionResource]bool

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
	self.factoryStopCh = make(chan struct{})
	self.factory = dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		clientProvider.DynamicClient(),
		utils.ResourceResyncTime,
		v1.NamespaceAll,
		nil,
	)
	self.secretFactory = dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		clientProvider.DynamicClient(),
		utils.ResourceResyncTime,
		v1.NamespaceAll,
		func(opts *metav1.ListOptions) {
			opts.FieldSelector = secretWatchFieldSelector
		},
	)
	self.informersConfigured = make(map[schema.GroupVersionResource]bool)
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
	// Backoff strategy: start at 1s, double up to 2min ("fast retry").
	// After fastRetryAttempts of fast retries without success, switch to
	// "slow lane" of one attempt per slowRetryInterval. We never give up
	// permanently - previously this loop stopped after 20 attempts (~40
	// min) and the resource was marked WatchingFailed forever, requiring
	// an operator restart to recover from any control-plane outage
	// longer than that window.
	const (
		fastRetryAttempts  = 20
		fastBackoffInitial = time.Second
		fastBackoffMax     = time.Minute * 2
		slowRetryInterval  = time.Minute * 5
	)
	backoff := fastBackoffInitial
	retryCount := 0

	for {
		select {
		case <-ctx.Done():
			self.logger.Info("Watcher context cancelled, stopping", "resource", resource)
			return
		default:
		}

		self.logger.Debug("Starting watcher", "resource", resource, "attempt", retryCount+1)

		watcherDone := make(chan error, 1)
		go func() {
			err := self.startSingleWatcher(ctx, resource, onAdd, onUpdate, onDelete)
			watcherDone <- err
		}()

		select {
		case <-ctx.Done():
			self.logger.Info("Watcher context cancelled during execution", "resource", resource)
			return
		case err := <-watcherDone:
			if err == nil {
				self.logger.Warn("Watcher completed successfully (this should not happen)", "resource", resource)
				return
			}
			retryCount++
			self.setWatcherState(resource, WatchingFailed)

			// First fastRetryAttempts: exponential backoff up to 2min.
			// After that: slow-lane retry every slowRetryInterval, forever,
			// so a transient API-server outage can self-heal.
			var sleep time.Duration
			if retryCount <= fastRetryAttempts {
				sleep = backoff
				backoff = min(backoff*2, fastBackoffMax)
				self.logger.Warn("Watcher failed, will retry",
					"resource", resource, "error", err,
					"attempt", retryCount, "backoff", sleep)
			} else {
				sleep = slowRetryInterval
				if retryCount == fastRetryAttempts+1 {
					self.logger.Error("Watcher still failing after fast-retry budget, switching to slow lane",
						"resource", resource, "error", err,
						"interval", slowRetryInterval)
				} else if retryCount%12 == 0 {
					// Every ~hour on the slow lane: re-log so the failure
					// stays visible in operator dashboards.
					self.logger.Warn("Watcher still failing on slow-lane retry",
						"resource", resource, "error", err,
						"slowAttempts", retryCount-fastRetryAttempts)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(sleep):
			}
		}
	}
}

// registerAndStartInformer holds handlerMapLock across ForResource,
// SetTransform, SetWatchErrorHandler and factory.Start so a concurrent
// Watch call can't get its informer started by our factory.Start while
// it's still mid-configuration. See the comment in startSingleWatcher
// for the race this prevents.
func (self *watcher) registerAndStartInformer(gvr schema.GroupVersionResource, resource utils.ResourceDescriptor) (cache.SharedIndexInformer, error) {
	self.handlerMapLock.Lock()
	defer self.handlerMapLock.Unlock()

	// Secrets are watched through a dedicated factory carrying a field
	// selector that drops Helm release-history secrets; every other kind
	// uses the shared factory. The factory.Start race documented in
	// startSingleWatcher is per-factory, so isolating Secrets onto their
	// own factory preserves that invariant.
	factory := self.factoryForGVR(gvr)
	resourceInformer := factory.ForResource(gvr).Informer()

	if !self.informersConfigured[gvr] {
		// Strip large metadata fields before caching to reduce in-process
		// memory usage. managedFields (server-side apply tracking) and
		// last-applied-configuration are never used by event handlers and
		// can be several KB per object.
		if err := resourceInformer.SetTransform(func(obj interface{}) (interface{}, error) {
			if u, ok := obj.(*unstructured.Unstructured); ok {
				u.SetManagedFields(nil)
				annotations := u.GetAnnotations()
				delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
				u.SetAnnotations(annotations)
			}
			return obj, nil
		}); err != nil {
			return nil, fmt.Errorf("failed to set transform: %s", err)
		}
		if err := resourceInformer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
			if err == io.EOF {
				self.logger.Debug("Watch connection closed normally", "resource", resource)
				return
			}
			if strings.Contains(err.Error(), "the server could not find the requested resource") {
				return
			}
			self.logger.Error("Encountered error while watching resource",
				"resourceName", resource.Plural,
				"resourceKind", resource.Kind,
				"resourceGroupVersion", resource.ApiVersion,
				"error", err)
		}); err != nil {
			return nil, fmt.Errorf("failed to set error watch handler: %s", err)
		}
		self.informersConfigured[gvr] = true
	}

	// Start the selected factory under the same lock. Idempotent for
	// already-running informers; this newly-registered one is launched.
	factory.Start(self.factoryStopCh)

	return resourceInformer, nil
}

// factoryForGVR returns the informer factory responsible for the given GVR.
// The core/v1 Secret GVR is routed to secretFactory (which excludes Helm
// release-history secrets via a field selector); everything else uses the
// shared factory.
func (self *watcher) factoryForGVR(gvr schema.GroupVersionResource) dynamicinformer.DynamicSharedInformerFactory {
	if gvr.Group == "" && gvr.Version == "v1" && gvr.Resource == "secrets" {
		return self.secretFactory
	}
	return self.factory
}

func (self *watcher) startSingleWatcher(ctx context.Context, resource utils.ResourceDescriptor, onAdd WatcherOnAdd, onUpdate WatcherOnUpdate, onDelete WatcherOnDelete) error {
	// IMPORTANT: ForResource + SetTransform + factory.Start MUST be
	// atomic under handlerMapLock. factory.Start iterates over every
	// informer currently registered on the factory, starts the ones not
	// yet running, and marks them as started. If goroutine A registers
	// informer A and calls Start while goroutine B has just called
	// ForResource(B) (which adds B to the factory's map) but not yet
	// SetTransform, then A's Start also runs B - B's SetTransform then
	// fails with "informer has already started" and B is permanently
	// unwatched until process restart. Holding the lock around the
	// whole register-configure-start sequence eliminates the window.
	gvr := self.createGroupVersionResource(resource.ApiVersion, resource.Plural)
	resourceInformer, err := self.registerAndStartInformer(gvr, resource)
	if err != nil {
		return err
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
			// Always invoke the generic onUpdate callback, even on resync
			// (same resourceVersion) - watcheractions.setStoreIfNeeded
			// needs the resync write to refresh the Valkey TTL, otherwise
			// static resources (Workspaces, Deployments, Secrets, ...)
			// vanish from the store once the initial TTL expires and never
			// reappear. The callback itself filters out resync events for
			// the downstream notifications (sendEventServerEvent +
			// aiManager.ProcessObject) so those don't fire on phantom
			// updates.
			if onUpdate != nil {
				onUpdate(resource, oldUnstructuredObj, newUnstructuredObj)
			}
			// Named-object subscribers (e.g. AI filter ConfigMap reload)
			// do NOT want resync phantoms - this was the original reason
			// the RV check existed. Apply it only here, not to onUpdate.
			if oldUnstructuredObj.GetResourceVersion() == newUnstructuredObj.GetResourceVersion() {
				return
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

	// Wait for this informer's cache to sync. Tied to the per-Watch ctx
	// so Unwatch can interrupt a stuck sync.
	syncTimeout := time.Second * 30
	syncCtx, syncCancel := context.WithTimeout(ctx, syncTimeout)
	defer syncCancel()

	syncDone := make(chan bool, 1)
	go func() {
		synced := cache.WaitForCacheSync(syncCtx.Done(), resourceInformer.HasSynced)
		syncDone <- synced
	}()

	select {
	case <-syncCtx.Done():
		_ = resourceInformer.RemoveEventHandler(handler)
		self.setWatcherState(resource, WatchingFailed)
		return fmt.Errorf("cache sync timeout after %v", syncTimeout)
	case synced := <-syncDone:
		if !synced {
			_ = resourceInformer.RemoveEventHandler(handler)
			self.setWatcherState(resource, WatchingFailed)
			return fmt.Errorf("failed to sync cache")
		}
	}

	self.logger.Debug("Watcher cache synced successfully", "resource", resource)
	self.setWatcherState(resource, Watching)

	// Block until Unwatch cancels ctx. The shared informer keeps running
	// in the factory even after we leave - any future Watch on the same
	// GVR will reuse the cached state. We just remove our event handler.
	<-ctx.Done()
	_ = resourceInformer.RemoveEventHandler(handler)

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
	// Intentionally do NOT close factoryStopCh here. UnwatchAll is invoked
	// from reconciler.Stop, which the leader-elector triggers on
	// OnLeadingEnded; the same operator process can win leadership again
	// later and call Start, which expects the factory to still be alive.
	// Closing the channel here used to wedge the second Start: new Watch
	// calls would register informers on a factory whose stopCh is already
	// closed, so the reflector goroutines exited immediately and no
	// events were ever delivered.
	//
	// The factory's reflectors and watch streams are now tied to the
	// watcher object's lifetime - they run until process exit (where the
	// OS reclaims them). Per-resource handlers are removed via the
	// individual Unwatch calls above, which is what actually stops events
	// from reaching this Watch's callbacks.
}

func (self *watcher) createGroupVersionResource(apiVersion string, plural string) schema.GroupVersionResource {
	gv, err := schema.ParseGroupVersion(apiVersion) // e.g., "apps/v1" or just "v1"
	if err != nil {
		self.logger.Error("invalid apiVersion", "apiVersion", apiVersion, "resourceName", plural, "error", err)
	}
	gvr := gv.WithResource(plural)
	return gvr
}
