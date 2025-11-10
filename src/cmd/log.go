package cmd

import (
	"fmt"
	"log/slog"
	"mogenius-operator/src/config"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/k8sexec"
	"strconv"
	"strings"
)

type logArgs struct {
	Namespace string `help:"" required:""`
	Pod       string `help:"" required:""`
	Container string `help:"" required:""`
	TailLines string `help:"" default:"1000"`
}

func RunLogs(args *logArgs, logger *slog.Logger, configModule config.ConfigModule) error {
	namespace := strings.TrimSpace(args.Namespace)
	if namespace == "" {
		return fmt.Errorf("empty --namespace")
	}

	container := strings.TrimSpace(args.Container)
	if container == "" {
		return fmt.Errorf("empty --container")
	}

	pod := strings.TrimSpace(args.Pod)
	if pod == "" {
		return fmt.Errorf("empty --pod")
	}

	tailLines := strings.TrimSpace(args.TailLines)
	tailLinesUint, err := strconv.ParseInt(tailLines, 10, 64)
	if err != nil {
		tailLinesUint = 1000
	}

	clientProvider := k8sclient.NewK8sClientProvider(logger, configModule)

	executor, err := k8sexec.NewLogs(
		logger,
		clientProvider.K8sClientSet().CoreV1().Pods(namespace),
		namespace,
		pod,
		container,
		tailLinesUint,
	)
	if err != nil {
		return err
	}

	err = executor.Start()
	if err != nil {
		return err
	}

	return nil
}
