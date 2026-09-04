package services

import (
	"errors"
	"fmt"
	mokubernetes "mogenius-operator/src/kubernetes"
	"mogenius-operator/src/store"
	"sort"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
)

// Typed sentinel errors so callers (socket handlers, storage/v2/info) can map
// a failed resolution to a stable browsableReason on the wire.
var (
	ErrPvcNotMounted    = errors.New("pvc is not mounted by any pod")
	ErrPvcSubPathOnly   = errors.New("pvc is only mounted via subPath mounts")
	ErrPvcNoExecTooling = errors.New("no mounting container has the required exec tooling")
	ErrPvcPodNotReady   = errors.New("no pod mounting the pvc is running and ready")
)

// Browsable reason strings of the storage/v2 wire contract.
const (
	BrowsableReasonNotMounted    = "NOT_MOUNTED"
	BrowsableReasonSubPathOnly   = "SUBPATH_ONLY"
	BrowsableReasonNoExecTooling = "NO_EXEC_TOOLING"
	BrowsableReasonPodNotReady   = "POD_NOT_READY"
)

// BrowsableReasonForError maps a resolver sentinel error onto its wire reason.
// Unknown errors map to an empty string.
func BrowsableReasonForError(err error) string {
	switch {
	case errors.Is(err, ErrPvcNotMounted):
		return BrowsableReasonNotMounted
	case errors.Is(err, ErrPvcSubPathOnly):
		return BrowsableReasonSubPathOnly
	case errors.Is(err, ErrPvcNoExecTooling):
		return BrowsableReasonNoExecTooling
	case errors.Is(err, ErrPvcPodNotReady):
		return BrowsableReasonPodNotReady
	default:
		return ""
	}
}

// PvcMountTarget identifies one container that mounts a PVC without subPath,
// usable as an exec substrate for file operations on that PVC.
type PvcMountTarget struct {
	Namespace     string
	PodName       string
	ContainerName string
	MountPath     string
}

// probeFn checks whether the given container has minimal exec tooling by
// running `stat -c %F <mountPath>` in it. Injectable for tests.
type probeFn func(namespace, podName, containerName, mountPath string) error

func defaultExecProbe(namespace, podName, containerName, mountPath string) error {
	_, err := mokubernetes.ExecInPod(namespace, podName, containerName, []string{"stat", "-c", "%F", mountPath}, nil)
	return err
}

// probeCache remembers capability-probe outcomes per (namespace, pod,
// container) for a short time so repeated file operations on the same PVC do
// not re-exec the probe on every call.
type probeCacheEntry struct {
	err error
	at  time.Time
}

const probeCacheTTL = 60 * time.Second

var (
	probeCacheMu sync.Mutex
	probeCache   = map[string]probeCacheEntry{}
)

// cachedProbe wraps probe with the ~60s (namespace, pod, container) cache.
func cachedProbe(probe probeFn) probeFn {
	return func(namespace, podName, containerName, mountPath string) error {
		key := namespace + "/" + podName + "/" + containerName
		probeCacheMu.Lock()
		if entry, ok := probeCache[key]; ok && time.Since(entry.at) < probeCacheTTL {
			probeCacheMu.Unlock()
			return entry.err
		}
		probeCacheMu.Unlock()

		err := probe(namespace, podName, containerName, mountPath)

		probeCacheMu.Lock()
		probeCache[key] = probeCacheEntry{err: err, at: time.Now()}
		probeCacheMu.Unlock()
		return err
	}
}

// ResolvePvcTarget picks the container to exec file operations for the given
// PVC in: a running pod that mounts the PVC without subPath, whose container
// is ready and passes the exec-tooling probe. Candidates are tried in a
// deterministic order (pod creationTimestamp, pod name, container name).
// Returns one of the ErrPvc* sentinel errors when no usable candidate exists.
func ResolvePvcTarget(namespace, pvcName string) (PvcMountTarget, error) {
	pods := store.GetPods(namespace)
	target, err := resolvePvcTargetFrom(pods, namespace, pvcName, cachedProbe(defaultExecProbe))
	if err == nil {
		// single funnel of all files/v2 and storage/v2/stats calls: keep the
		// idle-TTL reaper away from helper pods that are actively used
		touchStorageHelperIfTarget(pods, target)
	}
	return target, err
}

// resolvePvcTargetFrom is the pure core of ResolvePvcTarget, split out so
// tests can feed in-memory pods and a stubbed probe.
func resolvePvcTargetFrom(pods []v1.Pod, namespace, pvcName string, probe probeFn) (PvcMountTarget, error) {
	candidates, structuralErr := pvcMountCandidates(pods, namespace, pvcName)
	if structuralErr != nil {
		return PvcMountTarget{}, structuralErr
	}

	for _, candidate := range candidates {
		if err := probe(candidate.Namespace, candidate.PodName, candidate.ContainerName, candidate.MountPath); err != nil {
			serviceLogger.Debug("pvc exec probe failed, trying next candidate",
				"namespace", candidate.Namespace, "pod", candidate.PodName, "container", candidate.ContainerName, "error", err)
			continue
		}
		return candidate, nil
	}

	return PvcMountTarget{}, fmt.Errorf("%w: %s/%s", ErrPvcNoExecTooling, namespace, pvcName)
}

// pvcMountCandidates returns all structurally usable exec candidates for the
// PVC, sorted deterministically. When none exist it returns the structural
// sentinel error describing why (NOT_MOUNTED / SUBPATH_ONLY / POD_NOT_READY).
func pvcMountCandidates(pods []v1.Pod, namespace, pvcName string) ([]PvcMountTarget, error) {
	mounted := false
	nonSubPathMount := false

	type sortableCandidate struct {
		target  PvcMountTarget
		created time.Time
	}
	var candidates []sortableCandidate

	for i := range pods {
		pod := &pods[i]

		// terminating pods vanish within their grace period - exec'ing into them
		// would race the kubelet, and after an unmount the UI must see NOT_MOUNTED
		if pod.DeletionTimestamp != nil {
			continue
		}

		// volume names in this pod backed by the requested claim
		volumeNames := map[string]bool{}
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvcName {
				volumeNames[volume.Name] = true
			}
		}
		if len(volumeNames) == 0 {
			continue
		}

		readyByContainer := map[string]bool{}
		for _, status := range pod.Status.ContainerStatuses {
			readyByContainer[status.Name] = status.Ready
		}

		// Spec.Containers only: init and ephemeral containers are no exec substrate.
		for _, container := range pod.Spec.Containers {
			for _, mount := range container.VolumeMounts {
				if !volumeNames[mount.Name] {
					continue
				}
				mounted = true
				if mount.SubPath != "" || mount.SubPathExpr != "" {
					continue
				}
				nonSubPathMount = true
				if pod.Status.Phase != v1.PodRunning || !readyByContainer[container.Name] {
					continue
				}
				candidates = append(candidates, sortableCandidate{
					target: PvcMountTarget{
						Namespace:     pod.Namespace,
						PodName:       pod.Name,
						ContainerName: container.Name,
						MountPath:     mount.MountPath,
					},
					created: pod.CreationTimestamp.Time,
				})
			}
		}
	}

	if len(candidates) == 0 {
		switch {
		case !mounted:
			return nil, fmt.Errorf("%w: %s/%s", ErrPvcNotMounted, namespace, pvcName)
		case !nonSubPathMount:
			return nil, fmt.Errorf("%w: %s/%s", ErrPvcSubPathOnly, namespace, pvcName)
		default:
			return nil, fmt.Errorf("%w: %s/%s", ErrPvcPodNotReady, namespace, pvcName)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if !a.created.Equal(b.created) {
			return a.created.Before(b.created)
		}
		if a.target.PodName != b.target.PodName {
			return a.target.PodName < b.target.PodName
		}
		return a.target.ContainerName < b.target.ContainerName
	})

	result := make([]PvcMountTarget, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, candidate.target)
	}
	return result, nil
}
