package k8sexec

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"mogenius-operator/src/utils"
	"os"
	"os/signal"
	"syscall"

	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/retry"
)

type Logs interface {
	Start() error
}

type k8sLogs struct {
	logger     *slog.Logger
	restClient v1.PodInterface
	namespace  string
	pod        string
	container  string
	tailLines  int64
}

func NewLogs(logger *slog.Logger, client v1.PodInterface, namespace string, pod string, container string, tailLines int64) (Logs, error) {
	return &k8sLogs{
		logger:     logger,
		restClient: client,
		namespace:  namespace,
		pod:        pod,
		container:  container,
		tailLines:  tailLines,
	}, nil
}

func (e *k8sLogs) Start() error {
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

	// Get the logs request
	err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		return true
	}, func() error {
		req := e.restClient.GetLogs(e.pod, &v1core.PodLogOptions{
			Container: e.container,
			Follow:    true,
			TailLines: utils.Pointer(e.tailLines),
		})
		readCloser, err := req.Stream(ctx)
		if err != nil {
			return err
		}
		defer readCloser.Close()

		scanner := bufio.NewScanner(readCloser)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return err
		}

		return nil
	})
	return err
}
