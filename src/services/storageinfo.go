package services

import (
	"context"
	"fmt"
	mokubernetes "mogenius-operator/src/kubernetes"
	"mogenius-operator/src/store"
	"sort"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PV annotation carrying the provisioner that created the volume.
const pvProvisionedByAnnotation = "pv.kubernetes.io/provisioned-by"

// ── wire types (storage/v2/info, storage/v2/stats) ────────────────────────────

type StorageV2InfoRequestItem struct {
	Namespace string `json:"namespace" validate:"required"`
	PvcName   string `json:"pvcName" validate:"required"`
}

type StorageV2InfoRequest struct {
	Items      []StorageV2InfoRequestItem `json:"items" validate:"required"`
	WithEvents bool                       `json:"withEvents"`
}

type StorageV2MountedBy struct {
	PodName        string `json:"podName"`
	ControllerKind string `json:"controllerKind"`
	ControllerName string `json:"controllerName"`
	ContainerName  string `json:"containerName"`
	MountPath      string `json:"mountPath"`
	SubPath        bool   `json:"subPath"`
	Ready          bool   `json:"ready"`
}

type StorageV2Event struct {
	Type          string `json:"type"`
	Reason        string `json:"reason"`
	Message       string `json:"message"`
	LastTimestamp string `json:"lastTimestamp"`
}

type StorageV2InfoItem struct {
	Namespace        string               `json:"namespace"`
	PvcName          string               `json:"pvcName"`
	Phase            string               `json:"phase"`
	RequestedBytes   int64                `json:"requestedBytes"`
	CapacityBytes    int64                `json:"capacityBytes"`
	StorageClassName string               `json:"storageClassName"`
	AccessModes      []string             `json:"accessModes"`
	VolumeName       string               `json:"volumeName"`
	VolumeMode       string               `json:"volumeMode"`
	Provisioner      string               `json:"provisioner"`
	MountedBy        []StorageV2MountedBy `json:"mountedBy"`
	Browsable        bool                 `json:"browsable"`
	BrowsableReason  string               `json:"browsableReason"`
	Events           []StorageV2Event     `json:"events,omitempty"`
}

type StorageV2InfoResponse struct {
	Items []StorageV2InfoItem `json:"items"`
}

type StorageV2StatsResponse struct {
	TotalBytes      uint64 `json:"totalBytes"`
	UsedBytes       uint64 `json:"usedBytes"`
	FreeBytes       uint64 `json:"freeBytes"`
	SourcePod       string `json:"sourcePod"`
	SourceContainer string `json:"sourceContainer"`
}

// ── storage/v2/stats ──────────────────────────────────────────────────────────

// StorageV2Stats resolves the exec target for the PVC and reads filesystem
// usage via `df -B1 <mountPath>` in that container.
func StorageV2Stats(namespace, pvcName string) (StorageV2StatsResponse, error) {
	target, err := ResolvePvcTarget(namespace, pvcName)
	if err != nil {
		return StorageV2StatsResponse{}, err
	}

	free, used, total, err := mokubernetes.PodDiskUsage(target.Namespace, target.PodName, target.ContainerName, target.MountPath)
	if err != nil {
		return StorageV2StatsResponse{}, err
	}

	return StorageV2StatsResponse{
		TotalBytes:      total,
		UsedBytes:       used,
		FreeBytes:       free,
		SourcePod:       target.PodName,
		SourceContainer: target.ContainerName,
	}, nil
}

// ── storage/v2/info ───────────────────────────────────────────────────────────

// pvcPodMounts maps claimName → pods (by index into the namespace pod slice)
// that reference the claim, built with ONE scan over the namespace's pods.
type namespacePodIndex struct {
	pods         []v1.Pod
	podsPerClaim map[string][]int
}

func buildNamespacePodIndex(namespace string) namespacePodIndex {
	pods := store.GetPods(namespace)
	index := namespacePodIndex{pods: pods, podsPerClaim: map[string][]int{}}
	for i := range pods {
		seen := map[string]bool{}
		for _, volume := range pods[i].Spec.Volumes {
			if volume.PersistentVolumeClaim == nil {
				continue
			}
			claim := volume.PersistentVolumeClaim.ClaimName
			if !seen[claim] {
				seen[claim] = true
				index.podsPerClaim[claim] = append(index.podsPerClaim[claim], i)
			}
		}
	}
	return index
}

// StorageV2Info builds the batch PVC info of the storage/v2/info contract.
// It scans the pods of every distinct namespace in the batch exactly once.
func StorageV2Info(request StorageV2InfoRequest) (StorageV2InfoResponse, error) {
	response := StorageV2InfoResponse{Items: []StorageV2InfoItem{}}

	// one pod scan per distinct namespace
	podIndexes := map[string]namespacePodIndex{}
	for _, item := range request.Items {
		if _, ok := podIndexes[item.Namespace]; !ok {
			podIndexes[item.Namespace] = buildNamespacePodIndex(item.Namespace)
		}
	}

	for _, item := range request.Items {
		infoItem := StorageV2InfoItem{
			Namespace:   item.Namespace,
			PvcName:     item.PvcName,
			AccessModes: []string{},
			MountedBy:   []StorageV2MountedBy{},
		}

		pvc := getPvc(item.Namespace, item.PvcName)
		var pv *v1.PersistentVolume
		if pvc != nil {
			fillPvcFields(&infoItem, pvc)
			if pvc.Spec.VolumeName != "" {
				pv = getPv(pvc.Spec.VolumeName)
				if pv != nil {
					infoItem.Provisioner = pv.Annotations[pvProvisionedByAnnotation]
				}
			}
		}

		index := podIndexes[item.Namespace]
		infoItem.MountedBy, infoItem.Browsable, infoItem.BrowsableReason = computeMounts(index, item.PvcName)

		if request.WithEvents {
			pvName := ""
			if pv != nil {
				pvName = pv.Name
			}
			infoItem.Events = collectPvcEvents(item.Namespace, item.PvcName, pvName)
		}

		response.Items = append(response.Items, infoItem)
	}

	return response, nil
}

// fillPvcFields copies the PVC-derived fields of the wire contract.
func fillPvcFields(item *StorageV2InfoItem, pvc *v1.PersistentVolumeClaim) {
	item.Phase = string(pvc.Status.Phase)
	item.VolumeName = pvc.Spec.VolumeName

	if requested, ok := pvc.Spec.Resources.Requests[v1.ResourceStorage]; ok {
		item.RequestedBytes = requested.Value()
	}
	if capacity, ok := pvc.Status.Capacity[v1.ResourceStorage]; ok {
		item.CapacityBytes = capacity.Value()
	}

	if pvc.Spec.StorageClassName != nil {
		item.StorageClassName = *pvc.Spec.StorageClassName
	}
	if pvc.Spec.VolumeMode != nil {
		item.VolumeMode = string(*pvc.Spec.VolumeMode)
	}

	// actual (status) modes once bound, requested (spec) modes before
	accessModes := pvc.Status.AccessModes
	if len(accessModes) == 0 {
		accessModes = pvc.Spec.AccessModes
	}
	for _, mode := range accessModes {
		item.AccessModes = append(item.AccessModes, string(mode))
	}
}

// computeMounts builds the mountedBy entries and the structural browsable
// verdict for one PVC from the pre-built namespace pod index.
//
// Browsable is computed structurally only: a running pod with a ready
// container mounting the PVC without subPath → browsable=true (tentatively).
// NOT_MOUNTED / SUBPATH_ONLY / POD_NOT_READY are structural reasons.
// NO_EXEC_TOOLING is deliberately NOT probed here — exec-probing every PVC in
// a batch listing would be far too expensive; it surfaces only when an actual
// file operation or stats call fails the probe (via ErrPvcNoExecTooling).
func computeMounts(index namespacePodIndex, pvcName string) ([]StorageV2MountedBy, bool, string) {
	mountedBy := []StorageV2MountedBy{}
	browsable := false
	anyNonSubPath := false

	for _, podIdx := range index.podsPerClaim[pvcName] {
		pod := &index.pods[podIdx]

		volumeNames := map[string]bool{}
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvcName {
				volumeNames[volume.Name] = true
			}
		}

		readyByContainer := map[string]bool{}
		for _, status := range pod.Status.ContainerStatuses {
			readyByContainer[status.Name] = status.Ready
		}

		controllerKind, controllerName := controllerForPod(pod)

		for _, container := range pod.Spec.Containers {
			for _, mount := range container.VolumeMounts {
				if !volumeNames[mount.Name] {
					continue
				}
				subPath := mount.SubPath != "" || mount.SubPathExpr != ""
				ready := readyByContainer[container.Name]
				mountedBy = append(mountedBy, StorageV2MountedBy{
					PodName:        pod.Name,
					ControllerKind: controllerKind,
					ControllerName: controllerName,
					ContainerName:  container.Name,
					MountPath:      mount.MountPath,
					SubPath:        subPath,
					Ready:          ready,
				})
				if !subPath {
					anyNonSubPath = true
					if pod.Status.Phase == v1.PodRunning && ready {
						browsable = true
					}
				}
			}
		}
	}

	reason := ""
	if !browsable {
		switch {
		case len(mountedBy) == 0:
			reason = BrowsableReasonNotMounted
		case !anyNonSubPath:
			reason = BrowsableReasonSubPathOnly
		default:
			reason = BrowsableReasonPodNotReady
		}
	}
	return mountedBy, browsable, reason
}

// controllerForPod walks the pod's ownerReferences to the workload the user
// knows: a ReplicaSet owner is resolved to its own owner (the Deployment)
// when present; any other owner (StatefulSet, DaemonSet, Job, …) is used
// directly. A bare pod attributes to itself.
func controllerForPod(pod *v1.Pod) (kind string, name string) {
	owner := controllerOwnerRef(pod.OwnerReferences)
	if owner == nil {
		return "Pod", pod.Name
	}

	if owner.Kind == "ReplicaSet" {
		if replicaSet := store.GetReplicaset(pod.Namespace, owner.Name); replicaSet != nil {
			if rsOwner := controllerOwnerRef(replicaSet.OwnerReferences); rsOwner != nil {
				return rsOwner.Kind, rsOwner.Name
			}
		}
	}

	return owner.Kind, owner.Name
}

// controllerOwnerRef returns the controlling ownerReference, falling back to
// the first one when none is flagged as controller.
func controllerOwnerRef(refs []metav1.OwnerReference) *metav1.OwnerReference {
	for i := range refs {
		if refs[i].Controller != nil && *refs[i].Controller {
			return &refs[i]
		}
	}
	if len(refs) > 0 {
		return &refs[0]
	}
	return nil
}

// getPvc reads the PVC from the store, falling back to the live API when the
// store has no copy (e.g. right after a store wipe).
func getPvc(namespace, name string) *v1.PersistentVolumeClaim {
	if pvc := store.GetPersistentVolumeClaim(namespace, name); pvc != nil {
		return pvc
	}
	pvc, err := clientProvider.K8sClientSet().CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		serviceLogger.Warn("failed to get PVC", "namespace", namespace, "name", name, "error", err)
		return nil
	}
	return pvc
}

// getPv reads the PV from the store, falling back to the live API.
func getPv(name string) *v1.PersistentVolume {
	if pv := store.GetPersistentVolume(name); pv != nil {
		return pv
	}
	pv, err := clientProvider.K8sClientSet().CoreV1().PersistentVolumes().Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		serviceLogger.Warn("failed to get PV", "name", name, "error", err)
		return nil
	}
	return pv
}

// collectPvcEvents lists the events for the PVC and (when bound) its PV,
// newest first. Queries are namespaced to the PVC's namespace, matching the
// legacy storagestatus behavior.
func collectPvcEvents(namespace, pvcName, pvName string) []StorageV2Event {
	events := []StorageV2Event{}
	events = append(events, listEventsFor(namespace, pvcName, "PersistentVolumeClaim")...)
	if pvName != "" {
		events = append(events, listEventsFor(namespace, pvName, "PersistentVolume")...)
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].LastTimestamp > events[j].LastTimestamp
	})
	return events
}

func listEventsFor(namespace, name, kind string) []StorageV2Event {
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=%s", name, kind)
	eventList, err := clientProvider.K8sClientSet().CoreV1().Events(namespace).List(context.Background(), metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		serviceLogger.Warn("failed to list events", "namespace", namespace, "name", name, "kind", kind, "error", err)
		return nil
	}

	result := make([]StorageV2Event, 0, len(eventList.Items))
	for _, event := range eventList.Items {
		result = append(result, StorageV2Event{
			Type:          event.Type,
			Reason:        event.Reason,
			Message:       event.Message,
			LastTimestamp: event.LastTimestamp.Format(time.RFC3339),
		})
	}
	return result
}
