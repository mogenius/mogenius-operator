package services

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"mogenius-operator/src/debugcontainer"
	"mogenius-operator/src/k8sexec"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// ExecRequestConfigKeyMaxTimeout bounds the timeout a request may ask for.
	ExecRequestConfigKeyMaxTimeout = "MO_EXEC_REQUEST_MAX_TIMEOUT_SECONDS"
	// ExecRequestConfigKeyMaxOutput caps each output stream of a request.
	ExecRequestConfigKeyMaxOutput = "MO_EXEC_REQUEST_MAX_OUTPUT_BYTES"
	// execAdminBypassConfigKey is shared with the SSH gateway on purpose: a
	// one-off command and a shell into the same pod follow the same rule.
	execAdminBypassConfigKey = "MO_SSH_GATEWAY_ALLOW_ADMIN_BYPASS"

	execRequestDefaultMaxTimeout = 300 * time.Second
	execRequestDefaultMaxOutput  = 1 << 20

	// execDebugContainerTimeout bounds attaching and starting the ephemeral
	// debug container for a shell-less image. It is separate from the
	// command's own timeout: pulling an image is not the command's time.
	execDebugContainerTimeout = 90 * time.Second

	// debugContainerRootPath is where the target container's filesystem is
	// visible from a debug container that shares its PID namespace.
	debugContainerRootPath = "/proc/1/root"
)

// ExecRequest is the payload of `service/exec-request`: run one command line
// in a pod's container and return its outcome. It mirrors what Daytona's
// executeCommand accepts so the platform can pass the SDK's parameters on
// unchanged.
type ExecRequest struct {
	Namespace string `json:"namespace" validate:"required"`
	Pod       string `json:"pod" validate:"required"`
	// Container is optional; the pod's first container is used when empty.
	Container string `json:"container"`
	// Command is a shell command line: pipes, && and quoting work as they do
	// in a shell of the image (bash, sh or ash, whichever exists).
	Command string `json:"command" validate:"required"`
	// Cwd is the working directory to run in; empty keeps the container's.
	Cwd string `json:"cwd"`
	// Env is exported into the command's environment. Keys must be valid
	// shell identifiers.
	Env map[string]string `json:"env"`
	// TimeoutSeconds bounds the run; 0 means the default of ten seconds.
	// Requests above MO_EXEC_REQUEST_MAX_TIMEOUT_SECONDS are rejected.
	TimeoutSeconds int `json:"timeoutSeconds"`

	// IsAdmin and IsClusterAdmin are the platform's judgement that the
	// requester may act as the operator instead of being impersonated. They
	// arrive in the same frame as the request over the authenticated control
	// connection and are exactly as trustworthy as that connection; see
	// k8sexec.Identity and MO_SSH_GATEWAY_ALLOW_ADMIN_BYPASS.
	IsAdmin        bool `json:"isAdmin"`
	IsClusterAdmin bool `json:"isClusterAdmin"`

	// UserEmail is taken from the datagram's user field by the handler, not
	// from the payload, so every handler sees the same identity.
	UserEmail string `json:"-"`
}

// ExecResponse is the outcome of a finished command. A non-zero ExitCode is
// a successful response; the pattern only fails when the command could not
// be run at all or ran out of time.
type ExecResponse struct {
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	// Truncated is set when either stream exceeded MO_EXEC_REQUEST_MAX_OUTPUT_BYTES.
	Truncated bool `json:"truncated"`
	// Container is where the command actually ran: the requested (or first)
	// container, or the debug container attached to it when the image ships
	// no shell.
	Container  string `json:"container"`
	DurationMs int64  `json:"durationMs"`
}

// ExecAuditSummary is what the audit log keeps of a run. The command itself
// is in the payload already; the output is not recorded, since each stream
// may be up to the configured cap.
type ExecAuditSummary struct {
	Container  string `json:"container"`
	ExitCode   int    `json:"exitCode"`
	Truncated  bool   `json:"truncated"`
	DurationMs int64  `json:"durationMs"`
}

// AuditSummary reduces the response to what belongs in the audit log.
func (r ExecResponse) AuditSummary() ExecAuditSummary {
	return ExecAuditSummary{Container: r.Container, ExitCode: r.ExitCode, Truncated: r.Truncated, DurationMs: r.DurationMs}
}

// ExecuteCommand runs request.Command in the target container under the
// requester's Kubernetes identity and waits for the result. Images without a
// shell are served through an ephemeral debug container that shares the
// target's PID namespace; the working directory is then resolved under
// /proc/1/root so paths still refer to the target's filesystem.
func ExecuteCommand(request ExecRequest) (ExecResponse, error) {
	if strings.TrimSpace(request.Command) == "" {
		return ExecResponse{}, errors.New("exec: command must not be empty")
	}
	if err := k8sexec.ValidateEnv(request.Env); err != nil {
		return ExecResponse{}, fmt.Errorf("exec: %w", err)
	}
	maxTimeout, maxOutput := execRequestLimits()
	timeout, err := resolveExecTimeout(request.TimeoutSeconds, maxTimeout)
	if err != nil {
		return ExecResponse{}, fmt.Errorf("exec: %w", err)
	}

	allowAdminBypass, _ := config.TryGetBool(execAdminBypassConfigKey)
	clients, err := k8sexec.ResolveClients(serviceLogger, clientProvider, config.Get("MO_OWN_NAMESPACE"), allowAdminBypass, k8sexec.Identity{
		Email:   request.UserEmail,
		IsAdmin: request.IsAdmin || request.IsClusterAdmin,
	})
	if err != nil {
		return ExecResponse{}, fmt.Errorf("exec: %w", err)
	}

	ctx := context.Background()
	// Deliberately the requester's client: reading the pod is itself a read
	// the user must be allowed, and it settles which container to use.
	container, err := resolveExecContainer(ctx, clients.Clientset, request.Namespace, request.Pod, request.Container)
	if err != nil {
		return ExecResponse{}, fmt.Errorf("exec: %w", err)
	}
	shell, execContainer, cwd, err := prepareExecTarget(ctx, clients, request.Namespace, request.Pod, container, request.Cwd)
	if err != nil {
		return ExecResponse{}, fmt.Errorf("exec: %w", err)
	}

	argv := k8sexec.BuildShellCommand(shell, request.Command, cwd, request.Env, timeout)
	runCtx, cancel := context.WithTimeout(ctx, timeout+k8sexec.RunGrace)
	defer cancel()

	start := time.Now()
	result, runErr := k8sexec.Run(runCtx, clients, k8sexec.RunRequest{
		Namespace:      request.Namespace,
		Pod:            request.Pod,
		Container:      execContainer,
		Command:        argv,
		MaxOutputBytes: maxOutput,
	})
	elapsed := time.Since(start)

	if k8sexec.TimedOut(runErr, result.ExitCode, elapsed, timeout) {
		serviceLogger.Info("exec-request timed out", "namespace", request.Namespace, "pod", request.Pod, "container", execContainer, "timeout", timeout)
		return ExecResponse{}, fmt.Errorf("exec: command timed out after %s: %w", timeout, context.DeadlineExceeded)
	}
	if runErr != nil {
		return ExecResponse{}, fmt.Errorf("exec in %s/%s (%s): %w", request.Namespace, request.Pod, execContainer, runErr)
	}

	return ExecResponse{
		ExitCode:   result.ExitCode,
		Stdout:     result.Stdout,
		Stderr:     result.Stderr,
		Truncated:  result.Truncated,
		Container:  execContainer,
		DurationMs: elapsed.Milliseconds(),
	}, nil
}

// execRequestLimits reads the configured bounds, falling back to the
// defaults when a key is unset or unusable.
func execRequestLimits() (maxTimeout time.Duration, maxOutput int) {
	maxTimeout = execRequestDefaultMaxTimeout
	maxOutput = execRequestDefaultMaxOutput
	if config == nil {
		return maxTimeout, maxOutput
	}
	if seconds, err := config.TryGetInt(ExecRequestConfigKeyMaxTimeout); err == nil && seconds > 0 {
		maxTimeout = time.Duration(seconds) * time.Second
	}
	if bytes, err := config.TryGetInt(ExecRequestConfigKeyMaxOutput); err == nil && bytes > 0 {
		maxOutput = int(bytes)
	}
	return maxTimeout, maxOutput
}

// resolveExecTimeout applies the default and the upper bound. A request over
// the bound is refused rather than silently shortened: a client that asked
// for five minutes and got one would misread every long command as slow.
func resolveExecTimeout(requestedSeconds int, maxTimeout time.Duration) (time.Duration, error) {
	if requestedSeconds < 0 {
		return 0, fmt.Errorf("timeoutSeconds must not be negative, got %d", requestedSeconds)
	}
	if requestedSeconds == 0 {
		return k8sexec.DefaultRunTimeout, nil
	}
	timeout := time.Duration(requestedSeconds) * time.Second
	if maxTimeout > 0 && timeout > maxTimeout {
		return 0, fmt.Errorf("timeoutSeconds %d exceeds the cluster's maximum of %d (%s)", requestedSeconds, int(maxTimeout.Seconds()), ExecRequestConfigKeyMaxTimeout)
	}
	return timeout, nil
}

// resolveExecContainer settles the target container: the requested one if
// the pod has it, otherwise the pod's first container. Debug containers the
// operator attached earlier are accepted as explicit targets too.
func resolveExecContainer(ctx context.Context, client kubernetes.Interface, namespace, podName, requested string) (string, error) {
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("read pod %s/%s: %w", namespace, podName, err)
	}
	names := make([]string, 0, len(pod.Spec.Containers)+len(pod.Spec.EphemeralContainers))
	for _, c := range pod.Spec.Containers {
		names = append(names, c.Name)
	}
	if len(names) == 0 {
		return "", fmt.Errorf("pod %s/%s has no containers", namespace, podName)
	}
	if requested == "" {
		return names[0], nil
	}
	for _, name := range names {
		if name == requested {
			return name, nil
		}
	}
	for _, c := range pod.Spec.EphemeralContainers {
		if c.Name == requested {
			return c.Name, nil
		}
	}
	return "", fmt.Errorf("pod %s/%s has no container %q (available: %s)", namespace, podName, requested, strings.Join(names, ", "))
}

// prepareExecTarget finds a shell to run the command with. When the image
// has none, it attaches (or reuses) a debug container targeting the
// container and answers with that container's shell and the working
// directory translated to the target's filesystem.
func prepareExecTarget(ctx context.Context, clients *k8sexec.Clients, namespace, podName, container, cwd string) (shell, execContainer, execCwd string, err error) {
	shell, shellErr := detectExecShell(clients, namespace, podName, container)
	if shellErr == nil {
		return shell, container, cwd, nil
	}
	if debugcontainer.IsDebugContainer(container) {
		// Already the fallback; there is nothing further to attach to.
		return "", "", "", shellErr
	}

	serviceLogger.Info("no shell in container; running the command in a debug container",
		"namespace", namespace, "pod", podName, "container", container)
	ensureCtx, cancel := context.WithTimeout(ctx, execDebugContainerTimeout)
	debugName, debugErr := debugcontainer.Ensure(ensureCtx, clients.Clientset, namespace, podName, debugcontainer.Options{
		Image:           debugcontainer.ImageFromConfig(config),
		TargetContainer: container,
		Timeout:         execDebugContainerTimeout,
	})
	cancel()
	if debugErr != nil {
		return "", "", "", fmt.Errorf("%v; %w", shellErr, debugErr)
	}

	shell, err = detectExecShell(clients, namespace, podName, debugName)
	if err != nil {
		return "", "", "", fmt.Errorf("debug container %q: %w", debugName, err)
	}
	return shell, debugName, debugContainerCwd(cwd), nil
}

// detectExecShell asks the shared prober which shell the container ships, so
// exec-request, the web terminal and the SSH gateway agree on the answer.
func detectExecShell(clients *k8sexec.Clients, namespace, podName, container string) (string, error) {
	executor, err := k8sexec.NewExecutor(serviceLogger, clients.Clientset.CoreV1().RESTClient(), *clients.RestConfig, namespace, podName, container)
	if err != nil {
		return "", err
	}
	return k8sexec.DetectShell(executor, serviceLogger)
}

// debugContainerCwd maps a working directory meant for the target container
// onto the debug container, where the target's root is /proc/1/root. The
// path is anchored first so ".." cannot climb out of that root: a relative
// or escaping cwd would otherwise land in the debug image's own filesystem
// and the command would silently operate on the wrong files.
func debugContainerCwd(cwd string) string {
	if strings.TrimSpace(cwd) == "" {
		return debugContainerRootPath
	}
	return path.Join(debugContainerRootPath, path.Clean("/"+cwd))
}
