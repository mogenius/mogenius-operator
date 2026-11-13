package core

import (
	"bufio"
	"fmt"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/shutdown"
	"os"
	"os/exec"
	"runtime"
)

// NodeMetricsOrchestrator starts node-metrics next to the operator when it
// runs outside a cluster (local development); in a cluster the DaemonSet of
// the chart runs it.
type NodeMetricsOrchestrator interface {
	Orchestrate()
}

type nodeMetricsOrchestrator struct {
	logger         *slog.Logger
	config         config.ConfigModule
	clientProvider k8sclient.K8sClientProvider
}

func NewNodeMetricsOrchestrator(
	logger *slog.Logger,
	config config.ConfigModule,
	clientProviderModule k8sclient.K8sClientProvider,
) NodeMetricsOrchestrator {
	return &nodeMetricsOrchestrator{
		logger:         logger,
		config:         config,
		clientProvider: clientProviderModule,
	}
}

func (self *nodeMetricsOrchestrator) Orchestrate() {

	ownDeploymentName := self.config.Get("OWN_DEPLOYMENT_NAME")
	assert.Assert(ownDeploymentName != "")

	trafficCollectorEnabled, err := self.config.TryGetBool("MO_ENABLE_TRAFFIC_COLLECTOR")
	assert.Assert(err == nil, err)

	self.logger.Info("node metrics collector configuration", "enabled", trafficCollectorEnabled)
	if !trafficCollectorEnabled {
		return
	}

	if runtime.GOOS == "darwin" {
		self.logger.Error("SKIPPING node metrics collector setup on macOS", "reason", "not supported on macOS")
		return
	}

	if !self.clientProvider.RunsInCluster() {
		go func() {

			bin, err := os.Executable()
			assert.Assert(err == nil, "failed to get current executable path", err)

			nodemetrics := exec.Command(bin, "nodemetrics")

			// This buffer is allocated both for stdout and stderr.
			// Since this only happens in local development we dont have to care for a few megabytes of statically allocated memory.
			bufSize := 5 * 1024 * 1024 // 5MiB
			stdoutPipe, err := nodemetrics.StdoutPipe()
			assert.Assert(err == nil, "reading stdout of this child process has to work", err)
			stderrPipe, err := nodemetrics.StderrPipe()
			assert.Assert(err == nil, "reading stderr of this child process has to work", err)

			go func() {
				scanner := bufio.NewScanner(stdoutPipe)
				scanner.Buffer(make([]byte, bufSize), bufSize)
				for scanner.Scan() {
					output := string(scanner.Bytes())
					fmt.Fprintf(os.Stderr, "node-metrics %s | %s\n", "stdout", output)
				}
			}()

			go func() {
				scanner := bufio.NewScanner(stderrPipe)
				scanner.Buffer(make([]byte, bufSize), bufSize)
				for scanner.Scan() {
					output := scanner.Bytes()
					fmt.Fprintf(os.Stderr, "| node-metrics %s | %s\n", "stderr", output)
				}
			}()

			err = nodemetrics.Start()
			if err != nil {
				self.logger.Error("failed to start node-metrics", "error", err)
				shutdown.SendShutdownSignal(true)
				select {}
			}

			err = nodemetrics.Wait()
			if err != nil {
				self.logger.Error("failed to wait for node-metrics", "error", err)
				shutdown.SendShutdownSignal(true)
				select {}
			}
		}()
	}
}
