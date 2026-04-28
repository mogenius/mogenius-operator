package reconciler

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"mogenius-operator/src/k8sclient"
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

type genericReconciler struct {
	logger   *slog.Logger
	watcher  watcher.WatcherModule
	configs  []ResourceConfig
	interval time.Duration
	active   atomic.Bool
	caches   map[utils.ResourceDescriptor]*objectCache

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
) Reconciler {
	r := &genericReconciler{
		logger:      logger,
		watcher:     watcher.NewWatcher(logger.With("scope", "watcher"), clientProvider),
		configs:     configs,
		interval:    interval,
		caches:      make(map[utils.ResourceDescriptor]*objectCache, len(configs)),
		objectState: make(map[objectKey]ObjectStatus),
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
		cfg := cfg
		cache := r.caches[cfg.Resource]
		cache.clear()

		err := r.watcher.Watch(
			cfg.Resource,
			func(_ utils.ResourceDescriptor, obj *unstructured.Unstructured) {
				cache.set(obj)
				r.callHandler(ctx, cfg, obj, createOperation)
			},
			func(_ utils.ResourceDescriptor, _ *unstructured.Unstructured, newObj *unstructured.Unstructured) {
				cache.set(newObj)
				r.callHandler(ctx, cfg, newObj, updateOperation)
			},
			func(_ utils.ResourceDescriptor, obj *unstructured.Unstructured) {
				cache.remove(obj)
				r.clearObjectStatus(obj)
				r.callHandler(ctx, cfg, obj, deleteOperation)
			},
		)
		if err != nil {
			r.logger.Error("failed to watch resource", "resource", cfg.Resource, "error", err)
		}
	}

	if r.interval > 0 {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
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
		}()
	}
}

func (r *genericReconciler) Stop() {
	if !r.active.Swap(false) {
		return
	}
	r.cancel()
	r.watcher.UnwatchAll()
	r.wg.Wait()
	for _, cache := range r.caches {
		cache.clear()
	}
	r.statusMu.Lock()
	r.objectState = make(map[objectKey]ObjectStatus)
	r.statusMu.Unlock()
}

func (r *genericReconciler) callHandler(ctx context.Context, cfg ResourceConfig, obj *unstructured.Unstructured, operation operation) {
	objCopy := obj.DeepCopy()
	go func() {
		result := cfg.Reconcile(ctx, objCopy, operation)
		r.recordResult(cfg.Resource, objCopy, result)
	}()
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
