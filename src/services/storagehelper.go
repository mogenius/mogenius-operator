package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	mokubernetes "mogenius-operator/src/kubernetes"
	"mogenius-operator/src/shutdown"
	"mogenius-operator/src/store"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// Storage helper pods: operator-managed ephemeral busybox pods that mount an
// otherwise unmounted PVC at /data, so the existing exec-based file operations
// (files/v2/*, storage/v2/stats) have a pod to run in. Created on demand via
// storage/v2/mount, removed via storage/v2/unmount or by the idle reaper.

const (
	// StorageHelperLabelKey marks helper pods. Identification is this label
	// plus spec.volumes[].persistentVolumeClaim.claimName — never the pod
	// name, and never a pvc-name label value (63-char label limit).
	StorageHelperLabelKey   = "mogenius.io/storage-helper"
	StorageHelperLabelValue = "true"

	storageHelperNamePrefix    = "mo-storage-helper-"
	storageHelperContainerName = "helper"
	storageHelperMountPath     = "/data"

	// Fallback defaults; overridable via MO_STORAGE_HELPER_TTL and
	// MO_STORAGE_HELPER_IMAGE (declared in cmd.go).
	defaultStorageHelperTTL   = 15 * time.Minute
	defaultStorageHelperImage = "busybox:1.37" // renovate: datasource=docker depName=busybox

	// storageHelperReapInterval is how often the reaper scans for idle pods.
	storageHelperReapInterval = 60 * time.Second
)

// Wire statuses of the storage/v2/mount response.
const (
	StorageHelperStatusStarting = "STARTING"
	StorageHelperStatusReady    = "READY"
)

type StorageV2MountResponse struct {
	PodName string `json:"podName"`
	Status  string `json:"status"`
}

// ── pod naming ────────────────────────────────────────────────────────────────

// StorageHelperPodName derives the helper pod name for a PVC:
// "mo-storage-helper-<pvcName>" kept within the 63-char DNS-1123 label limit.
// When the pvc name must be truncated or sanitized, a short sha256 suffix of
// the ORIGINAL name keeps two long/odd pvc names from colliding.
func StorageHelperPodName(pvcName string) string {
	maxMiddle := validation.DNS1123LabelMaxLength - len(storageHelperNamePrefix)

	middle := sanitizeNameSegment(pvcName)
	if middle == pvcName && len(middle) <= maxMiddle {
		return storageHelperNamePrefix + middle
	}

	hash := sha256.Sum256([]byte(pvcName))
	suffix := hex.EncodeToString(hash[:])[:10]
	if len(middle) > maxMiddle-len(suffix)-1 {
		middle = middle[:maxMiddle-len(suffix)-1]
	}
	middle = strings.TrimRight(middle, "-")
	if middle == "" {
		return storageHelperNamePrefix + suffix
	}
	return storageHelperNamePrefix + middle + "-" + suffix
}

// sanitizeNameSegment lowercases s and replaces everything outside
// [a-z0-9-] with '-', trimming leading/trailing dashes.
func sanitizeNameSegment(s string) string {
	s = strings.ToLower(s)
	var builder strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('-')
		}
	}
	return strings.Trim(builder.String(), "-")
}

// ── config accessors (fall back to constants when config is not set up) ──────

func storageHelperTTL() time.Duration {
	if config != nil {
		if value, err := config.TryGet("MO_STORAGE_HELPER_TTL"); err == nil {
			if ttl, parseErr := time.ParseDuration(value); parseErr == nil && ttl > 0 {
				return ttl
			}
		}
	}
	return defaultStorageHelperTTL
}

func storageHelperImage() string {
	if config != nil {
		if value, err := config.TryGet("MO_STORAGE_HELPER_IMAGE"); err == nil && value != "" {
			return value
		}
	}
	return defaultStorageHelperImage
}

// ── pod spec ──────────────────────────────────────────────────────────────────

// buildStorageHelperPod builds the plain v1 Pod that mounts the PVC at /data.
// Runs as root (busybox default) but unprivileged: allowPrivilegeEscalation
// false, no host mounts. Capabilities are deliberately NOT dropped — the file
// operations exec chown/chmod as uid 0, which needs CAP_CHOWN/CAP_FOWNER from
// the default capability set.
func buildStorageHelperPod(namespace, pvcName string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StorageHelperPodName(pvcName),
			Namespace: namespace,
			Labels: map[string]string{
				StorageHelperLabelKey:          StorageHelperLabelValue,
				"app.kubernetes.io/managed-by": "mogenius-k8s-manager",
			},
		},
		Spec: v1.PodSpec{
			// RWO note: the UI only offers mounting for NOT_MOUNTED PVCs, so an
			// RWO volume attached elsewhere is not expected here. No node
			// affinity logic in v1 — the scheduler places pods for unattached
			// volumes; a pod stuck Pending simply reports STARTING and the idle
			// reaper removes it after the TTL.
			RestartPolicy:                 v1.RestartPolicyAlways,
			TerminationGracePeriodSeconds: new(int64(5)),
			Volumes: []v1.Volume{
				{
					Name: "data",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
					},
				},
			},
			Containers: []v1.Container{
				{
					Name:    storageHelperContainerName,
					Image:   storageHelperImage(),
					Command: []string{"sh", "-c", "trap 'exit 0' TERM; while true; do sleep 3600 & wait $!; done"},
					VolumeMounts: []v1.VolumeMount{
						{Name: "data", MountPath: storageHelperMountPath},
					},
					SecurityContext: &v1.SecurityContext{
						Privileged:               new(false),
						AllowPrivilegeEscalation: new(false),
					},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("10m"),
							v1.ResourceMemory: resource.MustParse("16Mi"),
						},
						Limits: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("100m"),
							v1.ResourceMemory: resource.MustParse("64Mi"),
						},
					},
				},
			},
		},
	}
}

// ── helper pod detection ──────────────────────────────────────────────────────

// isStorageHelperPod reports whether the pod carries the helper label.
func isStorageHelperPod(pod *v1.Pod) bool {
	return pod.Labels[StorageHelperLabelKey] == StorageHelperLabelValue
}

// podClaimsPvc reports whether the pod references the claim in spec.volumes.
func podClaimsPvc(pod *v1.Pod, pvcName string) bool {
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvcName {
			return true
		}
	}
	return false
}

// findStorageHelperPod returns the first helper pod claiming the PVC, nil when
// none exists. Identification via label + claimName, not via the pod name.
func findStorageHelperPod(pods []v1.Pod, pvcName string) *v1.Pod {
	for i := range pods {
		if isStorageHelperPod(&pods[i]) && podClaimsPvc(&pods[i], pvcName) {
			return &pods[i]
		}
	}
	return nil
}

// helperPodFor returns the first helper-labeled pod referencing the PVC in the
// pre-built namespace pod index, nil when none exists. Feeds storage/v2/info's
// helperMounted flag and helperStatus field from the one scan the index holds.
func helperPodFor(index namespacePodIndex, pvcName string) *v1.Pod {
	for _, podIdx := range index.podsPerClaim[pvcName] {
		pod := &index.pods[podIdx]
		// a helper that is already terminating (unmount in progress) no longer counts as mounted
		if isStorageHelperPod(pod) && pod.DeletionTimestamp == nil {
			return pod
		}
	}
	return nil
}

// helperMountedFor reports whether a helper pod references the PVC.
func helperMountedFor(index namespacePodIndex, pvcName string) bool {
	return helperPodFor(index, pvcName) != nil
}

// storageHelperStatus maps a helper pod's state onto the wire status.
func storageHelperStatus(pod *v1.Pod) string {
	if pod.Status.Phase != v1.PodRunning {
		return StorageHelperStatusStarting
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == storageHelperContainerName && status.Ready {
			return StorageHelperStatusReady
		}
	}
	return StorageHelperStatusStarting
}

// storageHelperWaitingReason derives why a helper pod is not (yet) running:
// the first container's Waiting state (e.g. "ContainerCreating" while kubelet
// retries a FailedMount) is preferred; a Pending pod with no container status
// yet falls back to its PodScheduled/ContainersReady condition. Both empty
// when the pod runs fine.
func storageHelperWaitingReason(pod *v1.Pod) (reason string, message string) {
	if len(pod.Status.ContainerStatuses) > 0 {
		if waiting := pod.Status.ContainerStatuses[0].State.Waiting; waiting != nil {
			return waiting.Reason, waiting.Message
		}
		return "", ""
	}
	if pod.Status.Phase == v1.PodPending {
		for _, conditionType := range []v1.PodConditionType{v1.PodScheduled, v1.ContainersReady} {
			for _, condition := range pod.Status.Conditions {
				if condition.Type == conditionType && condition.Status != v1.ConditionTrue && condition.Reason != "" {
					return condition.Reason, condition.Message
				}
			}
		}
	}
	return "", ""
}

// ── mount / unmount ──────────────────────────────────────────────────────────

// StorageV2Mount ensures a helper pod exists for the PVC (idempotent): an
// existing helper pod is reported with its current state, otherwise the pod is
// created and reported as STARTING.
func StorageV2Mount(namespace, pvcName string) (StorageV2MountResponse, error) {
	// existing helper pod: store first, live fallback by derived name
	if existing := findStorageHelperPod(store.GetPods(namespace), pvcName); existing != nil {
		helperTracker.touch(namespace, existing.Name)
		return StorageV2MountResponse{PodName: existing.Name, Status: storageHelperStatus(existing)}, nil
	}

	podName := StorageHelperPodName(pvcName)
	podClient := clientProvider.K8sClientSet().CoreV1().Pods(namespace)
	if live, err := podClient.Get(context.Background(), podName, metav1.GetOptions{}); err == nil {
		if isStorageHelperPod(live) && podClaimsPvc(live, pvcName) {
			helperTracker.touch(namespace, live.Name)
			return StorageV2MountResponse{PodName: live.Name, Status: storageHelperStatus(live)}, nil
		}
		// name collision with a foreign pod — refuse instead of adopting it
		return StorageV2MountResponse{}, fmt.Errorf("pod %s/%s already exists and is not a mogenius storage helper", namespace, podName)
	}

	pod := buildStorageHelperPod(namespace, pvcName)
	_, err := podClient.Create(context.Background(), pod, mokubernetes.MoCreateOptions(config))
	switch {
	case err == nil:
		serviceLogger.Info("created storage helper pod", "namespace", namespace, "pvc", pvcName, "pod", podName)
	case apierrors.IsAlreadyExists(err):
		// concurrent mount request already created it — idempotent success
	default:
		return StorageV2MountResponse{}, fmt.Errorf("failed to create storage helper pod %s/%s: %w", namespace, podName, err)
	}

	helperTracker.touch(namespace, podName)
	return StorageV2MountResponse{PodName: podName, Status: StorageHelperStatusStarting}, nil
}

// StorageV2Unmount deletes the helper pod(s) for the PVC. Idempotent: no
// helper pod is not an error.
func StorageV2Unmount(namespace, pvcName string) error {
	podClient := clientProvider.K8sClientSet().CoreV1().Pods(namespace)

	// candidates: helper pods from the store plus the derived name (covers a
	// store that lags right after creation)
	names := map[string]bool{StorageHelperPodName(pvcName): true}
	for _, pod := range store.GetPods(namespace) {
		if isStorageHelperPod(&pod) && podClaimsPvc(&pod, pvcName) {
			names[pod.Name] = true
		}
	}

	for name := range names {
		err := podClient.Delete(context.Background(), name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete storage helper pod %s/%s: %w", namespace, name, err)
		}
		if err == nil {
			serviceLogger.Info("deleted storage helper pod", "namespace", namespace, "pvc", pvcName, "pod", name)
		}
		helperTracker.forget(namespace, name)
	}
	return nil
}

// ── activity tracking ─────────────────────────────────────────────────────────

// storageHelperTracker remembers the last activity time per helper pod. Purely
// in-memory: after an operator restart a pod's activity clock starts at its
// first sighting by the reaper.
type storageHelperTracker struct {
	mu   sync.Mutex
	last map[string]time.Time
	now  func() time.Time
}

func newStorageHelperTracker(now func() time.Time) *storageHelperTracker {
	return &storageHelperTracker{last: map[string]time.Time{}, now: now}
}

var helperTracker = newStorageHelperTracker(time.Now)

func trackerKey(namespace, podName string) string {
	return namespace + "/" + podName
}

func (t *storageHelperTracker) touch(namespace, podName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.last[trackerKey(namespace, podName)] = t.now()
}

func (t *storageHelperTracker) forget(namespace, podName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.last, trackerKey(namespace, podName))
}

// selectIdle returns the pods idle longer than ttl and prunes tracker entries
// whose pods are gone. A pod unknown to the tracker (operator restarted) gets
// its first sighting as activity start and is never selected on that sweep.
func (t *storageHelperTracker) selectIdle(pods []v1.Pod, ttl time.Duration) []v1.Pod {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now()
	seen := map[string]bool{}
	var idle []v1.Pod
	for i := range pods {
		key := trackerKey(pods[i].Namespace, pods[i].Name)
		seen[key] = true
		lastActivity, known := t.last[key]
		if !known {
			t.last[key] = now
			continue
		}
		if now.Sub(lastActivity) > ttl {
			idle = append(idle, pods[i])
		}
	}

	// prune entries for pods that no longer exist
	for key := range t.last {
		if !seen[key] {
			delete(t.last, key)
		}
	}
	return idle
}

// touchStorageHelperIfTarget marks activity when the resolved exec target is a
// helper pod. Called from ResolvePvcTarget, the single funnel every files/v2
// operation and storage/v2/stats call goes through.
func touchStorageHelperIfTarget(pods []v1.Pod, target PvcMountTarget) {
	for i := range pods {
		if pods[i].Name == target.PodName && pods[i].Namespace == target.Namespace {
			if isStorageHelperPod(&pods[i]) {
				helperTracker.touch(target.Namespace, target.PodName)
			}
			return
		}
	}
}

// ── idle reaper ───────────────────────────────────────────────────────────────

// StartStorageHelperReaper starts the background loop that deletes helper pods
// idle longer than the TTL. Started once from cmd.startClusterSystems.
func StartStorageHelperReaper() {
	stop := make(chan struct{})
	shutdown.Add(func() { close(stop) })

	go func() {
		ticker := time.NewTicker(storageHelperReapInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				reapIdleStorageHelpers()
				reapStorageHelpersOfTerminatingPvcs()
			}
		}
	}()
}

// listStorageHelperPods lists all helper pods cluster-wide, store-backed with
// a live label-selector fallback when the store yields nothing.
func listStorageHelperPods() []v1.Pod {
	var helpers []v1.Pod
	pods := store.GetPods("*")
	for i := range pods {
		if isStorageHelperPod(&pods[i]) {
			helpers = append(helpers, pods[i])
		}
	}
	if pods != nil {
		return helpers
	}

	// live fallback (store error, e.g. right after a store wipe)
	list, err := clientProvider.K8sClientSet().CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		LabelSelector: StorageHelperLabelKey + "=" + StorageHelperLabelValue,
	})
	if err != nil {
		serviceLogger.Warn("storage helper reaper: failed to list helper pods", "error", err)
		return nil
	}
	return list.Items
}

// helperPodClaim returns the PVC a helper pod mounts (its single "data" volume).
func helperPodClaim(pod *v1.Pod) string {
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			return volume.PersistentVolumeClaim.ClaimName
		}
	}
	return ""
}

// selectHelpersOfTerminatingPvcs returns the live helper pods whose PVC carries
// a deletionTimestamp. Such a pod pins kubernetes.io/pvc-protection and the PVC
// would hang in Terminating forever - e.g. after a kubectl delete that bypassed
// the API's unmount-before-delete.
func selectHelpersOfTerminatingPvcs(pods []v1.Pod, lookup func(namespace, name string) *v1.PersistentVolumeClaim) []v1.Pod {
	var blocking []v1.Pod
	for i := range pods {
		pod := &pods[i]
		if pod.DeletionTimestamp != nil {
			continue
		}
		claim := helperPodClaim(pod)
		if claim == "" {
			continue
		}
		if pvc := lookup(pod.Namespace, claim); pvc != nil && pvc.DeletionTimestamp != nil {
			blocking = append(blocking, *pod)
		}
	}
	return blocking
}

func reapStorageHelpersOfTerminatingPvcs() {
	blocking := selectHelpersOfTerminatingPvcs(listStorageHelperPods(), getPvc)
	for _, pod := range blocking {
		err := clientProvider.K8sClientSet().CoreV1().Pods(pod.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			serviceLogger.Warn("storage helper reaper: failed to delete helper pod of terminating pvc",
				"namespace", pod.Namespace, "pod", pod.Name, "error", err)
			continue
		}
		serviceLogger.Info("storage helper reaper: deleted helper pod of terminating pvc",
			"namespace", pod.Namespace, "pod", pod.Name, "pvc", helperPodClaim(&pod))
		helperTracker.forget(pod.Namespace, pod.Name)
	}
}

func reapIdleStorageHelpers() {
	idle := helperTracker.selectIdle(listStorageHelperPods(), storageHelperTTL())
	for _, pod := range idle {
		err := clientProvider.K8sClientSet().CoreV1().Pods(pod.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			serviceLogger.Warn("storage helper reaper: failed to delete idle helper pod",
				"namespace", pod.Namespace, "pod", pod.Name, "error", err)
			continue
		}
		serviceLogger.Info("storage helper reaper: deleted idle helper pod",
			"namespace", pod.Namespace, "pod", pod.Name, "ttl", storageHelperTTL().String())
		helperTracker.forget(pod.Namespace, pod.Name)
	}
}
