package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestPodNotReadyReason_UnschedulablePortConflict(t *testing.T) {
	// The exact MOG scenario: node-exporter pod pending because hostPort 9100
	// is taken on the only eligible node.
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{
					Type:    corev1.PodScheduled,
					Status:  corev1.ConditionFalse,
					Reason:  "Unschedulable",
					Message: `0/2 nodes are available: 1 node(s) didn't have free ports for the requested pod ports.`,
				},
			},
		},
	}
	reason := podNotReadyReason(pod, "")
	assert.True(t, strings.HasPrefix(reason, "Pending - Unschedulable:"), "got %q", reason)
	assert.Contains(t, reason, "free ports")
}

func TestPodNotReadyReason_ReadyRunningSkipped(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			Phase:             corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{Name: "c", Ready: true}},
		},
	}
	assert.Equal(t, "", podNotReadyReason(pod, ""))
}

func TestPodNotReadyReason_SucceededSkipped(t *testing.T) {
	pod := &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodSucceeded}}
	assert.Equal(t, "", podNotReadyReason(pod, ""))
}

func TestPodNotReadyReason_CrashLoopBackOff(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "app",
				Ready: false,
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
					Reason:  "CrashLoopBackOff",
					Message: "back-off restarting failed container",
				}},
			}},
		},
	}
	reason := podNotReadyReason(pod, "")
	assert.Contains(t, reason, "CrashLoopBackOff")
	assert.Contains(t, reason, "(app)")
}

func TestPodNotReadyReason_PendingFallsBackToWarningEvent(t *testing.T) {
	pod := &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending}}
	reason := podNotReadyReason(pod, "FailedScheduling: insufficient memory")
	assert.Equal(t, "Pending - FailedScheduling: insufficient memory", reason)
}

func TestPodNotReadyReason_Failed(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			Phase:   corev1.PodFailed,
			Reason:  "Evicted",
			Message: "node out of memory",
		},
	}
	reason := podNotReadyReason(pod, "")
	assert.Contains(t, reason, "Failed - Evicted")
	assert.Contains(t, reason, "node out of memory")
}
