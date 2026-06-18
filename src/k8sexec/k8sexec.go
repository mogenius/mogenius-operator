package k8sexec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// probeTimeout bounds a single non-interactive shell probe.
const probeTimeout = 5 * time.Second

// ShellNotFoundExitCode is the process exit code used when no usable shell is
// found in the container. The parent process maps it to a NO_SHELL_AVAILABLE
// close reason so the frontend can render a dedicated message instead of a
// raw OCI runtime error. Chosen to not collide with shell exit codes or the
// 137 (SIGKILL) code already handled by the parent.
const ShellNotFoundExitCode = 66

type Executor interface {
	Start(command []string) error
	// Probe runs command non-interactively (no TTY, no stdin, output discarded)
	// and returns nil only if it starts and exits with code 0. It is used to
	// detect which shell binary is available inside the container before
	// opening an interactive session.
	Probe(command []string) error
}

type k8sExecutor struct {
	logger     *slog.Logger
	restClient rest.Interface
	restConfig rest.Config
	namespace  string
	pod        string
	container  string
}

func NewExecutor(logger *slog.Logger, client rest.Interface, config rest.Config, namespace string, pod string, container string) (Executor, error) {
	return &k8sExecutor{
		logger:     logger,
		restClient: client,
		restConfig: config,
		namespace:  namespace,
		pod:        pod,
		container:  container,
	}, nil
}

func (e *k8sExecutor) Probe(command []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()

	execRequest := e.restClient.
		Post().
		Resource("pods").
		Name(e.pod).
		Namespace(e.namespace).
		SubResource("exec").
		Param("container", e.container).
		Param("stdout", "true").
		Param("stdin", "false").
		Param("stderr", "true").
		Param("tty", "false")

	for _, arg := range command {
		execRequest.Param("command", arg)
	}
	executor, err := remotecommand.NewSPDYExecutor(&e.restConfig, "POST", execRequest.URL())
	if err != nil {
		return err
	}

	// Discard stdout but capture stderr so the original runtime error (e.g.
	// `exec: "sh": executable file not found in $PATH`) can be surfaced.
	var stderr bytes.Buffer
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: io.Discard,
		Stderr: &stderr,
	})
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%s (%w)", msg, err)
		}
		return err
	}
	return nil
}

func (e *k8sExecutor) Start(command []string) error {
	// context to trigger close of the interactive shell
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// listener for os close signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	go func() {
		<-sig
		e.logger.Debug("received interrupt, exiting...")
		cancel()
	}()

	// eventhandler for terminal resize events
	sizeQueue := NewSizeQueue()
	defer signal.Stop(sizeQueue.resize)

	// trigger `cancel()` on EOF
	// this happens when the user presses CTRL+D
	wrappedStdin := &eofReader{
		reader: os.Stdin,
		cancel: cancel,
	}

	// set the local terminal to raw mode to pass-through any escape sequences
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	// restore on exit
	defer func() {
		err := term.Restore(int(os.Stdin.Fd()), oldState)
		if err != nil {
			e.logger.Error("failed to revert terminal raw mode", "error", err)
		}
	}()

	// construct interactive session
	execRequest := e.restClient.
		Post().
		Resource("pods").
		Name(e.pod).
		Namespace(e.namespace).
		SubResource("exec").
		Param("container", e.container).
		Param("stdout", "true").
		Param("stdin", "true").
		Param("stderr", "true").
		Param("tty", "true")

	for _, arg := range command {
		execRequest.Param("command", arg)
	}
	executor, err := remotecommand.NewSPDYExecutor(&e.restConfig, "POST", execRequest.URL())
	if err != nil {
		return err
	}

	// start the shell and block while it is active
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             wrappedStdin,
		Stdout:            os.Stdout,
		Stderr:            os.Stderr,
		Tty:               true,
		TerminalSizeQueue: sizeQueue,
	})
	if err != nil {
		return err
	}

	return nil
}
