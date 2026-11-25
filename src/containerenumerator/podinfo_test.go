package containerenumerator_test

import (
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/containerenumerator"
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The pod sandbox (pause) and containers of other pods show up in the cgroup
// scan, but only the reported pod containers may be read by the cpu and ram
// monitors.
func TestPodProcessesSkipsSandboxAndUnreportedContainers(t *testing.T) {
	const app = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const sandbox = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	const other = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

	now := metav1.Now()
	pod := v1.Pod{
		Namespace: "default", Name: "web",
		Status: v1.PodStatus{
			StartTime: &now,
			ContainerStatuses: []v1.ContainerStatus{
				{Name: "web", Ready: true, ContainerID: "containerd://" + app},
			},
		},
	}
	nodeContainers := map[containerenumerator.ContainerId][]containerenumerator.ProcessId{
		app:     {100, 101},
		sandbox: {99},
		other:   {200},
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	pods := containerenumerator.NewPodInfoList(logger, []v1.Pod{pod}, nodeContainers)
	got := containerenumerator.PodProcesses(pods)

	assert.AssertT(t, len(got) == 1, got)
	assert.AssertT(t, len(got[app]) == 2, got)
	_, hasSandbox := got[sandbox]
	assert.AssertT(t, !hasSandbox, got)
}
