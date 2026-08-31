package sshgateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"mogenius-operator/src/k8sexec"

	"golang.org/x/crypto/ssh"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"
)

// SSH wire-format payloads (RFC 4254).
type ptyReqMsg struct {
	Term          string
	Cols, Rows    uint32
	Width, Height uint32
	Modes         string
}

type winChMsg struct {
	Cols, Rows    uint32
	Width, Height uint32
}

type execMsg struct {
	Command string
}

type subsystemMsg struct {
	Name string
}

type exitStatusMsg struct {
	Status uint32
}

// resolveContainer picks the target container: an SSH username matching a
// container name selects it; anything else falls back to the first container.
// The full container list is returned so sessions can hint at alternatives.
func resolveContainer(clients *execClients, namespace string, podName string, user string) (string, []string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pod, err := clients.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", nil, fmt.Errorf("pod %q not found: %w", podName, err)
	}
	if len(pod.Spec.Containers) == 0 {
		return "", nil, fmt.Errorf("pod %q has no containers", podName)
	}
	available := make([]string, 0, len(pod.Spec.Containers))
	for _, c := range pod.Spec.Containers {
		available = append(available, c.Name)
	}
	for _, c := range pod.Spec.Containers {
		if c.Name == user {
			return c.Name, available, nil
		}
	}
	return pod.Spec.Containers[0].Name, available, nil
}

// handleSession serves one SSH session channel: pty-req/env/window-change
// bookkeeping, plus the shell, exec or sftp payload the client asks for.
//
// The payload runs in its own goroutine and this loop keeps draining
// `requests` for the session's lifetime. That is not optional: x/crypto/ssh
// feeds the channel from the connection's single mux goroutine with an
// unguarded blocking send into a fixed-size buffer, so a loop that stops
// reading — as it would if the exec ran inline — deadlocks the entire SSH
// connection once the buffer fills. A client resizing its terminal reaches
// that in a few drags, and because the transport underneath is an unbuffered
// net.Pipe the stall propagates into the shared tunnel read loop and freezes
// every other sub-connection with it. Draining is also what makes terminal
// resize work at all: window-change arrives while the exec is running.
func handleSession(logger *slog.Logger, clients *execClients, channel ssh.Channel, requests <-chan *ssh.Request, namespace, podName, container string, containerNote string) {
	defer func() { _ = channel.Close() }()

	var (
		hasPty    bool
		sizeQueue = newSizeQueue()
		started   bool
	)
	defer sizeQueue.close()

	// Buffered so the payload goroutine never blocks on reporting its exit
	// code, even when this loop has already returned.
	finished := make(chan int, 1)

	for {
		select {
		case code := <-finished:
			sendExitStatus(channel, code)
			return

		case req, ok := <-requests:
			if !ok {
				// Client closed the channel; the deferred Close tears down a
				// payload that may still be streaming.
				return
			}

			switch req.Type {
			case "pty-req":
				var pty ptyReqMsg
				if err := ssh.Unmarshal(req.Payload, &pty); err != nil {
					replyErr(req, false)
					continue
				}
				hasPty = true
				sizeQueue.push(remotecommand.TerminalSize{Width: uint16(pty.Cols), Height: uint16(pty.Rows)})
				replyErr(req, true)

			case "window-change":
				var win winChMsg
				if err := ssh.Unmarshal(req.Payload, &win); err == nil {
					sizeQueue.push(remotecommand.TerminalSize{Width: uint16(win.Cols), Height: uint16(win.Rows)})
				}
				// window-change never wants a reply (req.WantReply is false).

			case "env":
				// Accepted but not forwarded — exec environments come from the
				// container spec, not the SSH client.
				replyErr(req, true)

			case "shell", "exec":
				if started {
					replyErr(req, false)
					continue
				}

				// Only the command is decoded here; shell detection costs a
				// round trip per candidate and belongs in the goroutine.
				var userCommand string
				if req.Type == "exec" {
					var ex execMsg
					if err := ssh.Unmarshal(req.Payload, &ex); err != nil {
						replyErr(req, false)
						continue
					}
					userCommand = ex.Command
				}

				started = true
				replyErr(req, true)

				// Copy the pty state: the loop keeps running and must not be
				// read from the goroutine concurrently.
				pty, interactive := hasPty, req.Type == "shell"
				go func() {
					finished <- runShellPayload(logger, clients, channel, sizeQueue,
						namespace, podName, container, containerNote, userCommand, pty, interactive)
				}()

			case "subsystem":
				var sub subsystemMsg
				if err := ssh.Unmarshal(req.Payload, &sub); err != nil || sub.Name != "sftp" || started {
					replyErr(req, false)
					continue
				}
				started = true
				replyErr(req, true)

				go func() {
					finished <- serveSFTP(logger, clients, channel, namespace, podName, container)
				}()

			default:
				replyErr(req, false)
			}
		}
	}
}

// shellNotFoundExitCode is what the client sees when the image ships no
// usable shell. 127 is the POSIX "command not found" code a real sshd would
// produce here — deliberately not k8sexec.ShellNotFoundExitCode (66), which
// is an internal signal between the operator's own processes for the web
// terminal, and would be meaningless to a standard ssh client.
const shellNotFoundExitCode = 127

// runShellPayload detects a usable shell and streams the exec, returning the
// exit code to report to the client.
func runShellPayload(
	logger *slog.Logger, clients *execClients, channel ssh.Channel, sizeQueue *sizeQueue,
	namespace, podName, container, containerNote, userCommand string, pty bool, interactive bool,
) int {
	shell, err := detectShell(logger, clients, namespace, podName, container)
	if err != nil {
		_, _ = fmt.Fprintf(channel.Stderr(), "mogenius ssh gateway: no usable shell in container %q: %v\r\n", container, err)
		return shellNotFoundExitCode
	}

	command := []string{shell}
	if userCommand != "" {
		command = append(command, "-c", userCommand)
	}

	// Interactive shells get a one-line hint which container was picked;
	// exec/scp streams stay clean.
	if interactive && pty && containerNote != "" {
		_, _ = fmt.Fprintf(channel, "%s\r\n", containerNote)
	}

	return runExec(logger, clients, channel, sizeQueue, namespace, podName, container, command, pty)
}

func replyErr(req *ssh.Request, ok bool) {
	if req.WantReply {
		_ = req.Reply(ok, nil)
	}
}

func sendExitStatus(channel ssh.Channel, code int) {
	_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(exitStatusMsg{Status: uint32(code)})) //nolint:gosec // exit codes are 0-255
}

// runExec streams the Kubernetes exec API to/from the SSH channel and returns
// the process exit code.
func runExec(logger *slog.Logger, clients *execClients, channel ssh.Channel, sizeQueue *sizeQueue, namespace, podName, container string, command []string, tty bool) int {
	executor, err := newExecutor(clients, execParams{
		namespace: namespace,
		pod:       podName,
		container: container,
		command:   command,
		stdin:     true,
		// With a TTY, stderr is merged into the pty stream; the API rejects
		// a separate stderr stream in that case.
		stderr: !tty,
		tty:    tty,
	})
	if err != nil {
		logger.Error("failed to create SPDY executor", "error", err)
		return 1
	}

	opts := remotecommand.StreamOptions{
		Stdin:  channel,
		Stdout: channel,
		Tty:    tty,
	}
	if tty {
		opts.TerminalSizeQueue = sizeQueue
	} else {
		opts.Stderr = channel.Stderr()
	}

	err = executor.StreamWithContext(context.Background(), opts)
	if err == nil {
		return 0
	}

	var codeErr utilexec.CodeExitError
	if errors.As(err, &codeErr) {
		return codeErr.Code
	}
	if errors.Is(err, io.EOF) {
		return 0
	}
	logger.Info("exec stream ended with error", "error", err)
	return 1
}

// detectShell asks the shared prober which shell the image actually ships,
// so an SSH session and the web terminal agree on the answer.
func detectShell(logger *slog.Logger, clients *execClients, namespace, podName, container string) (string, error) {
	executor, err := k8sexec.NewExecutor(
		logger, clients.clientset.CoreV1().RESTClient(), *clients.restConfig, namespace, podName, container)
	if err != nil {
		return "", err
	}
	return k8sexec.DetectShell(executor, logger)
}
