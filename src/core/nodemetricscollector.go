package core

import (
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/cpumonitor"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/networkmonitor"
	"mogenius-k8s-manager/src/rammonitor"
	"mogenius-k8s-manager/src/shutdown"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type NodeMetricsCollector interface {
	Run()
	Link(statsDb ValkeyStatsDb)
	Orchestrate()
}

type nodeMetricsCollector struct {
	logger         *slog.Logger
	config         config.ConfigModule
	clientProvider k8sclient.K8sClientProvider
	statsDb        ValkeyStatsDb
	leaderElector  LeaderElector

	cpuMonitor     cpumonitor.CpuMonitor
	ramMonitor     rammonitor.RamMonitor
	networkMonitor networkmonitor.NetworkMonitor
}

func NewNodeMetricsCollector(
	logger *slog.Logger,
	configModule config.ConfigModule,
	clientProviderModule k8sclient.K8sClientProvider,
	cpuMonitor cpumonitor.CpuMonitor,
	ramMonitor rammonitor.RamMonitor,
	networkMonitor networkmonitor.NetworkMonitor,
) NodeMetricsCollector {
	self := &nodeMetricsCollector{}

	self.logger = logger
	self.config = configModule
	self.clientProvider = clientProviderModule
	self.cpuMonitor = cpuMonitor
	self.ramMonitor = ramMonitor
	self.networkMonitor = networkMonitor

	return self
}

func (self *nodeMetricsCollector) Link(statsDb ValkeyStatsDb) {
	assert.Assert(statsDb != nil)

	self.statsDb = statsDb
}

func (self *nodeMetricsCollector) Orchestrate() {
	enabled, err := strconv.ParseBool(self.config.Get("MO_ENABLE_TRAFFIC_COLLECTOR"))
	assert.Assert(err == nil, err)
	self.logger.Info("node metrics collector configuration", "enabled", enabled)
	if !enabled {
		return
	}

	if self.clientProvider.RunsInCluster() {
		assert.Assert(false, "TODO: not implemented")
		// setup daemonset
		self.leaderElector.OnLeading(func() {
			// check if daemonset exists
			// -> create if it doesnt
			self.logger.Info("TODO: check if nodemetricscollector daemonset is installed and running")
		})
	} else {
		go func() {
			bin, err := os.Executable()
			assert.Assert(err == nil, "failed to get current executable path", err)

			nodemetrics := exec.Command(bin, "nodemetrics")
			outputBytes, err := nodemetrics.Output()
			if err != nil {
				// only print the last few lines to hopefully capture error messages
				output := string(outputBytes)
				outputLines := strings.Split(output, "\n")
				lastLinesStart := max(len(outputLines)-11, 0)
				lastLines := strings.Join(outputLines[lastLinesStart:], "\n")
				self.logger.Error("failed to run nodemetrics locally", "output", lastLines, "error", err)
				shutdown.SendShutdownSignal(true)
				select {}
			}
		}()
	}
}

func (self *nodeMetricsCollector) Run() {
	assert.Assert(self.logger != nil)
	assert.Assert(self.config != nil)
	assert.Assert(self.clientProvider != nil)
	assert.Assert(self.statsDb != nil)
	assert.Assert(self.cpuMonitor != nil)
	assert.Assert(self.ramMonitor != nil)
	assert.Assert(self.networkMonitor != nil)

	nodeName := self.config.Get("OWN_NODE_NAME")
	if !self.clientProvider.RunsInCluster() {
		nodeName = "local"
	}
	assert.Assert(nodeName != "")

	// network monitor
	go func() {
		self.networkMonitor.Run()
		go func() {
			for {
				metrics := self.networkMonitor.GetPodNetworkUsage()
				self.statsDb.AddInterfaceStatsToDb(metrics)
				time.Sleep(30 * time.Second)
			}
		}()
		go func() {
			for {
				metrics := self.networkMonitor.GetPodNetworkUsage()
				err := self.statsDb.AddNodeTrafficMetricsToDb(nodeName, metrics)
				if err != nil {
					self.logger.Error("failed to add node traffic metrics", "error", err)
				}
				time.Sleep(1 * time.Second)
			}
		}()
	}()

	// cpu usage
	go func() {
		for {
			metrics := self.cpuMonitor.CpuUsage()
			err := self.statsDb.AddNodeCpuMetricsToDb(nodeName, metrics)
			if err != nil {
				self.logger.Error("failed to add node cpu metrics", "error", err)
			}
			time.Sleep(1 * time.Second)
		}
	}()

	// ram usage
	go func() {
		for {
			metrics := self.ramMonitor.RamUsage()
			err := self.statsDb.AddNodeRamMetricsToDb(nodeName, metrics)
			if err != nil {
				self.logger.Error("failed to add node ram metrics", "error", err)
			}
			time.Sleep(1 * time.Second)
		}
	}()
}
