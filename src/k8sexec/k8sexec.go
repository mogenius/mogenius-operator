package k8sexec

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

type Executor interface {
	Start(command []string) error
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
