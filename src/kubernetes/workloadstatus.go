package kubernetes

import (
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"mogenius-operator/src/helm"
	"mogenius-operator/src/store"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type WorkloadStatusItemDto struct {
	UID        string `json:"uid" validate:"required"`
	Kind       string `json:"kind" validate:"required"`
	ApiVersion string `json:"apiVersion" validate:"required"`

	Name              string      `json:"name" validate:"required"`
	Namespace         string      `json:"namespace" validate:"required"`
	CreationTimestamp metav1.Time `json:"creationTimestamp"`
	OwnerReferences   any         `json:"ownerReferences,omitempty"`

	Status        any        `json:"status,omitempty"`
	Events        []v1.Event `json:"events,omitempty"`
	Replicas      *int       `json:"replicas,omitempty"`
	SpecClusterIP string     `json:"specClusterIP,omitempty"`
	SpecType      string     `json:"specType,omitempty"`

	// only for EndpointSlice
	Endpoints any `json:"endpoints,omitempty"`
}

type WorkloadStatusDto struct {
	Items []WorkloadStatusItemDto `json:"items" validate:"required"`
}

type GetWorkloadStatusHelmReleaseNameRequest struct {
	Release   string `json:"release" validate:"required"`
	Namespace string `json:"namespace" validate:"required"`
}
type GetWorkloadStatusRequest struct {
	ResourceDescriptor *utils.ResourceDescriptor                  `json:"resourceDescriptor,omitempty"`
	Namespaces         *[]string                                  `json:"namespaces,omitempty"`
	HelmReleases       *[]GetWorkloadStatusHelmReleaseNameRequest `json:"helmReleases,omitempty"`
	ResourceNames      *[]string                                  `json:"resourceNames,omitempty"`

	IgnoreDependentResources *bool `json:"ignoreDependentResources,omitempty"`
}

func getOrFetchReplicaSets(valkeyClient valkeyclient.ValkeyClient, cache map[string][]unstructured.Unstructured, namespace string) []unstructured.Unstructured {
	if cachedSets, found := cache[namespace]; found {
		return cachedSets
	}
	replicaSetsResults, err := store.SearchResourceByKeyParts(valkeyClient, utils.ReplicaSetResource.ApiVersion, utils.ReplicaSetResource.Kind, namespace, "*")
	if err != nil {
		k8sLogger.Warn("Error getting replicasets", "namespace", namespace, "error", err)
		return nil
	}
	cache[namespace] = replicaSetsResults
	return replicaSetsResults
}

func getOrFetchJobs(cache map[string][]unstructured.Unstructured, namespace string) []unstructured.Unstructured {
	if cachedSets, found := cache[namespace]; found {
		return cachedSets
	}
	jobResults, err := store.SearchResourceByKeyParts(valkeyClient, utils.JobResource.ApiVersion, utils.JobResource.Kind, namespace, "*")
	if err != nil {
		k8sLogger.Warn("Error getting jobs", "namespace", namespace, "error", err)
		return nil
	}
	cache[namespace] = jobResults
	return jobResults
}

func getOrFetchPods(cache map[string][]unstructured.Unstructured, namespace string) []unstructured.Unstructured {
	if cachedPods, found := cache[namespace]; found {
		return cachedPods
	}
	podsResults, err := store.SearchResourceByKeyParts(valkeyClient, utils.PodResource.ApiVersion, utils.PodResource.Kind, namespace, "*")
	if err != nil {
		k8sLogger.Warn("Error getting pods", "namespace", namespace, "error", err)
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
		ApiVersion:        workload.GetAPIVersion(),
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
	var workloadList []unstructured.Unstructured = []unstructured.Unstructured{}

	var wg sync.WaitGroup
	workloadListChan := make(chan []unstructured.Unstructured)

	// Check if ResourceDescriptor is empty (considered empty if all fields are empty strings or nil)
	isResourceDescriptorEmpty := requestData.ResourceDescriptor == nil || (requestData.ResourceDescriptor.Kind == "" && requestData.ResourceDescriptor.ApiVersion == "")

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
		var whitelist []*utils.ResourceDescriptor
		if requestData.ResourceDescriptor != nil {
			whitelist = append(whitelist, requestData.ResourceDescriptor)
		}

		for _, helmRelease := range *requestData.HelmReleases {
			wg.Go(func() {
				unstructuredResourceList, err := helm.HelmReleaseGetWorkloads(valkeyClient, helm.HelmReleaseGetWorkloadsRequest{
					Release:   helmRelease.Release,
					Namespace: helmRelease.Namespace,
					Whitelist: whitelist,
				})
				if err != nil {
					k8sLogger.Warn("Error getting workload list", "error", err)
				} else {
					workloadListChan <- unstructuredResourceList
				}
			})
		}
	}
	// only filter by ResourceDescriptor
	if !isResourceDescriptorEmpty && requestData.Namespaces == nil && requestData.ResourceNames == nil {
		wg.Go(func() {
			unstructuredResourceList := GetUnstructuredResourceListFromStore(requestData.ResourceDescriptor.ApiVersion, requestData.ResourceDescriptor.Kind, nil, nil)
			if len(unstructuredResourceList.Items) > 0 {
				workloadListChan <- unstructuredResourceList.Items
			}
		})
	} else
	// only filter by namespaces
	if isResourceDescriptorEmpty && requestData.Namespaces != nil && requestData.ResourceNames == nil {
		for _, namespace := range *requestData.Namespaces {
			wg.Go(func() {
				unstructuredResourceList, err := GetUnstructuredNamespaceResourceList(namespace, nil, nil)
				if err != nil {
					k8sLogger.Warn("Error getting workload list", "error", err)
				} else {
					workloadListChan <- unstructuredResourceList
				}
			})
		}

	} else
	// filter by ResourceDescriptor and namespaces
	if !isResourceDescriptorEmpty && requestData.Namespaces != nil && requestData.ResourceNames == nil {
		if requestData.ResourceDescriptor.Kind == "Namespace" && requestData.ResourceDescriptor.ApiVersion == "v1" {
			wg.Go(func() {
				unstructuredResourceNamespaceList := GetUnstructuredResourceListFromStore(requestData.ResourceDescriptor.ApiVersion, requestData.ResourceDescriptor.Kind, nil, nil)
				for _, namespace := range *requestData.Namespaces {
					if namespace == "" {
						continue
					}
					for _, item := range unstructuredResourceNamespaceList.Items {
						if item.GetName() == namespace {
							workloadListChan <- []unstructured.Unstructured{item}
						}
					}
				}
			})
		} else {
			for _, namespace := range *requestData.Namespaces {
				wg.Go(func() {
					unstructuredResourceList := GetUnstructuredResourceListFromStore(requestData.ResourceDescriptor.ApiVersion, requestData.ResourceDescriptor.Kind, &namespace, nil)
					if len(unstructuredResourceList.Items) > 0 {
						workloadListChan <- unstructuredResourceList.Items
					}
				})
			}
		}

	} else
	// filter by ResourceDescriptor, namespaces and resourceNames
	if !isResourceDescriptorEmpty && requestData.Namespaces != nil && requestData.ResourceNames != nil {
		for _, resourceName := range *requestData.ResourceNames {
			for _, namespace := range *requestData.Namespaces {
				wg.Go(func() {
					workloads, err := store.SearchByGroupKindNameNamespace(valkeyClient, requestData.ResourceDescriptor.ApiVersion, requestData.ResourceDescriptor.Kind, resourceName, &namespace)
					if err != nil {
						k8sLogger.Warn("Error getting workload", "error", err)
					} else {
						workloadListChan <- workloads
					}
				})
			}
		}
	} else
	// filter by ResourceDescriptor and resourceNames
	if !isResourceDescriptorEmpty && requestData.Namespaces == nil && requestData.ResourceNames != nil {
		for _, resourceName := range *requestData.ResourceNames {
			wg.Go(func() {
				workloads, err := store.SearchByGroupKindNameNamespace(valkeyClient, requestData.ResourceDescriptor.ApiVersion, requestData.ResourceDescriptor.Kind, resourceName, nil)
				if err != nil {
					k8sLogger.Warn("Error getting workload", "error", err)
				} else {
					workloadListChan <- workloads
				}
			})
		}
	} else
	// filter by namespaces and resourceNames
	if isResourceDescriptorEmpty && requestData.Namespaces != nil && requestData.ResourceNames != nil {
		for _, resourceName := range *requestData.ResourceNames {
			for _, namespace := range *requestData.Namespaces {
				wg.Go(func() {
					workloads, err := store.SearchByNamespaceAndName(valkeyClient, namespace, resourceName)
					if err != nil {
						k8sLogger.Warn("Error getting workload", "error", err)
					} else {
						workloadListChan <- workloads
					}
				})
			}
		}
	} else {
		k8sLogger.Debug("No filter applied")
		return []WorkloadStatusDto{}, nil
	}

	// Wait for all goroutines to finish and close the channel
	go func() {
		wg.Wait()
		close(workloadListChan)
	}()
	for res := range workloadListChan {
		workloadList = append(workloadList, res...)
	}

	if len(workloadList) == 0 {
		return []WorkloadStatusDto{}, nil
	}

	// get all events from the store
	eventUnstructuredList := GetUnstructuredResourceListFromStore(requestData.ResourceDescriptor.ApiVersion, requestData.ResourceDescriptor.Kind, utils.Pointer(""), nil)
	var eventList []v1.Event
	for _, item := range eventUnstructuredList.Items {
		var event v1.Event
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &event)
		if err != nil {
			continue
		}
		eventList = append(eventList, event)
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
