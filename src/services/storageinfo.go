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

type StorageInfoRequestItem struct {
	Namespace string `json:"namespace" validate:"required"`
	PvcName   string `json:"pvcName" validate:"required"`
}

type StorageInfoRequest struct {
	Items      []StorageInfoRequestItem `json:"items" validate:"required"`
	WithEvents bool                       `json:"withEvents"`
}

type StorageMountedBy struct {
	PodName        string `json:"podName"`
	ControllerKind string `json:"controllerKind"`
	ControllerName string `json:"controllerName"`
	ContainerName  string `json:"containerName"`
	MountPath      string `json:"mountPath"`
	SubPath        bool   `json:"subPath"`
	Ready          bool   `json:"ready"`
}

type StorageEvent struct {
	Type          string `json:"type"`
	Reason        string `json:"reason"`
	Message       string `json:"message"`
	LastTimestamp string `json:"lastTimestamp"`
}

// StorageHelperStatus reports the live state of the helper pod mounting a
// PVC — present on an item only while such a pod exists, so the UI can show
// why a mount hangs (e.g. FailedMount events, ContainerCreating) instead of a
// static "Mounting…" text.
type StorageHelperStatus struct {
	PodName string           `json:"podName"`
	Phase   string           `json:"phase"`
	Ready   bool             `json:"ready"`
	Reason  string           `json:"reason"`
	Message string           `json:"message"`
	Events  []StorageEvent `json:"events,omitempty"`
}

type StorageInfoItem struct {
	Namespace        string                 `json:"namespace"`
	PvcName          string                 `json:"pvcName"`
	Phase            string                 `json:"phase"`
	RequestedBytes   int64                  `json:"requestedBytes"`
	CapacityBytes    int64                  `json:"capacityBytes"`
	StorageClassName string                 `json:"storageClassName"`
	AccessModes      []string               `json:"accessModes"`
	VolumeName       string                 `json:"volumeName"`
	VolumeMode       string                 `json:"volumeMode"`
	Provisioner      string                 `json:"provisioner"`
	MountedBy        []StorageMountedBy   `json:"mountedBy"`
	Browsable        bool                   `json:"browsable"`
	BrowsableReason  string                 `json:"browsableReason"`
	HelperMounted    bool                   `json:"helperMounted"`
	HelperStatus     *StorageHelperStatus `json:"helperStatus,omitempty"`
	Events           []StorageEvent       `json:"events,omitempty"`
	// PVC creationTimestamp as RFC3339, "" when unknown
	CreatedAt string `json:"createdAt,omitempty"`
}

type StorageInfoResponse struct {
	Items []StorageInfoItem `json:"items"`
}

type StorageStatsResponse struct {
	TotalBytes      uint64 `json:"totalBytes"`
	UsedBytes       uint64 `json:"usedBytes"`
	FreeBytes       uint64 `json:"freeBytes"`
	SourcePod       string `json:"sourcePod"`
	SourceContainer string `json:"sourceContainer"`
}

// ── storage/v2/stats ──────────────────────────────────────────────────────────

// StorageStats resolves the exec target for the PVC and reads filesystem
// usage via `df -B1 <mountPath>` in that container.
func StorageStats(namespace, pvcName string) (StorageStatsResponse, error) {
	target, err := ResolvePvcTarget(namespace, pvcName)
	if err != nil {
		return StorageStatsResponse{}, err
	}

	free, used, total, err := mokubernetes.PodDiskUsage(target.Namespace, target.PodName, target.ContainerName, target.MountPath)
	if err != nil {
		return StorageStatsResponse{}, err
	}

	return StorageStatsResponse{
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

// StorageInfo builds the batch PVC info of the storage/v2/info contract.
// It scans the pods of every distinct namespace in the batch exactly once.
func StorageInfo(request StorageInfoRequest) (StorageInfoResponse, error) {
	response := StorageInfoResponse{Items: []StorageInfoItem{}}

	// one pod scan per distinct namespace
	podIndexes := map[string]namespacePodIndex{}
	for _, item := range request.Items {
		if _, ok := podIndexes[item.Namespace]; !ok {
			podIndexes[item.Namespace] = buildNamespacePodIndex(item.Namespace)
		}
	}

	for _, item := range request.Items {
		infoItem := StorageInfoItem{
			Namespace:   item.Namespace,
			PvcName:     item.PvcName,
			AccessModes: []string{},
			MountedBy:   []StorageMountedBy{},
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
		// the helper pod itself shows up in mountedBy naturally (controllerKind
		// "Pod"); this flag just tells the UI that one of them is ours, and
		// helperStatus carries its live state while it exists
		if helper := helperPodFor(index, item.PvcName); helper != nil {
			infoItem.HelperMounted = true
			infoItem.HelperStatus = buildHelperStatus(helper, item.Namespace, request.WithEvents)
		}

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
func fillPvcFields(item *StorageInfoItem, pvc *v1.PersistentVolumeClaim) {
	item.Phase = string(pvc.Status.Phase)
	item.VolumeName = pvc.Spec.VolumeName
	if !pvc.CreationTimestamp.IsZero() {
		item.CreatedAt = pvc.CreationTimestamp.UTC().Format(time.RFC3339)
	}

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
func computeMounts(index namespacePodIndex, pvcName string) ([]StorageMountedBy, bool, string) {
	mountedBy := []StorageMountedBy{}
	browsable := false
	anyNonSubPath := false

	for _, podIdx := range index.podsPerClaim[pvcName] {
		pod := &index.pods[podIdx]
		// a terminating pod (e.g. the helper right after an unmount) is no mount
		// target anymore; keeping it here would report browsable=true for a
		// volume nobody can exec into and the UI would keep the file view open
		if pod.DeletionTimestamp != nil {
			continue
		}

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
				mountedBy = append(mountedBy, StorageMountedBy{
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

// storageHelperEventLimit caps the events embedded in helperStatus — the UI
// needs the recent mount failures (e.g. FailedMount), not the full history.
const storageHelperEventLimit = 10

// buildHelperStatus maps the helper pod's live state onto the wire shape.
// Events are fetched only for withEvents requests (the single-item detail
// call the UI polls), newest first, capped at storageHelperEventLimit.
func buildHelperStatus(pod *v1.Pod, namespace string, withEvents bool) *StorageHelperStatus {
	reason, message := storageHelperWaitingReason(pod)
	status := &StorageHelperStatus{
		PodName: pod.Name,
		Phase:   string(pod.Status.Phase),
		Ready:   storageHelperStatus(pod) == StorageHelperStatusReady,
		Reason:  reason,
		Message: message,
	}
	if withEvents {
		events := listEventsFor(namespace, pod.Name, "Pod")
		sort.SliceStable(events, func(i, j int) bool {
			return events[i].LastTimestamp > events[j].LastTimestamp
		})
		if len(events) > storageHelperEventLimit {
			events = events[:storageHelperEventLimit]
		}
		status.Events = events
	}
	return status
}

// collectPvcEvents lists the events for the PVC and (when bound) its PV,
// newest first. Queries are namespaced to the PVC's namespace, matching the
// legacy storagestatus behavior.
func collectPvcEvents(namespace, pvcName, pvName string) []StorageEvent {
	events := []StorageEvent{}
	events = append(events, listEventsFor(namespace, pvcName, "PersistentVolumeClaim")...)
	if pvName != "" {
		events = append(events, listEventsFor(namespace, pvName, "PersistentVolume")...)
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].LastTimestamp > events[j].LastTimestamp
	})
	return events
}

func listEventsFor(namespace, name, kind string) []StorageEvent {
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=%s", name, kind)
	eventList, err := clientProvider.K8sClientSet().CoreV1().Events(namespace).List(context.Background(), metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		serviceLogger.Warn("failed to list events", "namespace", namespace, "name", name, "kind", kind, "error", err)
		return nil
	}

	result := make([]StorageEvent, 0, len(eventList.Items))
	for _, event := range eventList.Items {
		result = append(result, StorageEvent{
			Type:          event.Type,
			Reason:        event.Reason,
			Message:       event.Message,
			LastTimestamp: event.LastTimestamp.Format(time.RFC3339),
		})
	}
	return result
}
