package kubernetes

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"mogenius-k8s-manager/src/helm"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeyclient"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type WorkloadStatusItemDto struct {
	UID               string      `json:"uid" validate:"required"`
	Kind              string      `json:"kind" validate:"required"`
	Group             string      `json:"group" validate:"required"`
	Name              string      `json:"name" validate:"required"`
	Namespace         string      `json:"namespace" validate:"required"`
	CreationTimestamp metav1.Time `json:"creationTimestamp"`
	OwnerReferences   interface{} `json:"ownerReferences,omitempty"`

	Status        interface{} `json:"status,omitempty"`
	Events        []v1.Event  `json:"events,omitempty"`
	Replicas      *int        `json:"replicas,omitempty"`
	SpecClusterIP string      `json:"specClusterIP,omitempty"`
	SpecType      string      `json:"specType,omitempty"`

	// only for EndpointSlice
	Endpoints interface{} `json:"endpoints,omitempty"`
}

type WorkloadStatusDto struct {
	Items []WorkloadStatusItemDto `json:"items" validate:"required"`
}

type GetWorkloadStatusHelmReleaseNameRequest struct {
	Release   string `json:"release" validate:"required"`
	Namespace string `json:"namespace" validate:"required"`
}
type GetWorkloadStatusRequest struct {
	ResourceEntity *utils.SyncResourceEntry                   `json:"resourceEntity,omitempty"`
	Namespaces     *[]string                                  `json:"namespaces,omitempty"`
	HelmReleases   *[]GetWorkloadStatusHelmReleaseNameRequest `json:"helmReleases,omitempty"`
	ResourceNames  *[]string                                  `json:"resourceNames,omitempty"`

	IgnoreDependentResources *bool `json:"ignoreDependentResources,omitempty"`
}

func getOrFetchReplicaSets(valkeyClient valkeyclient.ValkeyClient, cache map[string][]unstructured.Unstructured, namespace string) []unstructured.Unstructured {
	if cachedSets, found := cache[namespace]; found {
		return cachedSets
	}
	replicaSetsResults, err := store.SearchByKeyParts(valkeyClient, "apps/v1", "ReplicaSet", namespace)
	if err != nil {
		k8sLogger.Warn("Error getting replicaset", "error", err)
		return nil
	}
	cache[namespace] = replicaSetsResults
	return replicaSetsResults
}

func getOrFetchJobs(cache map[string][]unstructured.Unstructured, namespace string) []unstructured.Unstructured {
	if cachedSets, found := cache[namespace]; found {
		return cachedSets
	}
	jobResults, err := store.SearchByKeyParts(valkeyClient, "batch/v1", "Job", namespace)
	if err != nil {
		k8sLogger.Warn("Error getting job", "error", err)
		return nil
	}
	cache[namespace] = jobResults
	return jobResults
}

func getOrFetchPods(cache map[string][]unstructured.Unstructured, namespace string) []unstructured.Unstructured {
	if cachedPods, found := cache[namespace]; found {
		return cachedPods
	}
	podsResults, err := store.SearchByKeyParts(valkeyClient, "v1", "Pod", namespace)
	if err != nil {
		k8sLogger.Warn("Error getting pod", "error", err)
		return nil
	}
	cache[namespace] = podsResults
	return podsResults
}

func hasOwnerReference(ownerReferences []metav1.OwnerReference, workloadUID types.UID) bool {
	for _, ownerReference := range ownerReferences {
		if ownerReference.UID == workloadUID {
			return true
		}
	}
	return false
}

// GetWorkloadStatusItems generates a list of WorkloadStatusItemDto for each provided workload object.
// The function filters events associated with the workload, extracts relevant details, and recursively processes dependent resources.
func GetWorkloadStatusItems(
	workload unstructured.Unstructured,
	eventList []v1.Event,
	ignoreDependentResources bool,
	replicaSetsCache map[string][]unstructured.Unstructured,
	jobsCache map[string][]unstructured.Unstructured,
	podsCache map[string][]unstructured.Unstructured,
) []WorkloadStatusItemDto {
	// Initialize the output slice for workload status items.
	var items []WorkloadStatusItemDto

	// Filter events that are related to the current workload.
	itemEvents := []v1.Event{}
	for _, event := range eventList {
		if event.InvolvedObject.UID == workload.GetUID() {
			itemEvents = append(itemEvents, event)
		}
	}

	// Extract the number of replicas from the workload object.
	var replicas *int
	replicasInt64, found, err := unstructured.NestedInt64(workload.Object, "spec", "replicas")
	if err != nil {
		k8sLogger.Warn("Error getting replicas", "error", err)
	} else if found {
		replicas = utils.Pointer(int(replicasInt64))
	}

	// Extract other relevant fields: ClusterIP, Type, and Status.
	specClusterIP, _, err := unstructured.NestedString(workload.Object, "spec", "clusterIP")
	if err != nil {
		k8sLogger.Warn("Error getting clusterIP", "error", err)
	}
	specType, _, err := unstructured.NestedString(workload.Object, "spec", "type")
	if err != nil {
		k8sLogger.Warn("Error getting clusterIP", "error", err)
	}
	status, _, err := unstructured.NestedFieldNoCopy(workload.Object, "status")
	if err != nil {
		k8sLogger.Warn("Error getting status", "error", err)
	}
	endpoints, _, err := unstructured.NestedFieldNoCopy(workload.Object, "endpoints")
	if err != nil {
		k8sLogger.Warn("Error getting endpoints", "error", err)
	}
	ownerReferences, _, err := unstructured.NestedFieldNoCopy(workload.Object, "metadata", "ownerReferences")
	if err != nil {
		k8sLogger.Warn("Error getting metadata.ownerReferences", "error", err)
	}

	// Append a new WorkloadStatusItemDto object to the result list.
	items = append(items, WorkloadStatusItemDto{
		UID:               string(workload.GetUID()),
		Kind:              workload.GetKind(),
		Group:             workload.GetAPIVersion(),
		Name:              workload.GetName(),
		Namespace:         workload.GetNamespace(),
		CreationTimestamp: workload.GetCreationTimestamp(),
		Status:            status,
		OwnerReferences:   ownerReferences,

		Events:        itemEvents,
		Replicas:      replicas,
		SpecClusterIP: specClusterIP,
		SpecType:      specType,

		// only for EndpointSlice
		Endpoints: endpoints,
	})

	if ignoreDependentResources {
		return items
	}

	// Check the kind of workload and process dependent resources
	switch workload.GetKind() {
	case "Deployment":
		// Get or fetch ReplicaSets relevant to the namespace.
		replicaSets := getOrFetchReplicaSets(valkeyClient, replicaSetsCache, workload.GetNamespace())

		if replicaSets != nil {
			var replicaSetsList []unstructured.Unstructured
			for _, replicaset := range replicaSets {

				replicasInt64, found, err := unstructured.NestedInt64(workload.Object, "spec", "replicas")
				if err != nil || !found {
					continue
				}

				if replicasInt64 > 0 && hasOwnerReference(replicaset.GetOwnerReferences(), workload.GetUID()) {
					replicaSetsList = append(replicaSetsList, replicaset)
				}
			}

			// Recursively process dependent ReplicaSets.
			for _, replicaset := range replicaSetsList {
				items = append(items, GetWorkloadStatusItems(replicaset, eventList, ignoreDependentResources, replicaSetsCache, jobsCache, podsCache)...)
			}
		}

	case "CronJob":
		// Get or fetch Jobs relevant to the namespace.
		jobs := getOrFetchJobs(jobsCache, workload.GetNamespace())

		if jobs != nil {
			var jobsList []unstructured.Unstructured
			for _, job := range jobs {
				if hasOwnerReference(job.GetOwnerReferences(), workload.GetUID()) {
					jobsList = append(jobsList, job)
				}
			}

			// Recursively process dependent Jobs.
			for _, job := range jobsList {
				items = append(items, GetWorkloadStatusItems(job, eventList, ignoreDependentResources, replicaSetsCache, jobsCache, podsCache)...)
			}
		}
	case "ReplicaSet":
		fallthrough
	case "StatefulSet":
		fallthrough
	case "DaemonSet":
		fallthrough
	case "Job":
		fallthrough
	case "ReplicationController":
		// Get or fetch Pods relevant to the namespace.
		pods := getOrFetchPods(podsCache, workload.GetNamespace())
		if pods != nil {
			var podsList []unstructured.Unstructured
			for _, pod := range pods {
				if hasOwnerReference(pod.GetOwnerReferences(), workload.GetUID()) {
					podsList = append(podsList, pod)
				}
			}

			// Recursively process dependent Pods.
			for _, pod := range podsList {
				items = append(items, GetWorkloadStatusItems(pod, eventList, ignoreDependentResources, replicaSetsCache, jobsCache, podsCache)...)
			}
		}

	}

	return items
}

// GetWorkloadStatus generates a list of WorkloadStatusDto objects by filtering and processing workloads based on the request data.
// The function accesses various caches and utilizes helper functions to retrieve and process workloads and events.
func GetWorkloadStatus(requestData GetWorkloadStatusRequest) ([]WorkloadStatusDto, error) {
	var workloadList []unstructured.Unstructured

	// Check if ResourceEntity is empty (considered empty if all fields are empty strings or nil)
	isResourceEntityEmpty := requestData.ResourceEntity == nil || (requestData.ResourceEntity.Kind == "" && requestData.ResourceEntity.Group == "" && requestData.ResourceEntity.Version == "")

	// if namespace an empty list, set it to nil
	if requestData.Namespaces != nil && len(*requestData.Namespaces) == 0 {
		requestData.Namespaces = nil
	}

	// if resourceNames an empty list, set it to nil
	if requestData.HelmReleases != nil && len(*requestData.HelmReleases) == 0 {
		requestData.HelmReleases = nil
	}

	// if resourceNames an empty list, set it to nil
	if requestData.ResourceNames != nil && len(*requestData.ResourceNames) == 0 {
		requestData.ResourceNames = nil
	}

	// filter by HelmReleaseNames
	if requestData.HelmReleases != nil && len(*requestData.HelmReleases) > 0 {
		k8sLogger.Debug("Filtering by HelmReleases")
		var whitelist []*utils.SyncResourceEntry
		if requestData.ResourceEntity != nil {
			whitelist = append(whitelist, requestData.ResourceEntity)
		}

		for _, helmRelease := range *requestData.HelmReleases {
			unstructuredResourceList, err := helm.HelmReleaseGetWorkloads(valkeyClient, helm.HelmReleaseGetWorkloadsRequest{
				Release:   helmRelease.Release,
				Namespace: helmRelease.Namespace,
				Whitelist: whitelist,
			})
			if err != nil {
				k8sLogger.Warn("Error getting workload list", "error", err)
			} else {
				workloadList = append(workloadList, unstructuredResourceList...)
			}
		}
	}
	// only filter by ResourceEntity
	if !isResourceEntityEmpty && requestData.Namespaces == nil && requestData.ResourceNames == nil {
		k8sLogger.Debug("Filtering by ResourceEntity")
		unstructuredResourceList, err := GetUnstructuredResourceListFromStore(requestData.ResourceEntity.Group, requestData.ResourceEntity.Kind, requestData.ResourceEntity.Version, requestData.ResourceEntity.Name, nil)
		if err != nil {
			k8sLogger.Warn("Error getting workload list", "error", err)
		} else {
			workloadList = append(workloadList, unstructuredResourceList.Items...)
		}
	} else
	// only filter by namespaces
	if isResourceEntityEmpty && requestData.Namespaces != nil && requestData.ResourceNames == nil {
		k8sLogger.Debug("Filtering by namespaces")
		for _, namespace := range *requestData.Namespaces {
			unstructuredResourceList, err := GetUnstructuredNamespaceResourceList(namespace, nil, nil)
			if err != nil {
				k8sLogger.Warn("Error getting workload list", "error", err)
			} else {
				workloadList = append(workloadList, *unstructuredResourceList...)
			}
		}

	} else
	// filter by ResourceEntity and namespaces
	if !isResourceEntityEmpty && requestData.Namespaces != nil && requestData.ResourceNames == nil {
		k8sLogger.Debug("Filtering by ResourceEntity and namespaces")
		if requestData.ResourceEntity.Kind == "Namespace" && requestData.ResourceEntity.Group == "v1" {
			unstructuredResourceNamespaceList, err := GetUnstructuredResourceListFromStore(requestData.ResourceEntity.Group, requestData.ResourceEntity.Kind, requestData.ResourceEntity.Version, requestData.ResourceEntity.Name, nil)
			if err != nil {
				k8sLogger.Warn("Error getting workload list", "error", err)
			} else {
				for _, namespace := range *requestData.Namespaces {
					if namespace == "" {
						continue
					}
					for _, item := range unstructuredResourceNamespaceList.Items {
						if item.GetName() == namespace {
							workloadList = append(workloadList, item)
						}
					}
				}
			}
		} else {
			for _, namespace := range *requestData.Namespaces {
				unstructuredResourceList, err := GetUnstructuredResourceListFromStore(requestData.ResourceEntity.Group, requestData.ResourceEntity.Kind, requestData.ResourceEntity.Version, requestData.ResourceEntity.Name, &namespace)
				if err != nil {
					k8sLogger.Warn("Error getting workload list", "error", err)
				} else {
					workloadList = append(workloadList, unstructuredResourceList.Items...)
				}
			}
		}

	} else
	// filter by ResourceEntity, namespaces and resourceNames
	if !isResourceEntityEmpty && requestData.Namespaces != nil && requestData.ResourceNames != nil {
		k8sLogger.Debug("Filtering by ResourceEntity, namespaces and resourceNames")
		for _, resourceName := range *requestData.ResourceNames {
			for _, namespace := range *requestData.Namespaces {
				workloads, err := store.SearchByGroupKindNameNamespace(valkeyClient, requestData.ResourceEntity.Group, requestData.ResourceEntity.Kind, resourceName, &namespace)
				if err != nil {
					k8sLogger.Warn("Error getting workload", "error", err)
				} else {
					workloadList = append(workloadList, workloads...)
				}
			}
		}
	} else
	// filter by ResourceEntity and resourceNames
	if !isResourceEntityEmpty && requestData.Namespaces == nil && requestData.ResourceNames != nil {
		k8sLogger.Debug("Filtering by ResourceEntity and resourceNames")
		for _, resourceName := range *requestData.ResourceNames {
			workloads, err := store.SearchByGroupKindNameNamespace(valkeyClient, requestData.ResourceEntity.Group, requestData.ResourceEntity.Kind, resourceName, nil)
			if err != nil {
				k8sLogger.Warn("Error getting workload", "error", err)
			} else {
				workloadList = append(workloadList, workloads...)
			}
		}
	} else
	// filter by namespaces and resourceNames
	if isResourceEntityEmpty && requestData.Namespaces != nil && requestData.ResourceNames != nil {
		k8sLogger.Debug("Filtering by namespaces and resourceNames")
		for _, resourceName := range *requestData.ResourceNames {
			for _, namespace := range *requestData.Namespaces {
				workloads, err := store.SearchByNamespaceAndName(valkeyClient, namespace, resourceName)
				if err != nil {
					k8sLogger.Warn("Error getting workload", "error", err)
				} else {
					workloadList = append(workloadList, workloads...)
				}
			}
		}
	} else {
		k8sLogger.Debug("No filter applied")
		return []WorkloadStatusDto{}, nil
	}

	if workloadList == nil {
		return []WorkloadStatusDto{}, nil
	}

	// get all events from the store
	eventResource := utils.SyncResourceEntry{
		Name:      "events",
		Kind:      "Event",
		Namespace: utils.Pointer(""),
		Group:     "v1",
		Version:   "",
	}
	eventUnstructuredList, err := GetUnstructuredResourceListFromStore(eventResource.Group, eventResource.Kind, eventResource.Version, eventResource.Name, eventResource.Namespace)
	var eventList []v1.Event
	if err != nil {
		eventList = []v1.Event{}
	} else {
		for _, item := range eventUnstructuredList.Items {
			var event v1.Event
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &event)
			if err != nil {
				continue
			}
			eventList = append(eventList, event)
		}
	}

	var results []WorkloadStatusDto

	// Initialize caches for ReplicaSets, Jobs, and Pods
	replicaSetsCache := map[string][]unstructured.Unstructured{}
	jobsCache := map[string][]unstructured.Unstructured{}
	podsCache := map[string][]unstructured.Unstructured{}

	ignoreDependentResources := false

	if requestData.IgnoreDependentResources != nil {
		ignoreDependentResources = *requestData.IgnoreDependentResources
	}

	// Generate workload status items
	completedWorkloads := map[string]bool{}
	for _, workload := range workloadList {
		if completedWorkloads[string(workload.GetUID())] {
			continue
		}
		items := GetWorkloadStatusItems(workload, eventList, ignoreDependentResources, replicaSetsCache, jobsCache, podsCache)
		completedWorkloads[string(workload.GetUID())] = true
		results = append(results, WorkloadStatusDto{Items: items})
	}

	return results, nil
}
