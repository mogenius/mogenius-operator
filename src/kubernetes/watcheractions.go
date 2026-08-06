package kubernetes

import (
	"context"
	"fmt"
	"mogenius-operator/src/ai"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/watcher"
	"mogenius-operator/src/websocket"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/kubectl/pkg/describe"
	"sigs.k8s.io/yaml"
)

const (
	VALKEY_RESOURCE_PREFIX = "resources"
)

// deprecatedResources are skipped during resource discovery because the API
// version is deprecated and a non-deprecated replacement is already watched.
// Watching them produces noisy client-go deprecation warnings and stores
// duplicate data in Valkey. Keyed by "groupVersion/kind".
//
//   - v1/Endpoints: replaced by discovery.k8s.io/v1 EndpointSlice; scheduled
//     for removal in Kubernetes 1.33+.
var deprecatedResources = map[string]struct{}{
	"v1/Endpoints": {},
}

func isDeprecatedResource(groupVersion, kind string) bool {
	_, ok := deprecatedResources[groupVersion+"/"+kind]
	return ok
}

type GetUnstructuredNamespaceResourceListRequest struct {
	Namespace string                      `json:"namespace" validate:"required"`
	Whitelist []*utils.ResourceDescriptor `json:"whitelist"`
	Blacklist []*utils.ResourceDescriptor `json:"blacklist"`
}

// lastWatchCheckStart throttles WatchStoreResources; guarded by a mutex
// because callers include the CRD debounce timer goroutine.
var (
	lastWatchCheckMu    sync.Mutex
	lastWatchCheckStart time.Time
)

func WatchStoreResources(wm watcher.WatcherModule, aiManager ai.AiManager, eventClient websocket.WebsocketClient) error {
	start := time.Now()

	// function should not be called more often than every 5 seconds
	// to avoid too many calls to the k8s api server
	// which can lead to rate limiting
	lastWatchCheckMu.Lock()
	if time.Since(lastWatchCheckStart) < 5*time.Second {
		lastWatchCheckMu.Unlock()
		return nil
	}
	lastWatchCheckStart = time.Now()
	lastWatchCheckMu.Unlock()

	resources, err := GetAvailableResources()
	if err != nil {
		return err
	}

	// Signal store readiness once every resource's informer has completed its
	// initial cache sync. OnSynced is registered before Watch so no sync
	// completion can slip between the two calls. An atomic counter tracks
	// pending resources; when it reaches zero MarkStoreReady is called.
	// sync.Once inside MarkStoreReady makes this idempotent across repeated
	// WatchStoreResources invocations (e.g. after CRD additions).
	pending := int64(len(resources))
	if pending == 0 {
		store.MarkStoreReady()
		return nil
	}

	// Each resource may settle its slot only once: it either syncs (OnSynced)
	// or fails to register a watcher (the error path below). Without the guard
	// a resource that does both would decrement twice and the counter could
	// skip past zero, leaving the store permanently "not ready".
	settleOnce := make([]sync.Once, len(resources))
	settle := func(i int) {
		settleOnce[i].Do(func() {
			if atomic.AddInt64(&pending, -1) == 0 {
				store.MarkStoreReady()
				k8sLogger.Info("store is warm: all watched resources completed their initial sync")
			}
		})
	}

	var firstWatchErr error
	for i, res := range resources {
		wm.OnSynced(res, func() { settle(i) })

		err := wm.Watch(res, func(resource utils.ResourceDescriptor, obj *unstructured.Unstructured) {
			setStoreIfNeeded(resource.ApiVersion, obj.GetName(), resource.Kind, obj.GetNamespace(), obj)
			handleCRDAddition(wm, aiManager, eventClient, resource)
			aiManager.ProcessObject(obj, "add", res)

			// suppress the add events for the first 10 seconds (because all resources are added initially)
			if time.Since(start) < 10*time.Second {
				return
			}
			sendEventServerEvent(eventClient, res.ApiVersion, resource.Kind, obj.GetName(), "add", obj)
		}, func(resource utils.ResourceDescriptor, oldObj, newObj *unstructured.Unstructured) {
			// Always refresh the Valkey entry so the TTL stays alive.
			// SharedInformer resync delivers UpdateFunc every
			// ResourceResyncTime (30 min) with oldRV == newRV, and the
			// Valkey TTL is ResourceResyncTime*2 (60 min) - dropping the
			// resync here used to evict every static resource (Workspaces,
			// Deployments, Secrets, Namespaces) from the store after the
			// initial 60-minute window.
			setStoreIfNeeded(resource.ApiVersion, newObj.GetName(), resource.Kind, newObj.GetNamespace(), newObj)

			// Filter out resync updates for downstream notifications: same
			// resource version means no actual change, so we don't want to
			// re-emit the change to the event server or re-run AI tasks.
			if oldObj.GetResourceVersion() == newObj.GetResourceVersion() {
				return
			}
			sendEventServerEvent(eventClient, resource.ApiVersion, resource.Kind, newObj.GetName(), "update", newObj)
			aiManager.ProcessObject(newObj, "update", res)
		}, func(resource utils.ResourceDescriptor, obj *unstructured.Unstructured) {
			deleteFromStoreIfNeeded(resource.ApiVersion, obj.GetName(), resource.Kind, obj.GetNamespace(), obj)
			if resource.Kind == "Pod" {
				store.ClearOwnerCachePodEntry(obj.GetNamespace(), obj.GetName())
			}
			sendEventServerEvent(eventClient, resource.ApiVersion, resource.Kind, obj.GetName(), "delete", obj)
			handleCRDDeletion(wm, resource, obj)
			aiManager.ProcessObject(obj, "delete", res)
		})
		if err != nil {
			if !strings.Contains(err.Error(), "resource is already being watched") {
				// Keep going instead of aborting the loop: one resource that
				// cannot be watched (missing RBAC, a CRD served by a broken
				// conversion webhook, ...) used to leave every later resource
				// unwatched and its readiness slot unsettled, so the store never
				// went ready. Its slot is settled here so the remaining kinds can
				// still take the store to ready.
				k8sLogger.Error("failed to initialize watchhandler for resource", "ApiVersion", res.ApiVersion, "kind", res.Kind, "error", err)
				if firstWatchErr == nil {
					firstWatchErr = err
				}
				settle(i)
			}
		} else {
			k8sLogger.Info("🚀 Watching resource", "kind", res.Kind, "plural", res.Plural)
		}
	}
	return firstWatchErr
}

var (
	crdDebounceTimer *time.Timer
	crdDebounceMutex sync.Mutex
)

// no matter how many CRD addition events we get in a short time frame
// this method will debounce them and only execute the logic once after 3 seconds
func handleCRDAddition(wm watcher.WatcherModule, aiManager ai.AiManager, eventClient websocket.WebsocketClient, resource utils.ResourceDescriptor) {
	if resource.Kind == "CustomResourceDefinition" {
		crdDebounceMutex.Lock()
		defer crdDebounceMutex.Unlock()

		// Cancel existing timer if it exists
		if crdDebounceTimer != nil {
			crdDebounceTimer.Stop()
		}

		// Create new timer that executes after 3 seconds
		crdDebounceTimer = time.AfterFunc(3*time.Second, func() {
			resetAvailableResourceCache()

			res, err := GetAvailableResources()
			if err != nil {
				k8sLogger.Error("Error getting available resources", "error", err)
				return
			}
			currentlyWatchedResources := wm.ListWatchedResources()
			if len(res) != len(currentlyWatchedResources) {
				err := WatchStoreResources(wm, aiManager, eventClient)
				if err != nil {
					k8sLogger.Error("Error watching store resources", "error", err)
				}
			}
		})
	}
}

// handleCRDDeletion stops watching the custom resource kind described by a
// deleted CRD and purges its stored resources. The descriptor to unwatch must
// be derived from the deleted CRD's spec — deriving it from `resource` (the
// watcher that delivered the event, i.e. CustomResourceDefinition itself)
// unwatched the CRD watcher instead, so the first CRD deletion made the
// operator blind to every subsequent CRD event: later deletions never reached
// the store, new CRDs were no longer discovered, and CRD entries stopped
// being resynced until the operator restarted.
func handleCRDDeletion(wm watcher.WatcherModule, resource utils.ResourceDescriptor, obj *unstructured.Unstructured) {
	if resource.Kind != "CustomResourceDefinition" {
		return
	}

	plural, _, _ := unstructured.NestedString(obj.Object, "spec", "names", "plural")
	kind, _, _ := unstructured.NestedString(obj.Object, "spec", "names", "kind")
	group, _, _ := unstructured.NestedString(obj.Object, "spec", "group")
	if plural == "" || kind == "" || group == "" {
		k8sLogger.Error("Error parsing deleted CRD for unwatching", "plural", plural, "kind", kind, "group", group)
		return
	}

	// the deleted kind must not be re-registered by a WatchStoreResources run
	// that still sees it in the discovery cache
	resetAvailableResourceCache()

	// The kind's watcher was registered with the server's preferred version at
	// discovery time; match by group/kind/plural against the active watchers
	// instead of guessing which of spec.versions that was.
	groupPrefix := group + "/"
	for _, watched := range wm.ListWatchedResources() {
		if watched.Kind != kind || watched.Plural != plural || !strings.HasPrefix(watched.ApiVersion, groupPrefix) {
			continue
		}
		if err := wm.Unwatch(watched); err != nil {
			k8sLogger.Error("Error unwatching resource of deleted CRD", "kind", watched.Kind, "apiVersion", watched.ApiVersion, "error", err)
		} else {
			k8sLogger.Info("STOP Watching resource", "kind", watched.Kind, "apiVersion", watched.ApiVersion)
		}

		// Purge the kind's stored resources right away: the cascade deletes of
		// its instances may never have reached the watcher, and with the kind
		// unwatched nothing refreshes or prunes the leftover entries — they
		// would linger in the paginated index until their TTL expires.
		if err := store.DropResourcesByKind(valkeyClient, watched.ApiVersion, watched.Kind, k8sLogger); err != nil {
			k8sLogger.Error("Error purging stored resources of deleted CRD", "kind", watched.Kind, "apiVersion", watched.ApiVersion, "error", err)
		}
	}
}

func setStoreIfNeeded(apiVersion string, resourceName string, kind string, namespace string, obj *unstructured.Unstructured) {
	obj = removeUnusedFieds(obj)

	// store primary key + ZSET indexes (by-creation, by-name) in a single
	// MULTI/EXEC so paginated readers never observe an index member whose
	// primary key is missing. apiVersion/kind/namespace/name come from the
	// watcher's ResourceDescriptor because obj.GetAPIVersion()/GetKind() are
	// often empty on DynamicClient-sourced Unstructured objects.
	err := store.SetResourceWithIndex(valkeyClient, apiVersion, kind, namespace, resourceName, obj, utils.ResourceResyncTime*2)
	if err != nil {
		k8sLogger.Error("Error setting object in store", "error", err)
	}
}

func sendEventServerEvent(eventClient websocket.WebsocketClient, apiVersion, kind, name, eventType string, obj *unstructured.Unstructured) {
	datagram := structs.CreateDatagramForClusterEvent("ClusterEvent", apiVersion, kind, name, eventType, obj)

	// send the datagram to the event server
	go func() {
		err := eventClient.WriteJSON(datagram)
		if err != nil {
			k8sLogger.Error("Error sending data to EventServer", "error", err)

		}
	}()
}

func deleteFromStoreIfNeeded(apiVersion string, resourceName string, kind string, namespace string, obj *unstructured.Unstructured) {
	if kind == "PersistentVolume" {
		var pv v1.PersistentVolume
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &pv)
		if err != nil {
			k8sLogger.Error("Error cannot cast from unstructured", "error", err)
			return
		}
		handlePVDeletion(&pv)
	}

	// other resources - delete primary key + both ZSET index members atomically.
	err := store.DeleteResourceWithIndex(valkeyClient, apiVersion, kind, namespace, resourceName, obj)
	if err != nil {
		k8sLogger.Error("Error deleting object in store", "error", err)
	}
}

func GetUnstructuredResourceListFromStore(apiVersion string, kind string, namespace *string, withData *bool) unstructured.UnstructuredList {
	selectedNamespace := ""
	if namespace != nil {
		selectedNamespace = *namespace
	}

	// try to get the data from the store (very fast)
	results := unstructured.UnstructuredList{}
	result := store.GetResourceByKindAndNamespace(valkeyClient, apiVersion, kind, selectedNamespace, k8sLogger)
	if result != nil {
		// delete data field to speed up transfer
		if withData == nil || !*withData {
			for i := range result {
				delete(result[i].Object, "data")
			}
		}
		results.Items = result
	}

	return results
}

func GetUnstructuredNamespaceResourceList(namespace string, whitelist []*utils.ResourceDescriptor, blacklist []*utils.ResourceDescriptor) ([]unstructured.Unstructured, error) {
	results := []unstructured.Unstructured{}

	resources, err := GetAvailableResources()
	if err != nil {
		return results, err
	}

	if whitelist == nil {
		whitelist = []*utils.ResourceDescriptor{}
	}

	if blacklist == nil {
		blacklist = []*utils.ResourceDescriptor{}
	}

	// Collect the allowed namespaced kinds, then fetch everything with ONE
	// namespace-scoped keyspace scan. The previous per-kind fan-out ran a
	// full SCAN per watched kind (80-150 kinds) for every call.
	allowed := make([]utils.ResourceDescriptor, 0, len(resources))
	includeNamespaceObject := false
	for _, v := range resources {
		if v.Namespaced {
			if len(whitelist) > 0 && !utils.ContainsResourceDescriptor(whitelist, v) {
				continue
			}
			if utils.ContainsResourceDescriptor(blacklist, v) {
				continue
			}
			allowed = append(allowed, v)
			continue
		}

		// Cluster-scoped resources are not bound to a namespace. The one that
		// is meaningful in a namespace-scoped query is the Namespace object
		// itself, which IS the namespace - return it looked up by name so the
		// workspace "Namespace" filter is not empty. (MOG-4362)
		//
		// This is gated on the kind being EXPLICITLY whitelisted (non-empty
		// whitelist containing it): existing callers that pass a nil/empty
		// whitelist for a specific namespace (workload status, argocd,
		// workspace controllers) must keep getting only namespaced resources,
		// so this stays strictly additive for the Namespace-filter case.
		if namespace == "" || len(whitelist) == 0 {
			continue
		}
		if !utils.ContainsResourceDescriptor(whitelist, v) {
			continue
		}
		if utils.ContainsResourceDescriptor(blacklist, v) {
			continue
		}
		if v.Kind == utils.NamespaceResource.Kind && v.ApiVersion == utils.NamespaceResource.ApiVersion {
			includeNamespaceObject = true
		}
	}

	results, err = store.GetResourcesByNamespaceAndKinds(valkeyClient, namespace, allowed)
	if err != nil {
		k8sLogger.Error("failed to fetch namespace resources", "namespace", namespace, "error", err)
		return []unstructured.Unstructured{}, err
	}

	if includeNamespaceObject {
		nsObj, err := store.GetResource(valkeyClient, utils.NamespaceResource.ApiVersion, utils.NamespaceResource.Kind, "", namespace, k8sLogger)
		if err == nil && nsObj != nil {
			results = append(results, *nsObj)
		}
	}

	return results, nil
}

func GetUnstructuredResource(apiVersion string, plural string, namespace, resourceName string) (*unstructured.Unstructured, error) {
	dynamicClient := clientProvider.DynamicClient()
	if namespace != "" {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Namespace(namespace).Get(context.Background(), resourceName, metav1.GetOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Get(context.Background(), resourceName, metav1.GetOptions{})
		return removeManagedFields(result), err
	}
}

func GetUnstructuredResourceFromStore(apiVersion string, kind string, namespace, resourceName string) (*unstructured.Unstructured, error) {
	return store.GetResource(valkeyClient, apiVersion, kind, namespace, resourceName, k8sLogger)
}

func CreateUnstructuredResource(apiVersion string, plural string, namespaced bool, yamlData string) (*unstructured.Unstructured, error) {
	dynamicClient := clientProvider.DynamicClient()
	obj := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(yamlData), obj)
	if err != nil {
		return nil, err
	}

	if namespaced {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Namespace(obj.GetNamespace()).Create(context.Background(), obj, metav1.CreateOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Create(context.Background(), obj, metav1.CreateOptions{})
		return removeManagedFields(result), err
	}
}

func UpdateUnstructuredResource(apiVersion string, plural string, namespaced bool, yamlData string) (*unstructured.Unstructured, error) {
	dynamicClient := clientProvider.DynamicClient()
	obj := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(yamlData), obj)
	if err != nil {
		return nil, err
	}

	if namespaced {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Namespace(obj.GetNamespace()).Update(context.Background(), obj, metav1.UpdateOptions{})
		return removeManagedFields(result), err
	} else {
		result, err := dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Update(context.Background(), obj, metav1.UpdateOptions{})
		return removeManagedFields(result), err
	}
}

// backgroundDeleteOptions returns DeleteOptions with background propagation.
// Without an explicit policy, batch/v1 Jobs default to Orphan on the server,
// which leaves the pods behind and puts a transient "orphan" finalizer on the
// Job while the GC decouples them.
func backgroundDeleteOptions() metav1.DeleteOptions {
	propagation := metav1.DeletePropagationBackground
	return metav1.DeleteOptions{PropagationPolicy: &propagation}
}

// BlockingFinalizers returns the finalizers that can actually block a deletion,
// dropping the transient garbage-collector finalizers ("orphan",
// "foregroundDeletion") that the API server removes on its own.
func BlockingFinalizers(finalizers []string) []string {
	blocking := []string{}
	for _, finalizer := range finalizers {
		if finalizer == metav1.FinalizerOrphanDependents || finalizer == metav1.FinalizerDeleteDependents {
			continue
		}
		blocking = append(blocking, finalizer)
	}
	return blocking
}

func DeleteUnstructuredResource(apiVersion string, plural string, namespace string, resourceName string) error {
	dynamicClient := clientProvider.DynamicClient()
	if namespace != "" {
		return dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Namespace(namespace).Delete(context.Background(), resourceName, backgroundDeleteOptions())
	} else {
		return dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Delete(context.Background(), resourceName, backgroundDeleteOptions())
	}
}

func DescribeUnstructuredResource(apiVersion string, plural string, namespace, resourceName string) (string, error) {
	config := clientProvider.ClientConfig()

	restMapping := &meta.RESTMapping{
		Resource: CreateGroupVersionResource(apiVersion, plural),
	}

	describer, ok := describe.GenericDescriberFor(restMapping, config)
	if !ok {
		return "", fmt.Errorf("failed to get describer")

	}

	output, err := describer.Describe(namespace, resourceName, describe.DescriberSettings{ShowEvents: true})
	if err != nil {
		fmt.Printf("Failed to describe resource: %v\n", err)
		return "", err
	}

	return output, nil
}

func TriggerUnstructuredResource(apiVersion string, plural string, namespace string, resourceName string) (*unstructured.Unstructured, error) {
	dynamicClient := clientProvider.DynamicClient()

	if plural == "cronjobs" || plural == "jobs" {
		job, err := GetUnstructuredResource(apiVersion, plural, namespace, resourceName)
		if err != nil {
			return nil, err
		}

		if plural == "cronjobs" {
			// get owner references
			ownerRefs, _, _ := unstructured.NestedSlice(job.Object, "metadata", "ownerReferences")
			if len(ownerRefs) == 0 {
				// if no owner references exists, create one
				ownerRef := map[string]any{
					"apiVersion":         job.GetAPIVersion(),
					"kind":               job.GetKind(),
					"name":               job.GetName(),
					"uid":                string(job.GetUID()),
					"controller":         true,
					"blockOwnerDeletion": true,
				}
				ownerRefs = append(ownerRefs, ownerRef)
				_ = unstructured.SetNestedSlice(job.Object, ownerRefs, "metadata", "ownerReferences")
			}
		}

		// cleanup
		unstructured.RemoveNestedField(job.Object, "metadata", "uid")
		unstructured.RemoveNestedField(job.Object, "metadata", "resourceVersion")
		unstructured.RemoveNestedField(job.Object, "metadata", "creationTimestamp")
		unstructured.RemoveNestedField(job.Object, "metadata", "labels", "controller-uid")
		unstructured.RemoveNestedField(job.Object, "metadata", "labels", "batch.kubernetes.io/controller-uid")
		unstructured.RemoveNestedField(job.Object, "metadata", "labels", "batch.kubernetes.io/job-name")
		unstructured.RemoveNestedField(job.Object, "spec", "selector")
		unstructured.RemoveNestedField(job.Object, "spec", "template", "metadata", "labels", "controller-uid")
		unstructured.RemoveNestedField(job.Object, "spec", "template", "metadata", "labels", "job-name")
		unstructured.RemoveNestedField(job.Object, "spec", "template", "metadata", "labels", "batch.kubernetes.io/controller-uid")
		unstructured.RemoveNestedField(job.Object, "spec", "template", "metadata", "labels", "batch.kubernetes.io/job-name")
		unstructured.RemoveNestedField(job.Object, "status")

		// replace
		jobname := job.GetName() + "-" + utils.NanoIdSmallLowerCase()
		job.SetName(jobname)
		job.SetKind("Job")
		if plural == "cronjobs" {
			template, _, err := unstructured.NestedMap(job.Object, "spec", "jobTemplate", "spec", "template")
			if err != nil {
				return nil, fmt.Errorf("field jobTemplate not found")
			}
			_ = unstructured.SetNestedField(job.Object, template, "spec", "template")
			plural = "jobs"
		}

		return dynamicClient.Resource(CreateGroupVersionResource(apiVersion, plural)).Namespace(namespace).Create(context.Background(), job, metav1.CreateOptions{})
	}
	return nil, fmt.Errorf("%s is a invalid resource for trigger. Only jobs or cronjobs can be triggert", plural)
}

type availableResourceCacheEntry struct {
	timestamp          time.Time
	availableResources []utils.ResourceDescriptor
}

var (
	resourceCache      availableResourceCacheEntry
	resourceCacheMutex sync.RWMutex      // RWMutex allows concurrent cache reads
	resourceCacheTTL   = 1 * time.Minute // Cache duration
)

func GetAvailableResources() ([]utils.ResourceDescriptor, error) {
	// Fast path: concurrent reads while cache is valid
	resourceCacheMutex.RLock()
	if time.Since(resourceCache.timestamp) < resourceCacheTTL {
		result := resourceCache.availableResources
		resourceCacheMutex.RUnlock()
		return result, nil
	}
	resourceCacheMutex.RUnlock()

	// Slow path: acquire write lock for cache refresh
	resourceCacheMutex.Lock()
	defer resourceCacheMutex.Unlock()

	// Double-check: another goroutine may have refreshed the cache while we waited
	if time.Since(resourceCache.timestamp) < resourceCacheTTL {
		return resourceCache.availableResources, nil
	}

	// Fetch resources from server
	clientset := clientProvider.K8sClientSet()
	resources, err := clientset.Discovery().ServerPreferredResources()
	if err != nil {
		if discovery.IsGroupDiscoveryFailedError(err) {
			k8sLogger.Error("Failed to discover group resources", "error", err)
		} else {
			k8sLogger.Error("Error discovering resources", "error", err)
			return nil, err
		}
	}

	var availableResources []utils.ResourceDescriptor
	for _, resourceList := range resources {
		for _, resource := range resourceList.APIResources {
			if !slices.Contains(resource.Verbs, "list") || !slices.Contains(resource.Verbs, "watch") {
				continue
			}
			if isDeprecatedResource(resourceList.GroupVersion, resource.Kind) {
				continue
			}
			availableResources = append(availableResources, utils.ResourceDescriptor{
				Plural:     resource.Name,
				ApiVersion: resourceList.GroupVersion,
				Kind:       resource.Kind,
				Namespaced: resource.Namespaced,
			})
		}
	}

	resourceCache.availableResources = availableResources
	resourceCache.timestamp = time.Now()

	return availableResources, nil
}

func resetAvailableResourceCache() {
	resourceCacheMutex.Lock()
	defer resourceCacheMutex.Unlock()
	resourceCache = availableResourceCacheEntry{}
}

func GetResourcesNameForKind(kind string) (name string, err error) {
	resources, err := GetAvailableResources()
	if err != nil {
		return "", err
	}

	for _, resource := range resources {
		if resource.Kind == kind {
			return resource.Plural, nil
		}
	}
	return "", fmt.Errorf("resource not found for name %s", name)
}

func CreateGroupVersionResource(apiVersion, plural string) schema.GroupVersionResource {
	gv, err := schema.ParseGroupVersion(apiVersion) // e.g., "apps/v1" or just "v1"
	if err != nil {
		k8sLogger.Error("invalid apiVersion", "apiVersion", apiVersion, "resourceName", plural, "error", err)
	}
	gvr := gv.WithResource(plural)
	return gvr
}

func removeManagedFields(obj *unstructured.Unstructured) *unstructured.Unstructured {
	if obj == nil {
		return obj
	}
	unstructuredContent := obj.Object
	delete(unstructuredContent, "managedFields")
	if meta, ok := unstructuredContent["metadata"].(map[string]any); ok {
		delete(meta, "managedFields")
	}
	return obj
}


func removeUnusedFieds(obj *unstructured.Unstructured) *unstructured.Unstructured {
	obj = removeManagedFields(obj)
	return obj
}
