package reconciler

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/metrics"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/watcher"
)

// ReconcileResult is returned by a ReconcileFunc to report the outcome for an
// object. A nil Err clears any previous status entry for that object.
type ReconcileResult struct {
	// Err is the reconciliation error. nil means success and removes any
	// existing status entry for the object.
	Err error
	// IsWarning downgrades an Err to a warning instead of an error. Has no
	// effect when Err is nil.
	IsWarning bool
}
type operation string

const (
	createOperation     operation = "create"
	updateOperation     operation = "update"
	deleteOperation     operation = "delete"
	backgroundOperation operation = "background"
)

// ReconcileFunc is called when an object needs reconciliation. It is invoked on
// add/update/delete events and, when a global interval is set, periodically for
// every cached object. The ctx is cancelled when Stop is called.
type ReconcileFunc func(ctx context.Context, obj *unstructured.Unstructured, operation operation) []ReconcileResult

// ResourceConfig registers one Kubernetes resource kind for watching.
type ResourceConfig struct {
	// Resource describes the Kubernetes resource to watch.
	Resource utils.ResourceDescriptor
	// Reconcile is called whenever an object needs to be reconciled.
	Reconcile ReconcileFunc
	// Filters are optional predicates; an object is only reconciled when all
	// filters return true. Applied before caching, so non-matching objects are
	// never stored or processed.
	Filters []ObjectFilter
}

// ObjectFilter is a predicate on an unstructured object.
type ObjectFilter func(*unstructured.Unstructured) bool

// NamespaceFilter returns an ObjectFilter that accepts only objects in ns.
func NamespaceFilter(ns string) ObjectFilter {
	return func(obj *unstructured.Unstructured) bool {
		return obj.GetNamespace() == ns
	}
}

// ObjectStatus holds the last reconciliation error or warning for one object.
type ObjectStatus struct {
	ResourceKind      string            `json:"resource_kind"`
	ResourceName      string            `json:"resource_name"`
	ResourceNamespace string            `json:"resource_namespace"`
	Result            []ReconcileResult `json:"message"`
}

// Reconciler watches a configurable set of Kubernetes resources and calls a
// per-resource handler on add/update/delete events and on a global timer.
type Reconciler interface {
	Start()
	Stop()
	Status() Status
}

// objectKey uniquely identifies a Kubernetes object across resource types.
type objectKey struct {
	kind      string
	namespace string
	name      string
}

// maxConcurrentReconciles bounds the number of Reconcile invocations in
// flight at once. Without a bound, a burst of events for 1000+ objects
// can spawn an equal number of goroutines and overload the k8s API
// client. The watcher callback blocks when the limit is reached, which
// gives us natural backpressure without needing a worker pool.
const maxConcurrentReconciles = 50

type genericReconciler struct {
	logger   *slog.Logger
	watcher  watcher.WatcherModule
	configs  []ResourceConfig
	interval time.Duration
	active   atomic.Bool
	caches   map[utils.ResourceDescriptor]*objectCache

	// reconcileSlots is a semaphore: send to acquire, receive to release.
	reconcileSlots chan struct{}

	statusMu    sync.RWMutex
	objectState map[objectKey]ObjectStatus
	lastUpdate  *time.Time

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newReconciler(
	logger *slog.Logger,
	clientProvider k8sclient.K8sClientProvider,
	interval time.Duration,
	configs []ResourceConfig,
) *genericReconciler {
	r := &genericReconciler{
		logger:         logger,
		watcher:        watcher.NewWatcher(logger.With("scope", "watcher"), clientProvider),
		configs:        configs,
		interval:       interval,
		caches:         make(map[utils.ResourceDescriptor]*objectCache, len(configs)),
		objectState:    make(map[objectKey]ObjectStatus),
		reconcileSlots: make(chan struct{}, maxConcurrentReconciles),
	}
	for _, cfg := range configs {
		r.caches[cfg.Resource] = newObjectCache()
	}
	return r
}

func (r *genericReconciler) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	r.ctx = ctx
	r.cancel = cancel
	r.active.Store(true)

	r.statusMu.Lock()
	r.objectState = make(map[objectKey]ObjectStatus)
	r.lastUpdate = nil
	r.statusMu.Unlock()

	for _, cfg := range r.configs {
		cache := r.caches[cfg.Resource]
		cache.clear()

		err := r.watcher.Watch(
			cfg.Resource,
			func(_ utils.ResourceDescriptor, obj *unstructured.Unstructured) {
				if !matchesFilters(cfg, obj) {
					return
				}
				cache.set(obj)
				metrics.SetReconcileTrackedObjects(cfg.Resource.Kind, cache.len())
				r.callHandler(ctx, cfg, obj, createOperation)
			},
			func(_ utils.ResourceDescriptor, oldObj *unstructured.Unstructured, newObj *unstructured.Unstructured) {
				if !matchesFilters(cfg, newObj) {
					return
				}
				cache.set(newObj)
				metrics.SetReconcileTrackedObjects(cfg.Resource.Kind, cache.len())
				// Informer resyncs redeliver every object unchanged, and
				// status-only patches don't bump metadata.generation (CRs
				// increment it on every non-metadata change; with a status
				// subresource, status writes are exempt). Skipping both
				// prevents resync reconcile storms and self-triggering
				// loops (reconcile -> status patch -> update event ->
				// reconcile). The periodic background sweep covers drift.
				if oldObj.GetResourceVersion() == newObj.GetResourceVersion() {
					return
				}
				if newObj.GetGeneration() != 0 && oldObj.GetGeneration() == newObj.GetGeneration() {
					return
				}
				r.callHandler(ctx, cfg, newObj, updateOperation)
			},
			func(_ utils.ResourceDescriptor, obj *unstructured.Unstructured) {
				if !matchesFilters(cfg, obj) {
					return
				}
				cache.remove(obj)
				metrics.SetReconcileTrackedObjects(cfg.Resource.Kind, cache.len())
				r.clearObjectStatus(obj)
				r.callHandler(ctx, cfg, obj, deleteOperation)
			},
		)
		if err != nil {
			r.logger.Error("failed to watch resource", "resource", cfg.Resource, "error", err)
		}
	}

	if r.interval > 0 {
		r.wg.Go(func() {
			ticker := time.NewTicker(r.interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					for _, cfg := range r.configs {
						cache := r.caches[cfg.Resource]
						for _, obj := range cache.snapshot() {
							r.callHandler(ctx, cfg, obj, backgroundOperation)
						}
					}
				}
			}
		})
	}
}

func (r *genericReconciler) Stop() {
	if !r.active.Swap(false) {
		return
	}
	r.cancel()
	r.watcher.UnwatchAll()
	r.wg.Wait()
	for resource, cache := range r.caches {
		cache.clear()
		metrics.SetReconcileTrackedObjects(resource.Kind, 0)
	}
	r.statusMu.Lock()
	r.objectState = make(map[objectKey]ObjectStatus)
	r.statusMu.Unlock()
}

func matchesFilters(cfg ResourceConfig, obj *unstructured.Unstructured) bool {
	for _, f := range cfg.Filters {
		if !f(obj) {
			return false
		}
	}
	return true
}

func (r *genericReconciler) callHandler(ctx context.Context, cfg ResourceConfig, obj *unstructured.Unstructured, operation operation) {
	objCopy := obj.DeepCopy()

	// Acquire a slot before spawning. Blocks (backpressures the watcher
	// callback) when maxConcurrentReconciles are already in flight.
	waitStart := time.Now()
	select {
	case r.reconcileSlots <- struct{}{}:
	case <-ctx.Done():
		return
	}
	metrics.ObserveReconcileQueueWait(time.Since(waitStart).Seconds())

	r.wg.Go(func() {
		defer func() { <-r.reconcileSlots }()
		start := time.Now()
		result := cfg.Reconcile(ctx, objCopy, operation)
		metrics.ObserveReconcileDuration(cfg.Resource.Kind, string(operation), time.Since(start).Seconds())
		r.recordResult(cfg.Resource, objCopy, result)
	})
}

// requeue re-reconciles every cached object of the given resource kind that
// matches the predicate. Reconcile handlers use this to refresh objects whose
// conditions depend on *other* objects (e.g. workspaces referencing a
// WorkspaceDashboard) without waiting for the next background sweep. The work
// runs on its own goroutine: callHandler blocks while all reconcile slots are
// taken, and the caller typically holds one of those slots — blocking here
// could deadlock the slot pool.
func (r *genericReconciler) requeue(resource utils.ResourceDescriptor, match func(*unstructured.Unstructured) bool) {
	if !r.active.Load() {
		return
	}
	r.wg.Go(func() {
		for _, cfg := range r.configs {
			if cfg.Resource != resource {
				continue
			}
			for _, obj := range r.caches[cfg.Resource].snapshot() {
				if match(obj) {
					r.callHandler(r.ctx, cfg, obj, backgroundOperation)
				}
			}
		}
	})
}

func (r *genericReconciler) recordResult(resource utils.ResourceDescriptor, obj *unstructured.Unstructured, result []ReconcileResult) {
	key := objectKey{
		kind:      resource.Kind,
		namespace: obj.GetNamespace(),
		name:      obj.GetName(),
	}
	now := time.Now()

	r.statusMu.Lock()
	defer r.statusMu.Unlock()

	r.lastUpdate = &now

	if len(result) == 0 {
		delete(r.objectState, key)
		return
	}

	r.objectState[key] = ObjectStatus{
		ResourceKind:      resource.Kind,
		ResourceName:      obj.GetName(),
		ResourceNamespace: obj.GetNamespace(),
		Result:            result,
	}
}

func (r *genericReconciler) clearObjectStatus(obj *unstructured.Unstructured) {
	r.statusMu.Lock()
	defer r.statusMu.Unlock()
	for k := range r.objectState {
		if k.namespace == obj.GetNamespace() && k.name == obj.GetName() {
			delete(r.objectState, k)
		}
	}
}
