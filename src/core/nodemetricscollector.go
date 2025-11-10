package core

import (
	"bufio"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/cpumonitor"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/networkmonitor"
	"mogenius-k8s-manager/src/rammonitor"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/structs"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"
)

type NodeMetricsCollector interface {
	// Run the nodemetrics collector locally.
	Run()
	Link(statsDb ValkeyStatsDb, leaderElector LeaderElector)
	// Manage instances of nodemetrics collector.
	// Either create the required DaemonSet or handle execution locally.
	Orchestrate()
}

type nodeMetricsCollector struct {
	logger         *slog.Logger
	config         config.ConfigModule
	procPath       string
	clientProvider k8sclient.K8sClientProvider
	statsDb        ValkeyStatsDb
	leaderElector  LeaderElector

	cpuMonitor     cpumonitor.CpuMonitor
	ramMonitor     rammonitor.RamMonitor
	networkMonitor networkmonitor.NetworkMonitor
}

func NewNodeMetricsCollector(
	logger *slog.Logger,
	config config.ConfigModule,
	clientProviderModule k8sclient.K8sClientProvider,
	cpuMonitor cpumonitor.CpuMonitor,
	ramMonitor rammonitor.RamMonitor,
	networkMonitor networkmonitor.NetworkMonitor,
) NodeMetricsCollector {
	self := &nodeMetricsCollector{}

	self.logger = logger
	self.config = config
	self.procPath = config.Get("MO_HOST_PROC_PATH")
	self.clientProvider = clientProviderModule
	self.cpuMonitor = cpuMonitor
	self.ramMonitor = ramMonitor
	self.networkMonitor = networkMonitor

	return self
}

func (self *nodeMetricsCollector) Link(statsDb ValkeyStatsDb, leaderElector LeaderElector) {
	assert.Assert(statsDb != nil)
	assert.Assert(leaderElector != nil)

	self.statsDb = statsDb
	self.leaderElector = leaderElector
}

func (self *nodeMetricsCollector) Orchestrate() {

	ownDeploymentName := self.config.Get("OWN_DEPLOYMENT_NAME")
	assert.Assert(ownDeploymentName != "")

	trafficCollectorEnabled, err := strconv.ParseBool(self.config.Get("MO_ENABLE_TRAFFIC_COLLECTOR"))
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

func (self *nodeMetricsCollector) Run() {
	assert.Assert(self.logger != nil)
	assert.Assert(self.config != nil)
	assert.Assert(self.clientProvider != nil)
	assert.Assert(self.statsDb != nil)
	assert.Assert(self.cpuMonitor != nil)
	assert.Assert(self.ramMonitor != nil)
	assert.Assert(self.networkMonitor != nil)

	nodeName := self.config.Get("OWN_NODE_NAME")
	assert.Assert(nodeName != "", "OWN_NODE_NAME has to be defined and non-empty", nodeName)

	// node-stats monitor
	go func() {
		machinestats := structs.MachineStats{
			BtfSupport: self.networkMonitor.BtfAvailable(),
		}
		for {
			err := self.statsDb.AddMachineStatsToDb(nodeName, machinestats)
			if err != nil {
				self.logger.Warn("failed to write machine stats for node", "node", nodeName, "error", err)
			}
			time.Sleep(1 * time.Minute)
		}
	}()

	// network monitor
	go func() {
		self.networkMonitor.Run()
		go func() {
			for {
				metrics := self.networkMonitor.GetPodNetworkUsage()
				self.statsDb.AddInterfaceStatsToDb(metrics)
				time.Sleep(60 * time.Second)
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
		go func() {
			for {
				status := self.networkMonitor.Snoopy().Status()
				err := self.statsDb.AddSnoopyStatusToDb(nodeName, status)
				if err != nil {
					self.logger.Error("failed to store snoopy status", "error", err)
				}
				time.Sleep(1 * time.Second)
			}
		}()
	}()

	// cpu usage
	go func() {
		go func() {
			for {
				metrics := self.cpuMonitor.CpuUsageGlobal()
				err := self.statsDb.AddNodeCpuMetricsToDb(nodeName, metrics)
				if err != nil {
					self.logger.Error("failed to add node cpu metrics", "error", err)
				}
				time.Sleep(1 * time.Second)
			}
		}()
		go func() {
			for {
				metrics := self.cpuMonitor.CpuUsageProcesses()
				err := self.statsDb.AddNodeCpuProcessMetricsToDb(nodeName, metrics)
				if err != nil {
					self.logger.Error("failed to add node cpu proc metrics", "error", err)
				}
				time.Sleep(1 * time.Second)
			}
		}()
	}()

	// ram usage
	go func() {
		go func() {
			for {
				metrics := self.ramMonitor.RamUsageGlobal()
				err := self.statsDb.AddNodeRamMetricsToDb(nodeName, metrics)
				if err != nil {
					self.logger.Error("failed to add node ram metrics", "error", err)
				}
				time.Sleep(1 * time.Second)
			}
		}()

		go func() {
			for {
				metrics := self.ramMonitor.RamUsageProcesses()
				err := self.statsDb.AddNodeRamProcessMetricsToDb(nodeName, metrics)
				if err != nil {
					self.logger.Error("failed to add node ram proc metrics", "error", err)
				}
				time.Sleep(1 * time.Second)
			}
		}()
	}()
}
