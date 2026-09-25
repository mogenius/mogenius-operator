package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"mogenius-operator/src/k8sexec"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestResolveExecTimeout(t *testing.T) {
	maxTimeout := 300 * time.Second

	got, err := resolveExecTimeout(0, maxTimeout)
	if err != nil || got != k8sexec.DefaultRunTimeout {
		t.Errorf("no timeout: got %s, %v; want the default %s", got, err, k8sexec.DefaultRunTimeout)
	}

	got, err = resolveExecTimeout(120, maxTimeout)
	if err != nil || got != 120*time.Second {
		t.Errorf("120s: got %s, %v", got, err)
	}

	got, err = resolveExecTimeout(300, maxTimeout)
	if err != nil || got != maxTimeout {
		t.Errorf("exactly the maximum must be allowed: got %s, %v", got, err)
	}

	if _, err := resolveExecTimeout(301, maxTimeout); err == nil || !strings.Contains(err.Error(), ExecRequestConfigKeyMaxTimeout) {
		t.Errorf("over the maximum must be refused and name the setting: %v", err)
	}
	if _, err := resolveExecTimeout(-1, maxTimeout); err == nil {
		t.Error("a negative timeout was accepted")
	}

	// No bound configured: anything goes.
	if got, err := resolveExecTimeout(3600, 0); err != nil || got != time.Hour {
		t.Errorf("unbounded: got %s, %v", got, err)
	}
}

// From a debug container the target's files live under /proc/1/root; a cwd
// meant for the target must be translated there and must not be able to
// climb out into the debug image's own filesystem.
func TestDebugContainerCwd(t *testing.T) {
	cases := map[string]string{
		"":                  "/proc/1/root",
		"   ":               "/proc/1/root",
		"/":                 "/proc/1/root",
		"/app":              "/proc/1/root/app",
		"/app/":             "/proc/1/root/app",
		"app":               "/proc/1/root/app",
		"/app/../etc":       "/proc/1/root/etc",
		"../../..":          "/proc/1/root",
		"/../../../etc":     "/proc/1/root/etc",
		"/with space/dir":   "/proc/1/root/with space/dir",
		"/proc/1/root/keep": "/proc/1/root/proc/1/root/keep",
	}
	for in, want := range cases {
		if got := debugContainerCwd(in); got != want {
			t.Errorf("debugContainerCwd(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveExecContainer(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "ns"},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{Name: "app"}, {Name: "sidecar"}},
			EphemeralContainers: []v1.EphemeralContainer{{
				EphemeralContainerCommon: v1.EphemeralContainerCommon{Name: "mogenius-debug"},
				TargetContainerName:      "app",
			}},
		},
	}
	client := fake.NewClientset(pod)
	ctx := context.Background()

	if got, err := resolveExecContainer(ctx, client, "ns", "web", ""); err != nil || got != "app" {
		t.Errorf("empty request: got %q, %v; want the first container", got, err)
	}
	if got, err := resolveExecContainer(ctx, client, "ns", "web", "sidecar"); err != nil || got != "sidecar" {
		t.Errorf("named container: got %q, %v", got, err)
	}
	if got, err := resolveExecContainer(ctx, client, "ns", "web", "mogenius-debug"); err != nil || got != "mogenius-debug" {
		t.Errorf("an attached debug container is a valid explicit target: got %q, %v", got, err)
	}
	if _, err := resolveExecContainer(ctx, client, "ns", "web", "nope"); err == nil || !strings.Contains(err.Error(), "app, sidecar") {
		t.Errorf("unknown container must be refused and list the available ones: %v", err)
	}
	if _, err := resolveExecContainer(ctx, client, "ns", "missing", ""); err == nil {
		t.Error("a missing pod was not reported")
	}
}

func TestExecuteCommandRejectsBadInputBeforeTouchingTheCluster(t *testing.T) {
	// clientProvider is nil in this test binary; any of these reaching the
	// cluster would fail with a different error or panic.
	if _, err := ExecuteCommand(ExecRequest{Namespace: "ns", Pod: "p", Command: "   "}); err == nil {
		t.Error("empty command accepted")
	}
	if _, err := ExecuteCommand(ExecRequest{Namespace: "ns", Pod: "p", Command: "true", Env: map[string]string{"A=B": "x"}}); err == nil {
		t.Error("invalid env key accepted")
	}
	if _, err := ExecuteCommand(ExecRequest{Namespace: "ns", Pod: "p", Command: "true", TimeoutSeconds: -5}); err == nil {
		t.Error("negative timeout accepted")
	}
}

func TestExecAuditSummaryDropsOutput(t *testing.T) {
	response := ExecResponse{ExitCode: 3, Stdout: strings.Repeat("x", 1024), Stderr: "err", Truncated: true, Container: "app", DurationMs: 42}
	summary := response.AuditSummary()
	if summary.ExitCode != 3 || summary.Container != "app" || !summary.Truncated || summary.DurationMs != 42 {
		t.Errorf("summary lost fields: %+v", summary)
	}
}
