package debugcontainer

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func podFixture(ephemeral []v1.EphemeralContainer, statuses []v1.ContainerStatus) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-0", Namespace: "prod"},
		Spec: v1.PodSpec{
			Containers:          []v1.Container{{Name: "app", Image: "gcr.io/distroless/static"}},
			EphemeralContainers: ephemeral,
		},
		Status: v1.PodStatus{
			Phase:                      v1.PodRunning,
			EphemeralContainerStatuses: statuses,
		},
	}
}

func running(name string) v1.ContainerStatus {
	return v1.ContainerStatus{Name: name, State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}}
}

func terminated(name string) v1.ContainerStatus {
	return v1.ContainerStatus{Name: name, State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{ExitCode: 0, Reason: "Completed"}}}
}

// reportRunningOnUpdate makes the fake kubelet: as soon as the spec gains an
// ephemeral container, its status is reported as Running.
func reportRunningOnUpdate(client *fake.Clientset) {
	client.PrependReactor("update", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		update := action.(k8stesting.UpdateAction)
		pod := update.GetObject().(*v1.Pod)
		for _, ec := range pod.Spec.EphemeralContainers {
			found := false
			for _, s := range pod.Status.EphemeralContainerStatuses {
				if s.Name == ec.Name {
					found = true
				}
			}
			if !found {
				pod.Status.EphemeralContainerStatuses = append(pod.Status.EphemeralContainerStatuses, running(ec.Name))
			}
		}
		return false, nil, nil // fall through to the tracker with the mutated object
	})
}

func fastOpts(target string) Options {
	return Options{TargetContainer: target, Timeout: 2 * time.Second, pollInterval: time.Millisecond}
}

func TestEnsureAttachesAndWaitsForRunning(t *testing.T) {
	client := fake.NewClientset(podFixture(nil, nil))
	reportRunningOnUpdate(client)
	var out bytes.Buffer
	opts := fastOpts("app")
	opts.Progress = &out

	name, err := Ensure(context.Background(), client, "prod", "api-0", opts)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if name != NamePrefix {
		t.Fatalf("name = %q, want %q", name, NamePrefix)
	}

	pod, _ := client.CoreV1().Pods("prod").Get(context.Background(), "api-0", metav1.GetOptions{})
	if len(pod.Spec.EphemeralContainers) != 1 {
		t.Fatalf("ephemeral containers = %d, want 1", len(pod.Spec.EphemeralContainers))
	}
	ec := pod.Spec.EphemeralContainers[0]
	if ec.TargetContainerName != "app" {
		t.Errorf("targetContainerName = %q, want app", ec.TargetContainerName)
	}
	if ec.Image != DefaultImage {
		t.Errorf("image = %q, want default %q", ec.Image, DefaultImage)
	}
	if ec.SecurityContext == nil || ec.SecurityContext.AllowPrivilegeEscalation == nil || *ec.SecurityContext.AllowPrivilegeEscalation {
		t.Errorf("allowPrivilegeEscalation must be false")
	}
	if !strings.Contains(out.String(), "attaching debug container") || !strings.Contains(out.String(), "is running") {
		t.Errorf("progress output missing steps: %q", out.String())
	}
	if !strings.HasSuffix(out.String(), "\r\n") {
		t.Errorf("progress lines must end in CRLF for raw terminals")
	}
}

// Opening the terminal twice must not grow the pod spec: ephemeral containers
// cannot be removed, so a running one is reused.
func TestEnsureReusesRunningDebugContainer(t *testing.T) {
	existing := buildSpec(NamePrefix, DefaultImage, "app")
	client := fake.NewClientset(podFixture([]v1.EphemeralContainer{existing}, []v1.ContainerStatus{running(NamePrefix)}))
	updates := 0
	client.PrependReactor("update", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		updates++
		return false, nil, nil
	})

	name, err := Ensure(context.Background(), client, "prod", "api-0", fastOpts("app"))
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if name != NamePrefix {
		t.Fatalf("name = %q, want reuse of %q", name, NamePrefix)
	}
	if updates != 0 {
		t.Fatalf("pod was updated %d times, want 0 (reuse)", updates)
	}
}

// A debug container that targets a different workload container lives in a
// different PID namespace; reusing it would show the wrong processes.
func TestEnsureDoesNotReuseContainerTargetingAnotherContainer(t *testing.T) {
	existing := buildSpec(NamePrefix, DefaultImage, "sidecar")
	client := fake.NewClientset(podFixture([]v1.EphemeralContainer{existing}, []v1.ContainerStatus{running(NamePrefix)}))
	reportRunningOnUpdate(client)

	name, err := Ensure(context.Background(), client, "prod", "api-0", fastOpts("app"))
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if name != NamePrefix+"-2" {
		t.Fatalf("name = %q, want a new numbered container", name)
	}
}

// A terminated debug container cannot be restarted; the next one gets a new
// number instead of colliding with the dead entry.
func TestEnsureNumbersPastTerminatedContainers(t *testing.T) {
	dead := buildSpec(NamePrefix, DefaultImage, "app")
	dead2 := buildSpec(NamePrefix+"-2", DefaultImage, "app")
	client := fake.NewClientset(podFixture(
		[]v1.EphemeralContainer{dead, dead2},
		[]v1.ContainerStatus{terminated(NamePrefix), terminated(NamePrefix + "-2")},
	))
	reportRunningOnUpdate(client)

	name, err := Ensure(context.Background(), client, "prod", "api-0", fastOpts("app"))
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if name != NamePrefix+"-3" {
		t.Fatalf("name = %q, want %q", name, NamePrefix+"-3")
	}
}

func TestEnsureExplainsPodSecurityRejection(t *testing.T) {
	client := fake.NewClientset(podFixture(nil, nil))
	client.PrependReactor("update", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(
			schema.GroupResource{Resource: "pods"}, "api-0",
			errors.New(`violates PodSecurity "restricted:latest": allowPrivilegeEscalation != false`))
	})

	_, err := Ensure(context.Background(), client, "prod", "api-0", fastOpts("app"))
	if err == nil || !strings.Contains(err.Error(), "Pod Security Admission") {
		t.Fatalf("want a Pod Security explanation, got %v", err)
	}
}

func TestEnsureExplainsMissingRBAC(t *testing.T) {
	client := fake.NewClientset(podFixture(nil, nil))
	client.PrependReactor("update", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(
			schema.GroupResource{Resource: "pods"}, "api-0",
			errors.New(`User "jane" cannot update resource "pods/ephemeralcontainers"`))
	})

	_, err := Ensure(context.Background(), client, "prod", "api-0", fastOpts("app"))
	if err == nil || !strings.Contains(err.Error(), "pods/ephemeralcontainers") {
		t.Fatalf("want an RBAC hint, got %v", err)
	}
}

func TestEnsureFailsFastOnImagePullError(t *testing.T) {
	client := fake.NewClientset(podFixture(nil, nil))
	client.PrependReactor("update", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		pod := action.(k8stesting.UpdateAction).GetObject().(*v1.Pod)
		pod.Status.EphemeralContainerStatuses = []v1.ContainerStatus{{
			Name:  NamePrefix,
			State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "Back-off pulling image"}},
		}}
		return false, nil, nil
	})

	start := time.Now()
	_, err := Ensure(context.Background(), client, "prod", "api-0", fastOpts("app"))
	if err == nil || !strings.Contains(err.Error(), "cannot pull the debug image") || !strings.Contains(err.Error(), ConfigKeyImage) {
		t.Fatalf("want an image pull explanation naming the config key, got %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("pull error should fail fast, took %s", time.Since(start))
	}
}

func TestEnsureTimesOutWithLastState(t *testing.T) {
	client := fake.NewClientset(podFixture(nil, nil))
	client.PrependReactor("update", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		pod := action.(k8stesting.UpdateAction).GetObject().(*v1.Pod)
		pod.Status.EphemeralContainerStatuses = []v1.ContainerStatus{{
			Name:  NamePrefix,
			State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "ContainerCreating"}},
		}}
		return false, nil, nil
	})

	opts := fastOpts("app")
	opts.Timeout = 20 * time.Millisecond
	_, err := Ensure(context.Background(), client, "prod", "api-0", opts)
	if err == nil || !strings.Contains(err.Error(), "did not start") || !strings.Contains(err.Error(), "ContainerCreating") {
		t.Fatalf("want a timeout naming the last state, got %v", err)
	}
}

func TestEnsureRequiresTargetContainer(t *testing.T) {
	client := fake.NewClientset(podFixture(nil, nil))
	if _, err := Ensure(context.Background(), client, "prod", "api-0", fastOpts("")); err == nil {
		t.Fatal("want an error without a target container")
	}
}

func TestIsDebugContainer(t *testing.T) {
	for name, want := range map[string]bool{
		NamePrefix:            true,
		NamePrefix + "-2":     true,
		NamePrefix + "ger":    false,
		"app":                 false,
		"debug":               false,
		"mogenius-debugger-1": false,
	} {
		if got := IsDebugContainer(name); got != want {
			t.Errorf("IsDebugContainer(%q) = %v, want %v", name, got, want)
		}
	}
}
