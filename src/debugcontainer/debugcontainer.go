// Package debugcontainer attaches an operator-managed ephemeral container to a
// running pod so that a shell exists where the workload image ships none.
//
// The Kubernetes exec API always starts a binary from the target container's
// own filesystem; a distroless or scratch image therefore has nothing to
// start. `kubectl debug` solves this by adding an ephemeral container with a
// tool image into the already running pod sandbox — no restart, no
// reschedule, the customer's containers untouched. This package does the same
// for the web terminal and the SSH gateway, which then exec into the debug
// container through the exact code path they already use.
//
// Ephemeral containers can neither be removed nor restarted once added, so a
// running one is always reused and names are numbered when an earlier one has
// terminated — otherwise every terminal click would grow the pod spec.
package debugcontainer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	cfg "mogenius-operator/src/config"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// NamePrefix names the debug containers this package adds. The first one
	// is exactly NamePrefix, later ones NamePrefix-2, -3, … A username equal to
	// NamePrefix on the SSH gateway selects (and, if needed, creates) it.
	NamePrefix = "mogenius-debug"

	// ConfigKeyImage is the operator config key for the debug image
	// (declared in cmd.go, fed by Helm from features.debugTools.image).
	ConfigKeyImage = "MO_DEBUG_CONTAINER_IMAGE"

	// DefaultImage is used when the config key is unset. Matches the Helm
	// chart's features.debugTools.image default.
	DefaultImage = "docker.io/nicolaka/netshoot:v0.16" // renovate: datasource=docker depName=nicolaka/netshoot

	defaultTimeout = 60 * time.Second
	pollInterval   = 500 * time.Millisecond
)

// Options tune a single Ensure call.
type Options struct {
	// Image of the debug container. Empty falls back to DefaultImage.
	Image string
	// TargetContainer is the workload container whose PID namespace the debug
	// container joins (targetContainerName). Required: without it the debug
	// shell can see neither the workload's processes nor, via /proc/<pid>/root,
	// its filesystem.
	TargetContainer string
	// Progress receives human-readable progress lines while the container is
	// being attached and started. Lines end in "\r\n" so they render correctly
	// on a raw terminal. May be nil.
	Progress io.Writer
	// Timeout bounds the wait for the container to reach Running. Zero means
	// defaultTimeout.
	Timeout time.Duration
	// pollInterval is overridable for tests.
	pollInterval time.Duration
}

// ImageFromConfig resolves the debug image from the operator config, falling
// back to DefaultImage when the key is unset or empty.
func ImageFromConfig(config cfg.ConfigModule) string {
	if config != nil {
		if value, err := config.TryGet(ConfigKeyImage); err == nil && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return DefaultImage
}

// IsDebugContainer reports whether name is one of ours.
func IsDebugContainer(name string) bool {
	return name == NamePrefix || strings.HasPrefix(name, NamePrefix+"-")
}

// Ensure returns the name of a running debug container targeting
// opts.TargetContainer in the pod, attaching and starting a new one when none
// is running. The returned name is what callers pass as the exec container.
//
// Errors are worded for the person sitting at the terminal: RBAC and admission
// rejections (Pod Security "restricted", Kyverno/Gatekeeper) and image pull
// failures name their cause instead of surfacing the raw API error alone.
func Ensure(ctx context.Context, client kubernetes.Interface, namespace, podName string, opts Options) (string, error) {
	if client == nil {
		return "", errors.New("debug container: no kubernetes client")
	}
	if strings.TrimSpace(opts.TargetContainer) == "" {
		return "", errors.New("debug container: target container is required")
	}
	image := strings.TrimSpace(opts.Image)
	if image == "" {
		image = DefaultImage
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	interval := opts.pollInterval
	if interval <= 0 {
		interval = pollInterval
	}

	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("debug container: cannot read pod %s/%s: %w", namespace, podName, err)
	}

	if name := runningDebugContainer(pod, opts.TargetContainer); name != "" {
		progress(opts.Progress, "mogenius: reusing running debug container %q", name)
		return name, nil
	}

	name := nextName(pod)
	progress(opts.Progress, "mogenius: no shell in container %q — attaching debug container %q (%s)", opts.TargetContainer, name, image)

	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, buildSpec(name, image, opts.TargetContainer))
	if _, err := client.CoreV1().Pods(namespace).UpdateEphemeralContainers(ctx, podName, pod, metav1.UpdateOptions{}); err != nil {
		return "", describeAttachError(err, image)
	}

	return waitRunning(ctx, client, namespace, podName, name, timeout, interval, opts.Progress)
}

// runningDebugContainer returns the name of a running debug container that
// targets the given workload container, or "" when there is none. A debug
// container targeting another container is not reused: it lives in a
// different PID namespace and would show the wrong processes and filesystem.
func runningDebugContainer(pod *v1.Pod, target string) string {
	targets := make(map[string]string, len(pod.Spec.EphemeralContainers))
	for _, ec := range pod.Spec.EphemeralContainers {
		targets[ec.Name] = ec.TargetContainerName
	}
	for _, status := range pod.Status.EphemeralContainerStatuses {
		if !IsDebugContainer(status.Name) || status.State.Running == nil {
			continue
		}
		if targets[status.Name] == target {
			return status.Name
		}
	}
	return ""
}

// nextName picks the first unused name in the NamePrefix, NamePrefix-2, …
// sequence. Names are checked against the spec, not the status: an entry that
// has been added but not yet reported is still taken.
func nextName(pod *v1.Pod) string {
	taken := make(map[string]struct{}, len(pod.Spec.EphemeralContainers)+len(pod.Spec.Containers))
	for _, ec := range pod.Spec.EphemeralContainers {
		taken[ec.Name] = struct{}{}
	}
	for _, c := range pod.Spec.Containers {
		taken[c.Name] = struct{}{}
	}
	for _, c := range pod.Spec.InitContainers {
		taken[c.Name] = struct{}{}
	}
	if _, used := taken[NamePrefix]; !used {
		return NamePrefix
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", NamePrefix, i)
		if _, used := taken[candidate]; !used {
			return candidate
		}
	}
}

// buildSpec is the ephemeral container we attach.
//
// It idles in a sleep loop instead of relying on the image's entrypoint, so
// the exec that follows has a running container regardless of what the image
// would do on its own. It runs unprivileged and cannot escalate, but keeps
// the default capability set and the image's user: visibility into the
// target's filesystem via /proc/<pid>/root needs the same uid or ptrace
// rights, and that is the whole point of the container. Namespaces enforcing
// Pod Security "restricted" reject this spec — the error then names the
// policy, see describeAttachError.
//
// The API forbids resources, ports, probes and lifecycle on ephemeral
// containers; they share the pod's headroom.
func buildSpec(name, image, target string) v1.EphemeralContainer {
	return v1.EphemeralContainer{
		EphemeralContainerCommon: v1.EphemeralContainerCommon{
			Name:            name,
			Image:           image,
			ImagePullPolicy: v1.PullIfNotPresent,
			Command:         []string{"sh", "-c", "trap 'exit 0' TERM; while true; do sleep 3600 & wait $!; done"},
			Stdin:           true,
			TTY:             true,
			SecurityContext: &v1.SecurityContext{
				Privileged:               new(false),
				AllowPrivilegeEscalation: new(false),
				SeccompProfile:           &v1.SeccompProfile{Type: v1.SeccompProfileTypeRuntimeDefault},
			},
			TerminationMessagePolicy: v1.TerminationMessageReadFile,
		},
		TargetContainerName: target,
	}
}

// describeAttachError turns the API error of UpdateEphemeralContainers into a
// message that says what to do about it.
func describeAttachError(err error, image string) error {
	switch {
	case apierrors.IsForbidden(err):
		msg := err.Error()
		if strings.Contains(msg, "violates PodSecurity") || strings.Contains(msg, "pod security") {
			return fmt.Errorf("debug container: rejected by Pod Security Admission — the namespace enforces a profile that forbids the debug container (%v)", trimAPIError(err))
		}
		return fmt.Errorf("debug container: forbidden — the operator or your identity lacks update on pods/ephemeralcontainers, or an admission policy rejected it (%v)", trimAPIError(err))
	case apierrors.IsInvalid(err):
		return fmt.Errorf("debug container: the cluster rejected the container spec (image %s): %v", image, trimAPIError(err))
	case apierrors.IsNotFound(err):
		return fmt.Errorf("debug container: the pod disappeared before the container could be attached: %v", trimAPIError(err))
	default:
		return fmt.Errorf("debug container: attaching failed: %v", trimAPIError(err))
	}
}

// waitRunning polls the pod until the debug container runs, fails early on
// pull and create errors, and reports the last known state on timeout.
func waitRunning(ctx context.Context, client kubernetes.Interface, namespace, podName, name string, timeout, interval time.Duration, out io.Writer) (string, error) {
	deadline := time.Now().Add(timeout)
	progress(out, "mogenius: waiting for %q to start", name)
	lastState := "not reported yet"
	for {
		pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("debug container: cannot read pod %s/%s while waiting for %q: %w", namespace, podName, name, err)
		}
		for _, status := range pod.Status.EphemeralContainerStatuses {
			if status.Name != name {
				continue
			}
			switch {
			case status.State.Running != nil:
				progress(out, "mogenius: debug container %q is running", name)
				return name, nil
			case status.State.Terminated != nil:
				t := status.State.Terminated
				return "", fmt.Errorf("debug container: %q exited (reason %s, code %d) %s", name, t.Reason, t.ExitCode, strings.TrimSpace(t.Message))
			case status.State.Waiting != nil:
				w := status.State.Waiting
				lastState = w.Reason
				if isFatalWaiting(w.Reason) {
					return "", describeWaitingError(name, w)
				}
			}
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("debug container: %q did not start within %s (last state: %s)", name, timeout, lastState)
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("debug container: cancelled while waiting for %q: %w", name, ctx.Err())
		case <-time.After(interval):
		}
	}
}

// isFatalWaiting lists kubelet waiting reasons that do not resolve by waiting.
func isFatalWaiting(reason string) bool {
	switch reason {
	case "ErrImagePull", "ImagePullBackOff", "InvalidImageName", "CreateContainerConfigError", "CreateContainerError", "RunContainerError":
		return true
	}
	return false
}

func describeWaitingError(name string, w *v1.ContainerStateWaiting) error {
	switch w.Reason {
	case "ErrImagePull", "ImagePullBackOff", "InvalidImageName":
		return fmt.Errorf("debug container: the cluster cannot pull the debug image (%s) — mirror it into a reachable registry and set %s: %s", w.Reason, ConfigKeyImage, strings.TrimSpace(w.Message))
	default:
		return fmt.Errorf("debug container: %q failed to start (%s): %s", name, w.Reason, strings.TrimSpace(w.Message))
	}
}

// trimAPIError drops the client-go request prefix so messages stay readable.
func trimAPIError(err error) string {
	msg := err.Error()
	if idx := strings.Index(msg, ": "); idx >= 0 && strings.HasPrefix(msg, "pods ") {
		msg = msg[idx+2:]
	}
	return strings.TrimSpace(msg)
}

func progress(out io.Writer, format string, args ...any) {
	if out == nil {
		return
	}
	_, _ = fmt.Fprintf(out, format+"\r\n", args...)
}
