package core

import (
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/cpumonitor"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/networkmonitor"
	"mogenius-k8s-manager/src/rammonitor"
	"strconv"
	"time"
)

type NodeMetricsCollector interface {
	Run()
	Link(statsDb ValkeyStatsDb)
}

type nodeMetricsCollector struct {
	logger         *slog.Logger
	config         config.ConfigModule
	clientProvider k8sclient.K8sClientProvider
	statsDb        ValkeyStatsDb

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

func (self *nodeMetricsCollector) Run() {
	assert.Assert(self.logger != nil)
	assert.Assert(self.config != nil)
	assert.Assert(self.clientProvider != nil)
	assert.Assert(self.statsDb != nil)
	assert.Assert(self.cpuMonitor != nil)
	assert.Assert(self.ramMonitor != nil)
	assert.Assert(self.networkMonitor != nil)

	enabled, err := strconv.ParseBool(self.config.Get("MO_ENABLE_TRAFFIC_COLLECTOR"))
	assert.Assert(err == nil, err)
	if enabled {
		// network monitor
		go func() {
			self.networkMonitor.Run()
			for {
				metrics := self.networkMonitor.NetworkUsage()
				self.logger.Info("network usage", "metrics", len(metrics))
				// TODO: @daniel (hier brauchen wir ein object wo der livetraffic des nodes ankommt)
				self.statsDb.AddInterfaceStatsToDb(metrics)
				time.Sleep(30 * time.Second)
			}
		}()

		// cpu usage
		go func() {
			for {
				metrics := self.cpuMonitor.CpuUsage()
				self.statsDb.AddNodeCpuMetricsToDb(self.config.Get("OWN_NODE_NAME"), metrics)
				time.Sleep(1 * time.Second)
			}
		}()

		// ram usage
		go func() {
			for {
				metrics := self.ramMonitor.RamUsage()
				self.statsDb.AddNodeRamMetricsToDb(self.config.Get("OWN_NODE_NAME"), metrics)
				time.Sleep(1 * time.Second)
			}
		}()
	}
}
