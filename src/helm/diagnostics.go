package helm

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"helm.sh/helm/v4/pkg/cli"
)

// maxDiagnosticPods caps how many not-ready pods we report so a broadly broken
// install (e.g. a namespace full of pending pods) cannot flood the log stream.
const maxDiagnosticPods = 15

// logReleaseFailureDiagnostics inspects the release namespace after a failed
// install/upgrade and writes the underlying reason of every not-ready pod into
// the release log (logs:helm, keyed by namespace+release - i.e. the very log
// the UI shows for the install).
//
// Helm's wait error only says e.g. "DaemonSet ... not ready: Available 1/2 /
// context deadline exceeded" without WHY. The actionable reason (Unschedulable:
// no free ports, ImagePullBackOff, FailedMount, ...) lives on the pods'
// PodScheduled condition, their container states, and Warning events - this
// surfaces all three so the user no longer needs kubectl. Best-effort: any
// error here is logged at debug and never masks the original install error.
func logReleaseFailureDiagnostics(settings *cli.EnvSettings, namespace, release string) {
	if settings == nil || namespace == "" {
		return
	}

	restConfig, err := settings.RESTClientGetter().ToRESTConfig()
	if err != nil {
		helmLogger.Debug("release diagnostics: failed to build REST config", "releaseName", release, "namespace", namespace, "error", err.Error())
		return
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		helmLogger.Debug("release diagnostics: failed to build clientset", "releaseName", release, "namespace", namespace, "error", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		helmLogger.Debug("release diagnostics: failed to list pods", "releaseName", release, "namespace", namespace, "error", err.Error())
		return
	}

	warnings := latestWarningByObject(ctx, clientset, namespace)

	reported := 0
	for i := range pods.Items {
		pod := &pods.Items[i]
		reason := podNotReadyReason(pod, warnings[pod.Name])
		if reason == "" {
			continue
		}
		if reported >= maxDiagnosticPods {
			helmLogger.Warn("release diagnostics: more not-ready pods omitted",
				"releaseName", release, "namespace", namespace, "shown", reported)
			break
		}
		reported++
		helmLogger.Warn(fmt.Sprintf("install incomplete: pod %q is %s", pod.Name, reason),
			"releaseName", release, "namespace", namespace, "pod", pod.Name)
	}

	if reported == 0 {
		helmLogger.Warn("install did not complete, but all pods in the namespace look ready - the stuck resource may be a Job, PVC or custom resource; check `kubectl get events`",
			"releaseName", release, "namespace", namespace)
	}
}

// podNotReadyReason returns a human-readable reason why the pod is not ready, or
// "" when the pod is ready/succeeded and should not be reported.
func podNotReadyReason(pod *corev1.Pod, latestWarning string) string {
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return ""
	case corev1.PodPending:
		// Scheduling failures (the common case) carry their reason on the
		// PodScheduled=False condition - exactly the "Unschedulable: no free
		// ports" message that was previously invisible.
		if msg := unschedulableMessage(pod); msg != "" {
			return "Pending - " + msg
		}
		if cs := waitingContainerReason(pod); cs != "" {
			return "Pending - " + cs
		}
		if latestWarning != "" {
			return "Pending - " + latestWarning
		}
		return "Pending"
	case corev1.PodFailed:
		if pod.Status.Reason != "" || pod.Status.Message != "" {
			return strings.TrimSpace(fmt.Sprintf("Failed - %s %s", pod.Status.Reason, pod.Status.Message))
		}
		return "Failed"
	default: // Running / Unknown
		if !allContainersReady(pod) {
			if cs := waitingContainerReason(pod); cs != "" {
				return fmt.Sprintf("%s - %s", pod.Status.Phase, cs)
			}
			if latestWarning != "" {
				return fmt.Sprintf("%s - %s", pod.Status.Phase, latestWarning)
			}
			return fmt.Sprintf("%s - containers not ready", pod.Status.Phase)
		}
		return ""
	}
}

// unschedulableMessage returns the PodScheduled=False message when the pod could
// not be scheduled, else "".
func unschedulableMessage(pod *corev1.Pod) string {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse {
			return strings.TrimSpace(fmt.Sprintf("%s: %s", c.Reason, c.Message))
		}
	}
	return ""
}

// waitingContainerReason returns the first waiting/terminated container reason
// (e.g. ImagePullBackOff, CrashLoopBackOff, CreateContainerError) across init
// and regular containers, or "".
func waitingContainerReason(pod *corev1.Pod) string {
	statuses := append([]corev1.ContainerStatus{}, pod.Status.InitContainerStatuses...)
	statuses = append(statuses, pod.Status.ContainerStatuses...)
	for _, cs := range statuses {
		if cs.Ready {
			continue
		}
		if w := cs.State.Waiting; w != nil && w.Reason != "" {
			return strings.TrimSpace(fmt.Sprintf("%s: %s (%s)", w.Reason, w.Message, cs.Name))
		}
		if t := cs.State.Terminated; t != nil && t.Reason != "" && t.Reason != "Completed" {
			return strings.TrimSpace(fmt.Sprintf("%s: %s (%s)", t.Reason, t.Message, cs.Name))
		}
	}
	return ""
}

func allContainersReady(pod *corev1.Pod) bool {
	for _, cs := range pod.Status.ContainerStatuses {
		if !cs.Ready {
			return false
		}
	}
	return true
}

// latestWarningByObject returns the most recent Warning event message per
// involved object name in the namespace, used as a fallback reason when a pod's
// own status does not explain the problem (e.g. FailedMount, FailedScheduling).
func latestWarningByObject(ctx context.Context, clientset kubernetes.Interface, namespace string) map[string]string {
	out := map[string]string{}
	events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return out
	}
	type stamped struct {
		when    time.Time
		message string
	}
	latest := map[string]stamped{}
	for i := range events.Items {
		e := &events.Items[i]
		if e.Type != corev1.EventTypeWarning {
			continue
		}
		when := e.LastTimestamp.Time
		if when.IsZero() {
			when = e.EventTime.Time
		}
		name := e.InvolvedObject.Name
		if prev, ok := latest[name]; !ok || when.After(prev.when) {
			latest[name] = stamped{when: when, message: strings.TrimSpace(fmt.Sprintf("%s: %s", e.Reason, e.Message))}
		}
	}
	for name, s := range latest {
		out[name] = s.message
	}
	return out
}
