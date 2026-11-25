package watcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/metrics"
	"mogenius-operator/src/utils"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
)

// Helm stores up to 10 history revisions per release as Secrets of type
// helm.sh/release.v1. In clusters with many releases these dominate the
// Secret list (often thousands, several KB each) and push the initial
// cache sync past the 30s timeout below, so the Secret watcher never
// syncs and the cache stays empty. These secrets are never read from the
// operator's store (Helm uses its own storage driver, which talks to the
// API directly), so they are excluded server-side via a field selector on
// the Secret informer (see listOptionsTweak).
const helmReleaseSecretType = "helm.sh/release.v1"
const secretWatchFieldSelector = "type!=" + helmReleaseSecretType

// Watch weight constants. Handlers with a lower weight fire before those with a
// higher weight, regardless of the order in which Watch was called. Use
// WatchWeightStore for primary write-through callbacks (e.g. Valkey) and
// WatchWeightReconcile for downstream logic that needs the store up to date.
const (
	WatchWeightStore     = 0
	WatchWeightReconcile = 10
)

// A generic kubernetes resource watcher
type WatcherModule interface {
	// Watch registers callbacks for the given resource at the given weight.
	// Multiple callers may Watch the same resource; on each event all
	// registered callbacks are called in ascending weight order — lower weight
	// fires first. If the informer is already fully synced when Watch is called,
	// existing objects are replayed through onAdd so the caller can populate its
	// local cache. Returns a cancel func that removes this registration and an
	// error if the informer cannot be started for a new resource.
	Watch(resource utils.ResourceDescriptor, weight int, onAdd WatcherOnAdd, onUpdate WatcherOnUpdate, onDelete WatcherOnDelete) (func(), error)
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

	// OnSynced registers a callback that is called once when the informer for
	// the given resource completes its initial cache sync. If the resource is
	// already in Watching state when OnSynced is called, the callback fires
	// immediately. Safe to call before or after Watch.
	OnSynced(resource utils.ResourceDescriptor, cb func())

	// WatchHelmReleaseSecrets starts a lightweight, metadata-only informer for
	// Helm release secrets (type=helm.sh/release.v1) and invokes onChange
	// (debounced) whenever one is added/updated/deleted. These secrets are
	// deliberately excluded from the generic Secret watcher (they would blow
	// the initial sync at scale); a metadata-only informer stays cheap because
	// it never pulls the (multi-KB, gzipped) release payload. It exists so the
	// release-list cache can be invalidated on ANY release change - including
	// out-of-band ones (helm CLI, other controllers) - not just operator-driven
	// mutations.
	WatchHelmReleaseSecrets(onChange func()) error
}

// weightedHandler holds a set of callbacks at a given weight.
type weightedHandler struct {
	id       int
	weight   int
	onAdd    WatcherOnAdd
	onUpdate WatcherOnUpdate
	onDelete WatcherOnDelete
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
	// WatchingOverflow: the kind holds more objects than maxObjectsPerKind,
	// so no informer was started for it (see checkObjectLimit). It counts as
	// settled for OnSynced so the store can still go ready without it, and is
	// re-checked every slowRetryInterval.
	WatchingOverflow WatcherResourceState = "Overflow"
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

	// handlersMu protects the per-resource weighted handler lists used by the
	// event fan-out. Separate from handlerMapLock to avoid deadlocks between
	// the Watch registration path and the event delivery path.
	handlersMu sync.RWMutex
	handlers   map[utils.ResourceDescriptor][]*weightedHandler
	handlerSeq int

	// informers holds one running informer per GVR, guarded by
	// handlerMapLock. Every Watch of the same GVR shares it (the previous
	// code built a factory per Watch() call, ~80 reflectors and watch streams
	// for one cluster). Each informer has its own stop channel rather than
	// living in a shared factory, so the object limit can stop exactly the
	// informer that grew too large and release its cache (see
	// stopOverflowingInformer). Informers otherwise run for the watcher's
	// lifetime: Unwatch only detaches handlers, so a leader-election
	// Stop/Start cycle reuses the warm cache.
	informers map[schema.GroupVersionResource]*informerEntry
	// helmSecretsStopCh is never closed: the Helm release secret informer
	// (WatchHelmReleaseSecrets) runs for the process lifetime.
	helmSecretsStopCh chan struct{}

	objectSubsMu     sync.RWMutex
	objectSubsAdd    map[objectSubscriptionKey][]func(*unstructured.Unstructured)
	objectSubsUpdate map[objectSubscriptionKey][]func(*unstructured.Unstructured)
	objectSubsDelete map[objectSubscriptionKey][]func(*unstructured.Unstructured)

	// syncedCallbacks holds per-resource callbacks fired once when the
	// informer cache sync completes (state transitions to Watching) or the
	// kind is given up as too large (WatchingOverflow).
	syncedCallbacks map[utils.ResourceDescriptor][]func()

	// dynamicClient backs every informer and the object-count probe.
	dynamicClient dynamic.Interface

	// maxObjectsPerKind is the object count above which a kind is not
	// watched; 0 disables the limit. See checkObjectLimit and
	// stopOverflowingInformer.
	maxObjectsPerKind int
	// overflowRecheckInterval is how often an overflowing kind is probed
	// again (slowRetryInterval; tests shorten it).
	overflowRecheckInterval time.Duration
}

// informerEntry is one running informer. objects counts the cached objects
// (adds minus deletes) so the object limit can be enforced at runtime
// without listing the cache.
type informerEntry struct {
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	stopOnce     sync.Once
	objects      atomic.Int64
	overflowedAt atomic.Int64
	gauge        prometheus.Gauge
}

// NewWatcher creates the watcher module. Kinds holding more than
// maxObjectsPerKind objects are not watched (0 disables the limit).
func NewWatcher(logger *slog.Logger, clientProvider k8sclient.K8sClientProvider, maxObjectsPerKind int) WatcherModule {
	self := newWatcher(logger, clientProvider.DynamicClient(), maxObjectsPerKind)
	self.clientProvider = clientProvider
	return self
}

// newWatcher builds the watcher on a dynamic client alone, so tests can run
// it against a fake. clientProvider is left nil; only WatchHelmReleaseSecrets
// needs it.
func newWatcher(logger *slog.Logger, dynamicClient dynamic.Interface, maxObjectsPerKind int) *watcher {
	self := &watcher{}
	self.maxObjectsPerKind = maxObjectsPerKind
	self.dynamicClient = dynamicClient
	self.handlerMapLock = sync.RWMutex{}
	self.activeHandlers = make(map[utils.ResourceDescriptor]resourceContext, 0)
	self.logger = logger
	self.informers = make(map[schema.GroupVersionResource]*informerEntry)
	self.helmSecretsStopCh = make(chan struct{})
	self.overflowRecheckInterval = slowRetryInterval
	self.objectSubsAdd = make(map[objectSubscriptionKey][]func(*unstructured.Unstructured))
	self.objectSubsUpdate = make(map[objectSubscriptionKey][]func(*unstructured.Unstructured))
	self.objectSubsDelete = make(map[objectSubscriptionKey][]func(*unstructured.Unstructured))
	self.syncedCallbacks = make(map[utils.ResourceDescriptor][]func())
	self.handlers = make(map[utils.ResourceDescriptor][]*weightedHandler)

	return self
}

type resourceContext struct {
	state     WatcherResourceState
	informer  cache.SharedIndexInformer
	handler   cache.ResourceEventHandlerRegistration
	cancelCtx context.CancelFunc
}

// insertWeighted inserts entry into handlers maintaining ascending weight order.
// Handlers with equal weight retain insertion order (stable within a weight).
func insertWeighted(handlers []*weightedHandler, entry *weightedHandler) []*weightedHandler {
	handlers = append(handlers, entry)
	for i := len(handlers) - 1; i > 0 && handlers[i].weight < handlers[i-1].weight; i-- {
		handlers[i], handlers[i-1] = handlers[i-1], handlers[i]
	}
	return handlers
}

func (self *watcher) Watch(resource utils.ResourceDescriptor, weight int, onAdd WatcherOnAdd, onUpdate WatcherOnUpdate, onDelete WatcherOnDelete) (func(), error) {
	assert.Assert(self.logger != nil)

	// Add to the weighted handler list under handlersMu before checking
	// activeHandlers, so the informer's fan-out sees this entry from the
	// moment the first event fires.
	self.handlersMu.Lock()
	if self.handlers == nil {
		self.handlers = make(map[utils.ResourceDescriptor][]*weightedHandler)
	}
	id := self.handlerSeq
	self.handlerSeq++
	entry := &weightedHandler{id: id, weight: weight, onAdd: onAdd, onUpdate: onUpdate, onDelete: onDelete}
	self.handlers[resource] = insertWeighted(self.handlers[resource], entry)

	// If the informer is already synced, replay existing objects so the caller
	// can populate its local cache without waiting for the next real event.
	var replay []any
	self.handlerMapLock.RLock()
	existing, alreadyWatching := self.activeHandlers[resource]
	if alreadyWatching && existing.state == Watching && existing.informer != nil {
		replay = existing.informer.GetStore().List()
	}
	self.handlerMapLock.RUnlock()
	self.handlersMu.Unlock()

	if !alreadyWatching {
		self.handlerMapLock.Lock()
		// Re-check under write lock — another goroutine may have started it.
		if _, exists := self.activeHandlers[resource]; !exists {
			ctx, cancel := context.WithCancel(context.Background())
			self.activeHandlers[resource] = resourceContext{
				state:     WatcherInitializing,
				cancelCtx: cancel,
			}
			go self.watchWithRetry(ctx, resource)
		}
		self.handlerMapLock.Unlock()
	}

	// Replay outside all locks.
	if onAdd != nil {
		for _, obj := range replay {
			if u, ok := obj.(*unstructured.Unstructured); ok {
				onAdd(resource, u)
			}
		}
	}

	return func() {
		self.handlersMu.Lock()
		defer self.handlersMu.Unlock()
		hs := self.handlers[resource]
		for i, h := range hs {
			if h.id == id {
				self.handlers[resource] = append(hs[:i], hs[i+1:]...)
				return
			}
		}
	}, nil
}

// slowRetryInterval is the retry pace of a watcher that keeps failing after
// its fast retries, and the re-check pace of an overflowing kind.
const slowRetryInterval = time.Minute * 5

func (self *watcher) watchWithRetry(ctx context.Context, resource utils.ResourceDescriptor) {
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
			err := self.startSingleWatcher(ctx, resource)
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

			// Too many objects: not a failure to retry fast but a deliberate
			// refusal. Re-check on the slow lane until the kind shrinks.
			var limitErr *objectLimitError
			if errors.As(err, &limitErr) {
				if self.markOverflow(resource) {
					self.logger.Error("Resource kind holds too many objects, not watching it to protect the operator from running out of memory; it is re-checked every few minutes",
						"resource", resource, "objects", limitErr.count, "limit", limitErr.limit,
						"recheckInterval", self.overflowRecheckInterval)
				}
				retryCount = 0
				backoff = fastBackoffInitial
				select {
				case <-ctx.Done():
					return
				case <-time.After(self.overflowRecheckInterval):
				}
				continue
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

// registerAndStartInformer returns the running informer for gvr, creating,
// configuring and starting it on first use. It holds handlerMapLock for the
// whole sequence so two concurrent Watch calls of the same GVR cannot both
// create one.
func (self *watcher) registerAndStartInformer(gvr schema.GroupVersionResource, resource utils.ResourceDescriptor) (*informerEntry, error) {
	self.handlerMapLock.Lock()
	defer self.handlerMapLock.Unlock()

	if entry, ok := self.informers[gvr]; ok {
		return entry, nil
	}

	resourceInformer := dynamicinformer.NewFilteredDynamicInformer(
		self.dynamicClient,
		gvr,
		v1.NamespaceAll,
		utils.ResourceResyncTime,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		listOptionsTweak(gvr),
	).Informer()

	// The transform is the only place that may mutate an object: it runs
	// once, before the object enters the informer cache. Event handlers
	// receive the cached pointer and share it with every other handler,
	// with the resync path and with the goroutines that marshal it, so
	// any write there is a concurrent map write.
	//
	// Strip large metadata fields to reduce in-process memory usage.
	// managedFields (server-side apply tracking) and
	// last-applied-configuration are never used by event handlers and
	// can be several KB per object.
	//
	// Stamp apiVersion/kind while we are here: Unstructured objects from
	// the dynamic client frequently arrive with empty TypeMeta, and the
	// store persists it so cross-kind readers can call GetKind().
	if err := resourceInformer.SetTransform(func(obj any) (any, error) {
		if u, ok := obj.(*unstructured.Unstructured); ok {
			prepareObject(u, resource)
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

	entry := &informerEntry{
		informer: resourceInformer,
		stopCh:   make(chan struct{}),
		gauge:    metrics.WatcherCachedObjects(resource.ApiVersion, resource.Kind),
	}
	// Registered before Run, so it counts every object from the initial
	// list on. A relist emits deletes for vanished objects, so adds minus
	// deletes stays the cache size.
	if _, err := resourceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(any) {
			count := entry.objects.Add(1)
			entry.gauge.Set(float64(count))
			if self.maxObjectsPerKind > 0 && count > int64(self.maxObjectsPerKind) {
				// async: stopping takes handlerMapLock, which a Watch call
				// may hold while waiting on this informer
				go self.stopOverflowingInformer(gvr, entry, count)
			}
		},
		DeleteFunc: func(any) {
			entry.gauge.Set(float64(entry.objects.Add(-1)))
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to add object counter: %s", err)
	}

	self.informers[gvr] = entry
	go resourceInformer.Run(entry.stopCh)
	return entry, nil
}

// stopOverflowingInformer stops an informer whose cache grew past the object
// limit and forgets it, so its cached objects can be garbage collected. The
// startSingleWatcher waiting on it sees stopCh close and reports the kind
// as overflowing; watchWithRetry then re-checks it on the slow lane.
func (self *watcher) stopOverflowingInformer(gvr schema.GroupVersionResource, entry *informerEntry, count int64) {
	entry.stopOnce.Do(func() {
		entry.overflowedAt.Store(count)

		self.handlerMapLock.Lock()
		if self.informers[gvr] == entry {
			delete(self.informers, gvr)
		}
		self.forgetInformerLocked(entry)
		self.handlerMapLock.Unlock()

		close(entry.stopCh)
		entry.gauge.Set(0)
	})
}

// forgetInformerLocked drops every resourceContext reference to a stopped
// informer so its cache can be garbage collected. Caller holds
// handlerMapLock.
func (self *watcher) forgetInformerLocked(entry *informerEntry) {
	for resource, rc := range self.activeHandlers {
		if rc.informer == entry.informer {
			rc.informer = nil
			rc.handler = nil
			self.activeHandlers[resource] = rc
		}
	}
}

// listOptionsTweak returns the server-side filter for gvr. Helm stores up to
// 10 history revisions per release as Secrets of type helm.sh/release.v1;
// they dominate the Secret list in clusters with many releases and are never
// read from the operator's store, so the Secret informer excludes them. The
// field selector only exists on Secrets, every other kind is unfiltered.
func listOptionsTweak(gvr schema.GroupVersionResource) dynamicinformer.TweakListOptionsFunc {
	if gvr.Group == "" && gvr.Version == "v1" && gvr.Resource == "secrets" {
		return func(opts *metav1.ListOptions) {
			opts.FieldSelector = secretWatchFieldSelector
		}
	}
	return nil
}

// prepareObject strips metadata no handler reads and stamps TypeMeta. It runs
// in the informer transform, before an object is shared with handlers -
// afterwards the object is read-only.
func prepareObject(u *unstructured.Unstructured, resource utils.ResourceDescriptor) {
	u.SetManagedFields(nil)
	annotations := u.GetAnnotations()
	delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
	u.SetAnnotations(annotations)
	if u.GetAPIVersion() == "" {
		u.SetAPIVersion(resource.ApiVersion)
	}
	if u.GetKind() == "" {
		u.SetKind(resource.Kind)
	}
}

func (self *watcher) startSingleWatcher(ctx context.Context, resource utils.ResourceDescriptor) error {
	gvr := self.createGroupVersionResource(resource.ApiVersion, resource.Plural)

	if err := self.checkObjectLimit(ctx, gvr); err != nil {
		return err
	}
	self.clearOverflow(resource)

	entry, err := self.registerAndStartInformer(gvr, resource)
	if err != nil {
		return err
	}
	resourceInformer := entry.informer

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
			self.fanOutAdd(resource, unstructuredObj)
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
			self.fanOutUpdate(resource, oldUnstructuredObj, newUnstructuredObj)
		},
		DeleteFunc: func(obj any) {
			unstructuredObj, ok := toUnstructured(obj)
			if !ok {
				self.logger.Warn("failed to deserialize", "type", fmt.Sprintf("%T", obj))
				return
			}
			self.fanOutDelete(resource, unstructuredObj)
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
	case <-entry.stopCh:
		// grew past the object limit while syncing
		return self.overflowError(entry)
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

	// Block until Unwatch cancels ctx or the informer is stopped for
	// exceeding the object limit. On Unwatch the informer keeps running -
	// any future Watch on the same GVR reuses the cached state; we just
	// remove our event handler.
	select {
	case <-entry.stopCh:
		return self.overflowError(entry)
	case <-ctx.Done():
	}
	_ = resourceInformer.RemoveEventHandler(handler)

	self.logger.Info("Stopping watcher", "resource", resource)
	return nil
}

// overflowError describes why a stopped informer was stopped. It also drops
// the resourceContext reference again: the informer may have been stopped
// before startSingleWatcher stored it (overflow during the initial sync), and
// that reference would keep the whole cache alive while the kind overflows.
func (self *watcher) overflowError(entry *informerEntry) error {
	self.handlerMapLock.Lock()
	self.forgetInformerLocked(entry)
	self.handlerMapLock.Unlock()
	return &objectLimitError{count: int(entry.overflowedAt.Load()), limit: self.maxObjectsPerKind}
}

// fanOutAdd delivers an added object to every registered handler in weight
// order, then to the per-object subscribers.
func (self *watcher) fanOutAdd(resource utils.ResourceDescriptor, obj *unstructured.Unstructured) {
	for _, h := range self.handlersFor(resource) {
		if h.onAdd != nil {
			h.onAdd(resource, obj)
		}
	}
	self.fireObjectSubs(self.objectSubsAdd, obj)
}

// fanOutUpdate delivers an update to every registered handler in weight order.
// All onUpdate callbacks run even on resync (same resourceVersion) - the
// store-write handler needs the resync to refresh the Valkey TTL; each
// callback filters phantom events for its own downstream work (event server,
// AI, reconcilers). Named-object subscribers (e.g. AI filter ConfigMap
// reload) do NOT get resync phantoms.
func (self *watcher) fanOutUpdate(resource utils.ResourceDescriptor, oldObj, newObj *unstructured.Unstructured) {
	for _, h := range self.handlersFor(resource) {
		if h.onUpdate != nil {
			h.onUpdate(resource, oldObj, newObj)
		}
	}
	if oldObj.GetResourceVersion() == newObj.GetResourceVersion() {
		return
	}
	self.fireObjectSubs(self.objectSubsUpdate, newObj)
}

// fanOutDelete delivers a deleted object to every registered handler in
// weight order, then to the per-object subscribers.
func (self *watcher) fanOutDelete(resource utils.ResourceDescriptor, obj *unstructured.Unstructured) {
	for _, h := range self.handlersFor(resource) {
		if h.onDelete != nil {
			h.onDelete(resource, obj)
		}
	}
	self.fireObjectSubs(self.objectSubsDelete, obj)
}

// handlersFor returns a snapshot of the resource's handlers, so callbacks run
// without holding handlersMu (a callback may call Watch).
func (self *watcher) handlersFor(resource utils.ResourceDescriptor) []*weightedHandler {
	self.handlersMu.RLock()
	defer self.handlersMu.RUnlock()
	return append([]*weightedHandler(nil), self.handlers[resource]...)
}

// objectLimitError reports a kind that holds more objects than the watcher
// is allowed to cache.
type objectLimitError struct {
	count int
	limit int
}

func (e *objectLimitError) Error() string {
	return fmt.Sprintf("resource holds %d objects, more than the limit of %d", e.count, e.limit)
}

// checkObjectLimit counts the kind's objects with a single LIST of one item
// (the API server reports the rest as remainingItemCount) and refuses to
// start an informer when there are more than maxObjectsPerKind. The informer's
// initial LIST loads every object into memory at once, before any handler
// could count them, so an oversized kind has to be caught here; growth after
// the start is caught by the counter in registerAndStartInformer.
//
// A running informer is not checked again: its counter covers it. Errors of the probe itself are not fatal - the informer
// runs into the same error and reports it through the usual retry path.
func (self *watcher) checkObjectLimit(ctx context.Context, gvr schema.GroupVersionResource) error {
	if self.maxObjectsPerKind <= 0 {
		return nil
	}
	self.handlerMapLock.RLock()
	_, running := self.informers[gvr]
	self.handlerMapLock.RUnlock()
	if running {
		return nil
	}

	opts := metav1.ListOptions{Limit: 1}
	if tweak := listOptionsTweak(gvr); tweak != nil {
		tweak(&opts)
	}
	probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	list, err := self.dynamicClient.Resource(gvr).List(probeCtx, opts)
	if err != nil {
		self.logger.Debug("object count probe failed, starting the informer anyway", "gvr", gvr.String(), "error", err)
		return nil
	}
	count, known := listObjectCount(list)
	if known && count > self.maxObjectsPerKind {
		return &objectLimitError{count: count, limit: self.maxObjectsPerKind}
	}
	return nil
}

// listObjectCount returns the total number of objects behind a (possibly
// paginated) list response. known is false when the server did not report
// remainingItemCount although more pages exist.
func listObjectCount(list *unstructured.UnstructuredList) (count int, known bool) {
	count = len(list.Items)
	if remaining := list.GetRemainingItemCount(); remaining != nil {
		return count + int(*remaining), true
	}
	return count, list.GetContinue() == ""
}

// markOverflow switches the resource to WatchingOverflow and reports whether
// it was not in that state before (so the caller logs only the transition).
func (self *watcher) markOverflow(resource utils.ResourceDescriptor) bool {
	previous, _ := self.State(resource)
	self.setWatcherState(resource, WatchingOverflow)
	metrics.SetWatcherOverflow(resource.ApiVersion, resource.Kind, true)
	return previous != WatchingOverflow
}

// clearOverflow resets the overflow gauge once a kind fits the limit again.
func (self *watcher) clearOverflow(resource utils.ResourceDescriptor) {
	if state, _ := self.State(resource); state != WatchingOverflow {
		return
	}
	self.setWatcherState(resource, WatcherInitializing)
	metrics.SetWatcherOverflow(resource.ApiVersion, resource.Kind, false)
	self.logger.Info("Resource kind is below the object limit again, watching it", "resource", resource)
}

func (self *watcher) setWatcherState(resource utils.ResourceDescriptor, state WatcherResourceState) {
	self.handlerMapLock.Lock()
	defer self.handlerMapLock.Unlock()

	if resourceContext, ok := self.activeHandlers[resource]; ok {
		resourceContext.state = state
		self.activeHandlers[resource] = resourceContext
	}

	if state == Watching || state == WatchingOverflow {
		cbs := self.syncedCallbacks[resource]
		delete(self.syncedCallbacks, resource)
		for _, cb := range cbs {
			go cb()
		}
	}
}

func (self *watcher) OnSynced(resource utils.ResourceDescriptor, cb func()) {
	self.handlerMapLock.Lock()
	defer self.handlerMapLock.Unlock()

	if ctx, ok := self.activeHandlers[resource]; ok && (ctx.state == Watching || ctx.state == WatchingOverflow) {
		go cb()
		return
	}
	self.syncedCallbacks[resource] = append(self.syncedCallbacks[resource], cb)
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

	// The consumers' handler registrations are kept: Unwatch stops the
	// informer (e.g. its CRD was deleted), it does not unsubscribe anyone.
	// Dropping them silently disconnected the reconciler from a kind whose
	// CRD was deleted and re-created until the next restart, since only the
	// store re-registers. When the kind is watched again, the new informer
	// serves every registered handler; consumers that re-register replace
	// their old registration via the unregister func Watch returned.

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
	// Intentionally do NOT stop the informers here. UnwatchAll is invoked
	// from reconciler.Stop, which the leader-elector triggers on
	// OnLeadingEnded; the same operator process can win leadership again
	// later and call Start, which expects the informers to still be alive.
	// Stopping them here used to wedge the second Start: new Watch calls
	// found informers whose stop channel was already closed, so the
	// reflector goroutines exited immediately and no events were ever
	// delivered.
	//
	// The informers' reflectors and watch streams are tied to the watcher
	// object's lifetime - they run until process exit (where the OS
	// reclaims them) unless the object limit stops one. Per-resource
	// handlers are removed via the individual Unwatch calls above, which is
	// what actually stops events from reaching this Watch's callbacks.
}

// WatchHelmReleaseSecrets - see the interface doc. The informer is metadata
// only (no release payload decode), runs for the watcher's lifetime on
// helmSecretsStopCh, and coalesces bursts of events (e.g. the initial-sync
// storm, or a single `helm upgrade` writing several secrets) into one onChange
// via a short debounce.
func (self *watcher) WatchHelmReleaseSecrets(onChange func()) error {
	assert.Assert(self.logger != nil)
	if onChange == nil {
		return fmt.Errorf("onChange must not be nil")
	}

	metaClient, err := metadata.NewForConfig(self.clientProvider.ClientConfig())
	if err != nil {
		return fmt.Errorf("create metadata client: %w", err)
	}

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	informer := metadatainformer.NewFilteredMetadataInformer(
		metaClient,
		gvr,
		v1.NamespaceAll,
		utils.ResourceResyncTime,
		nil,
		func(opts *metav1.ListOptions) {
			opts.FieldSelector = "type=" + helmReleaseSecretType
		},
	).Informer()

	debounced := newDebouncer(2*time.Second, onChange)
	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(any) { debounced.trigger() },
		UpdateFunc: func(any, any) { debounced.trigger() },
		DeleteFunc: func(any) { debounced.trigger() },
	}); err != nil {
		return fmt.Errorf("add helm release secret event handler: %w", err)
	}

	if err := informer.SetWatchErrorHandler(func(_ *cache.Reflector, err error) {
		if err == io.EOF {
			return
		}
		self.logger.Warn("Helm release secret watcher error", "error", err)
	}); err != nil {
		return fmt.Errorf("set watch error handler: %w", err)
	}

	go informer.Run(self.helmSecretsStopCh)
	self.logger.Info("Watching Helm release secrets for release-list cache invalidation")
	return nil
}

// debouncer coalesces rapid trigger() calls into a single fn() invocation that
// fires once no trigger has happened for delay.
type debouncer struct {
	mu    sync.Mutex
	timer *time.Timer
	delay time.Duration
	fn    func()
}

func newDebouncer(delay time.Duration, fn func()) *debouncer {
	return &debouncer{delay: delay, fn: fn}
}

func (d *debouncer) trigger() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.delay, d.fn)
}

func (self *watcher) createGroupVersionResource(apiVersion string, plural string) schema.GroupVersionResource {
	gv, err := schema.ParseGroupVersion(apiVersion) // e.g., "apps/v1" or just "v1"
	if err != nil {
		self.logger.Error("invalid apiVersion", "apiVersion", apiVersion, "resourceName", plural, "error", err)
	}
	gvr := gv.WithResource(plural)
	return gvr
}
