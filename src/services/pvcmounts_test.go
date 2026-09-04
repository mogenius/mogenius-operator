package services

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	// the resolver logs skipped candidates; tests run without Setup()
	serviceLogger = slog.New(slog.NewTextHandler(io.Discard, nil))
}

type testMount struct {
	container string
	subPath   bool
	ready     bool
}

// makePvcPod builds an in-memory pod that mounts claimName in the given
// containers via a single volume named "data".
func makePvcPod(name string, created time.Time, phase v1.PodPhase, claimName string, mounts []testMount) v1.Pod {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "test-ns",
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{
				{
					Name: "data",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: claimName},
					},
				},
			},
		},
		Status: v1.PodStatus{Phase: phase},
	}
	for _, mount := range mounts {
		volumeMount := v1.VolumeMount{Name: "data", MountPath: "/mnt/data"}
		if mount.subPath {
			volumeMount.SubPath = "sub"
		}
		pod.Spec.Containers = append(pod.Spec.Containers, v1.Container{
			Name:         mount.container,
			VolumeMounts: []v1.VolumeMount{volumeMount},
		})
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
			Name:  mount.container,
			Ready: mount.ready,
		})
	}
	return pod
}

func probeOk(string, string, string, string) error { return nil }

func TestPvcMountCandidatesSkipTerminatingPods(t *testing.T) {
	now := time.Now()
	terminating := makePvcPod("helper", now, v1.PodRunning, "pvc", []testMount{{container: "helper", ready: true}})
	deleted := metav1.NewTime(now)
	terminating.DeletionTimestamp = &deleted

	// an unmounted helper still lingers in the store during its grace period;
	// the UI must already see NOT_MOUNTED instead of a browsable volume
	_, err := pvcMountCandidates([]v1.Pod{terminating}, "test-ns", "pvc")
	if !errors.Is(err, ErrPvcNotMounted) {
		t.Fatalf("expected ErrPvcNotMounted for a terminating pod, got %v", err)
	}
}

func TestResolvePvcTargetFromStructuralErrors(t *testing.T) {
	now := time.Now()

	t.Run("no pod mounts the pvc", func(t *testing.T) {
		pods := []v1.Pod{makePvcPod("a", now, v1.PodRunning, "other-pvc", []testMount{{container: "app", ready: true}})}
		_, err := resolvePvcTargetFrom(pods, "test-ns", "my-pvc", probeOk)
		if !errors.Is(err, ErrPvcNotMounted) {
			t.Fatalf("expected ErrPvcNotMounted, got %v", err)
		}
	})

	t.Run("only subPath mounts", func(t *testing.T) {
		pods := []v1.Pod{makePvcPod("a", now, v1.PodRunning, "my-pvc", []testMount{{container: "app", subPath: true, ready: true}})}
		_, err := resolvePvcTargetFrom(pods, "test-ns", "my-pvc", probeOk)
		if !errors.Is(err, ErrPvcSubPathOnly) {
			t.Fatalf("expected ErrPvcSubPathOnly, got %v", err)
		}
	})

	t.Run("pod not running", func(t *testing.T) {
		pods := []v1.Pod{makePvcPod("a", now, v1.PodPending, "my-pvc", []testMount{{container: "app", ready: false}})}
		_, err := resolvePvcTargetFrom(pods, "test-ns", "my-pvc", probeOk)
		if !errors.Is(err, ErrPvcPodNotReady) {
			t.Fatalf("expected ErrPvcPodNotReady, got %v", err)
		}
	})

	t.Run("container not ready", func(t *testing.T) {
		pods := []v1.Pod{makePvcPod("a", now, v1.PodRunning, "my-pvc", []testMount{{container: "app", ready: false}})}
		_, err := resolvePvcTargetFrom(pods, "test-ns", "my-pvc", probeOk)
		if !errors.Is(err, ErrPvcPodNotReady) {
			t.Fatalf("expected ErrPvcPodNotReady, got %v", err)
		}
	})
}

func TestResolvePvcTargetFromDeterministicOrder(t *testing.T) {
	now := time.Now()

	t.Run("oldest pod wins", func(t *testing.T) {
		pods := []v1.Pod{
			makePvcPod("younger", now, v1.PodRunning, "my-pvc", []testMount{{container: "app", ready: true}}),
			makePvcPod("older", now.Add(-time.Hour), v1.PodRunning, "my-pvc", []testMount{{container: "app", ready: true}}),
		}
		target, err := resolvePvcTargetFrom(pods, "test-ns", "my-pvc", probeOk)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if target.PodName != "older" {
			t.Fatalf("expected pod 'older', got %q", target.PodName)
		}
	})

	t.Run("equal timestamps fall back to pod name, then container name", func(t *testing.T) {
		pods := []v1.Pod{
			makePvcPod("pod-b", now, v1.PodRunning, "my-pvc", []testMount{{container: "app", ready: true}}),
			makePvcPod("pod-a", now, v1.PodRunning, "my-pvc", []testMount{
				{container: "zeta", ready: true},
				{container: "alpha", ready: true},
			}),
		}
		target, err := resolvePvcTargetFrom(pods, "test-ns", "my-pvc", probeOk)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if target.PodName != "pod-a" || target.ContainerName != "alpha" {
			t.Fatalf("expected pod-a/alpha, got %s/%s", target.PodName, target.ContainerName)
		}
		if target.MountPath != "/mnt/data" {
			t.Fatalf("expected mount path /mnt/data, got %q", target.MountPath)
		}
	})
}

func TestResolvePvcTargetFromProbe(t *testing.T) {
	now := time.Now()

	t.Run("failing probe skips to the next candidate", func(t *testing.T) {
		pods := []v1.Pod{
			makePvcPod("first", now.Add(-time.Hour), v1.PodRunning, "my-pvc", []testMount{{container: "distroless", ready: true}}),
			makePvcPod("second", now, v1.PodRunning, "my-pvc", []testMount{{container: "app", ready: true}}),
		}
		probe := func(_, pod, container, _ string) error {
			if container == "distroless" {
				return fmt.Errorf("exec failed: executable file not found")
			}
			return nil
		}
		target, err := resolvePvcTargetFrom(pods, "test-ns", "my-pvc", probe)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if target.PodName != "second" || target.ContainerName != "app" {
			t.Fatalf("expected second/app, got %s/%s", target.PodName, target.ContainerName)
		}
	})

	t.Run("all probes failing yields ErrPvcNoExecTooling", func(t *testing.T) {
		pods := []v1.Pod{
			makePvcPod("a", now, v1.PodRunning, "my-pvc", []testMount{{container: "app", ready: true}}),
		}
		probe := func(string, string, string, string) error { return fmt.Errorf("no tooling") }
		_, err := resolvePvcTargetFrom(pods, "test-ns", "my-pvc", probe)
		if !errors.Is(err, ErrPvcNoExecTooling) {
			t.Fatalf("expected ErrPvcNoExecTooling, got %v", err)
		}
	})
}

func TestBrowsableReasonForError(t *testing.T) {
	cases := map[string]struct {
		err    error
		reason string
	}{
		"not mounted":     {fmt.Errorf("wrap: %w", ErrPvcNotMounted), BrowsableReasonNotMounted},
		"subpath only":    {ErrPvcSubPathOnly, BrowsableReasonSubPathOnly},
		"no exec tooling": {ErrPvcNoExecTooling, BrowsableReasonNoExecTooling},
		"pod not ready":   {ErrPvcPodNotReady, BrowsableReasonPodNotReady},
		"unknown":         {errors.New("boom"), ""},
	}
	for name, c := range cases {
		if got := BrowsableReasonForError(c.err); got != c.reason {
			t.Errorf("%s: expected %q, got %q", name, c.reason, got)
		}
	}
}

func TestComputeMountsStructural(t *testing.T) {
	now := time.Now()

	index := func(pods ...v1.Pod) namespacePodIndex {
		idx := namespacePodIndex{pods: pods, podsPerClaim: map[string][]int{}}
		for i := range pods {
			for _, volume := range pods[i].Spec.Volumes {
				if volume.PersistentVolumeClaim != nil {
					claim := volume.PersistentVolumeClaim.ClaimName
					idx.podsPerClaim[claim] = append(idx.podsPerClaim[claim], i)
				}
			}
		}
		return idx
	}

	t.Run("running ready non-subPath mount is browsable", func(t *testing.T) {
		idx := index(makePvcPod("a", now, v1.PodRunning, "my-pvc", []testMount{{container: "app", ready: true}}))
		mounts, browsable, reason := computeMounts(idx, "my-pvc")
		if !browsable || reason != "" {
			t.Fatalf("expected browsable with empty reason, got browsable=%v reason=%q", browsable, reason)
		}
		if len(mounts) != 1 || mounts[0].PodName != "a" || mounts[0].ControllerKind != "Pod" || mounts[0].ControllerName != "a" {
			t.Fatalf("unexpected mounts: %+v", mounts)
		}
	})

	t.Run("no mounts", func(t *testing.T) {
		idx := index(makePvcPod("a", now, v1.PodRunning, "other", []testMount{{container: "app", ready: true}}))
		mounts, browsable, reason := computeMounts(idx, "my-pvc")
		if browsable || reason != BrowsableReasonNotMounted || len(mounts) != 0 {
			t.Fatalf("expected NOT_MOUNTED, got browsable=%v reason=%q mounts=%d", browsable, reason, len(mounts))
		}
	})

	t.Run("subPath only", func(t *testing.T) {
		idx := index(makePvcPod("a", now, v1.PodRunning, "my-pvc", []testMount{{container: "app", subPath: true, ready: true}}))
		mounts, browsable, reason := computeMounts(idx, "my-pvc")
		if browsable || reason != BrowsableReasonSubPathOnly {
			t.Fatalf("expected SUBPATH_ONLY, got browsable=%v reason=%q", browsable, reason)
		}
		if len(mounts) != 1 || !mounts[0].SubPath {
			t.Fatalf("expected one subPath mount entry, got %+v", mounts)
		}
	})

	t.Run("pod not ready", func(t *testing.T) {
		idx := index(makePvcPod("a", now, v1.PodPending, "my-pvc", []testMount{{container: "app", ready: false}}))
		_, browsable, reason := computeMounts(idx, "my-pvc")
		if browsable || reason != BrowsableReasonPodNotReady {
			t.Fatalf("expected POD_NOT_READY, got browsable=%v reason=%q", browsable, reason)
		}
	})

	t.Run("direct owner is attributed without replicaset walk", func(t *testing.T) {
		pod := makePvcPod("db-0", now, v1.PodRunning, "my-pvc", []testMount{{container: "db", ready: true}})
		controller := true
		pod.OwnerReferences = []metav1.OwnerReference{{Kind: "StatefulSet", Name: "db", Controller: &controller}}
		idx := index(pod)
		mounts, _, _ := computeMounts(idx, "my-pvc")
		if len(mounts) != 1 || mounts[0].ControllerKind != "StatefulSet" || mounts[0].ControllerName != "db" {
			t.Fatalf("unexpected controller attribution: %+v", mounts)
		}
	})
}
