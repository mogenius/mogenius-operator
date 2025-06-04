package rammonitor

import (
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/k8sclient"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type RamMonitor interface {
	RamUsage() RamMetrics
}

type ramMonitor struct {
	logger         *slog.Logger
	config         config.ConfigModule
	clientProvider k8sclient.K8sClientProvider

	collectorStarted atomic.Bool

	lastMetrics     RamMetrics
	lastMetricsLock sync.RWMutex
}

func NewRamMonitor(logger *slog.Logger, config config.ConfigModule, clientProvider k8sclient.K8sClientProvider) RamMonitor {
	self := &ramMonitor{}

	self.logger = logger
	self.clientProvider = clientProvider
	self.config = config
	self.collectorStarted = atomic.Bool{}
	self.lastMetrics = RamMetrics{}
	self.lastMetricsLock = sync.RWMutex{}

	return self
}

func (self *ramMonitor) RamUsage() RamMetrics {
	alreadyStarted := self.collectorStarted.Swap(true)
	if !alreadyStarted {
		self.startCollector()
	}

	self.lastMetricsLock.RLock()
	metrics := self.lastMetrics
	self.lastMetricsLock.RUnlock()

	return metrics
}

func (self *ramMonitor) startCollector() {
	if runtime.GOOS != "linux" {
		return
	}
	go func() {
		firstRun := true
		nodeName := self.config.Get("OWN_NODE_NAME")
		if !self.clientProvider.RunsInCluster() {
			nodeName = "local"
		}
		assert.Assert(nodeName != "")

		path := self.config.Get("MO_HOST_PROC_PATH") + "/meminfo"

		for {
			if !firstRun {
				time.Sleep(1000 * time.Millisecond)
			}
			firstRun = false

			data := RamMetrics{}

			data.NodeName = nodeName
			fileData, err := os.ReadFile(path)
			if err != nil {
				self.logger.Error("failed to read ram stats", "path", path, "error", err)
				continue
			}

			lines := strings.Split(string(fileData), "\n")
			var memAvailable float64

			for _, line := range lines {
				fields := strings.Fields(line)
				if len(fields) < 2 {
					continue
				}

				val, err := strconv.ParseFloat(fields[1], 64)
				if err != nil {
					self.logger.Error("failed to parse ram field as float", "error", err)
					continue
				}

				switch fields[0] {
				case "MemTotal:":
					data.TotalKb = val
				case "MemAvailable:":
					memAvailable = val
				}
			}
			data.UsedKb = data.TotalKb - memAvailable

			self.lastMetricsLock.Lock()
			self.lastMetrics = data
			self.lastMetricsLock.Unlock()
		}
	}()
}

type RamMetrics struct {
	TotalKb  float64 `json:"totalKb"`
	UsedKb   float64 `json:"usedKb"`
	NodeName string  `json:"nodeName"`
}
