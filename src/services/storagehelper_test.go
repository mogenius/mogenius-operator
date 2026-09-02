package services

import (
	"regexp"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
)

var dns1123Label = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

func TestStorageHelperPodName(t *testing.T) {
	t.Run("short valid pvc name passes through", func(t *testing.T) {
		got := StorageHelperPodName("my-pvc")
		if got != "mo-storage-helper-my-pvc" {
			t.Fatalf("expected mo-storage-helper-my-pvc, got %q", got)
		}
	})

	t.Run("long names are truncated with a hash and do not collide", func(t *testing.T) {
		base := strings.Repeat("a", 60)
		nameA := StorageHelperPodName(base + "-one")
		nameB := StorageHelperPodName(base + "-two")
		if nameA == nameB {
			t.Fatalf("two long pvc names collided: %q", nameA)
		}
		for _, name := range []string{nameA, nameB} {
			if len(name) > 63 {
				t.Fatalf("pod name exceeds 63 chars (%d): %q", len(name), name)
			}
			if !dns1123Label.MatchString(name) {
				t.Fatalf("pod name is not a valid DNS-1123 label: %q", name)
			}
			if !strings.HasPrefix(name, "mo-storage-helper-") {
				t.Fatalf("pod name lost its prefix: %q", name)
			}
		}
	})

	t.Run("sanitized names get a hash so distinct pvcs stay distinct", func(t *testing.T) {
		nameDot := StorageHelperPodName("data.set")
		nameDash := StorageHelperPodName("data-set")
		if nameDot == nameDash {
			t.Fatalf("sanitization collided %q with %q", "data.set", "data-set")
		}
		if !dns1123Label.MatchString(nameDot) || len(nameDot) > 63 {
			t.Fatalf("sanitized name invalid: %q", nameDot)
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		long := strings.Repeat("x", 100)
		if StorageHelperPodName(long) != StorageHelperPodName(long) {
			t.Fatal("pod name derivation is not deterministic")
		}
	})
}

// helperPod builds an in-memory helper pod (label + pvc volume) for tests.
func helperPod(namespace, name, claimName string) v1.Pod {
	pod := makePvcPod(name, time.Now(), v1.PodRunning, claimName, []testMount{{container: storageHelperContainerName, ready: true}})
	pod.Namespace = namespace
	pod.Labels = map[string]string{StorageHelperLabelKey: StorageHelperLabelValue}
	return pod
}

func TestStorageHelperTrackerSelectIdle(t *testing.T) {
	now := time.Now()
	clock := now
	tracker := newStorageHelperTracker(func() time.Time { return clock })
	ttl := 15 * time.Minute

	pods := []v1.Pod{helperPod("ns", "helper-a", "pvc-a"), helperPod("ns", "helper-b", "pvc-b")}

	t.Run("unknown pods get first-seen and are not reaped yet", func(t *testing.T) {
		if idle := tracker.selectIdle(pods, ttl); len(idle) != 0 {
			t.Fatalf("expected no idle pods on first sighting, got %d", len(idle))
		}
	})

	t.Run("pods idle beyond the ttl are selected", func(t *testing.T) {
		clock = now.Add(16 * time.Minute)
		idle := tracker.selectIdle(pods, ttl)
		if len(idle) != 2 {
			t.Fatalf("expected both pods idle, got %d", len(idle))
		}
	})

	t.Run("touched pods are kept alive", func(t *testing.T) {
		tracker.touch("ns", "helper-a")
		idle := tracker.selectIdle(pods, ttl)
		if len(idle) != 1 || idle[0].Name != "helper-b" {
			t.Fatalf("expected only helper-b idle, got %+v", idle)
		}
	})

	t.Run("entries of vanished pods are pruned", func(t *testing.T) {
		remaining := []v1.Pod{pods[0]}
		tracker.selectIdle(remaining, ttl)
		tracker.mu.Lock()
		_, stillTracked := tracker.last[trackerKey("ns", "helper-b")]
		tracker.mu.Unlock()
		if stillTracked {
			t.Fatal("expected vanished pod helper-b to be pruned from the tracker")
		}
	})
}

func TestTouchStorageHelperIfTarget(t *testing.T) {
	clock := time.Now()
	original := helperTracker
	helperTracker = newStorageHelperTracker(func() time.Time { return clock })
	defer func() { helperTracker = original }()

	helper := helperPod("test-ns", "helper", "my-pvc")
	regular := makePvcPod("regular", time.Now(), v1.PodRunning, "my-pvc", []testMount{{container: "app", ready: true}})
	pods := []v1.Pod{helper, regular}

	touchStorageHelperIfTarget(pods, PvcMountTarget{Namespace: "test-ns", PodName: "regular", ContainerName: "app"})
	if len(helperTracker.last) != 0 {
		t.Fatal("resolving a regular pod must not touch the tracker")
	}

	touchStorageHelperIfTarget(pods, PvcMountTarget{Namespace: "test-ns", PodName: "helper", ContainerName: storageHelperContainerName})
	if _, ok := helperTracker.last[trackerKey("test-ns", "helper")]; !ok {
		t.Fatal("resolving the helper pod must touch the tracker")
	}
}

func buildTestIndex(pods ...v1.Pod) namespacePodIndex {
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

func TestHelperMountedFor(t *testing.T) {
	now := time.Now()

	t.Run("helper-labeled pod mounting the pvc sets the flag", func(t *testing.T) {
		idx := buildTestIndex(helperPod("test-ns", "helper", "my-pvc"))
		if !helperMountedFor(idx, "my-pvc") {
			t.Fatal("expected helperMounted=true")
		}
	})

	t.Run("regular pods do not set the flag", func(t *testing.T) {
		idx := buildTestIndex(makePvcPod("app", now, v1.PodRunning, "my-pvc", []testMount{{container: "app", ready: true}}))
		if helperMountedFor(idx, "my-pvc") {
			t.Fatal("expected helperMounted=false")
		}
	})

	t.Run("helper pod on another pvc does not set the flag", func(t *testing.T) {
		idx := buildTestIndex(helperPod("test-ns", "helper", "other-pvc"))
		if helperMountedFor(idx, "my-pvc") {
			t.Fatal("expected helperMounted=false for a helper on a different pvc")
		}
	})
}

// A helper pod that exists but is still Pending must render the item as
// browsable=false with POD_NOT_READY (not NOT_MOUNTED), so the UI shows the
// mount as in progress instead of offering the mount button again.
func TestPendingHelperPodYieldsPodNotReady(t *testing.T) {
	pending := makePvcPod("mo-storage-helper-my-pvc", time.Now(), v1.PodPending, "my-pvc", []testMount{{container: storageHelperContainerName, ready: false}})
	pending.Labels = map[string]string{StorageHelperLabelKey: StorageHelperLabelValue}
	idx := buildTestIndex(pending)

	mounts, browsable, reason := computeMounts(idx, "my-pvc")
	if browsable {
		t.Fatal("pending helper pod must not be browsable")
	}
	if reason != BrowsableReasonPodNotReady {
		t.Fatalf("expected POD_NOT_READY, got %q", reason)
	}
	if len(mounts) != 1 || mounts[0].ControllerKind != "Pod" {
		t.Fatalf("expected helper pod in mountedBy with controllerKind Pod, got %+v", mounts)
	}
	if !helperMountedFor(idx, "my-pvc") {
		t.Fatal("expected helperMounted=true for the pending helper pod")
	}
}

func TestStorageHelperPodSpec(t *testing.T) {
	pod := buildStorageHelperPod("test-ns", "my-pvc")

	if pod.Name != "mo-storage-helper-my-pvc" || pod.Namespace != "test-ns" {
		t.Fatalf("unexpected metadata: %s/%s", pod.Namespace, pod.Name)
	}
	if !isStorageHelperPod(pod) {
		t.Fatal("built pod must carry the helper label")
	}
	if !podClaimsPvc(pod, "my-pvc") {
		t.Fatal("built pod must claim the pvc")
	}
	if pod.Spec.TerminationGracePeriodSeconds == nil || *pod.Spec.TerminationGracePeriodSeconds != 5 {
		t.Fatal("expected terminationGracePeriodSeconds 5")
	}

	container := pod.Spec.Containers[0]
	if container.Image != defaultStorageHelperImage {
		t.Fatalf("expected pinned default image %q, got %q", defaultStorageHelperImage, container.Image)
	}
	if len(container.VolumeMounts) != 1 || container.VolumeMounts[0].MountPath != storageHelperMountPath || container.VolumeMounts[0].SubPath != "" {
		t.Fatalf("expected a single non-subPath mount at %s, got %+v", storageHelperMountPath, container.VolumeMounts)
	}
	securityContext := container.SecurityContext
	if securityContext == nil || securityContext.AllowPrivilegeEscalation == nil || *securityContext.AllowPrivilegeEscalation {
		t.Fatal("expected allowPrivilegeEscalation=false")
	}
	if securityContext.Privileged == nil || *securityContext.Privileged {
		t.Fatal("expected privileged=false")
	}
	// chown/chmod as uid 0 need CAP_CHOWN/CAP_FOWNER from the default set
	if securityContext.Capabilities != nil {
		t.Fatal("capabilities must not be restricted (default set required for chown/chmod as root)")
	}
}
