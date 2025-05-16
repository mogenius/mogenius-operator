package cpumonitor

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

type CpuMonitor interface {
	CpuUsage() CpuMetrics
}

type cpuMonitor struct {
	logger         *slog.Logger
	config         config.ConfigModule
	clientProvider k8sclient.K8sClientProvider

	collectorStarted atomic.Bool

	lastMetrics     CpuMetrics
	lastMetricsLock sync.RWMutex
}

func NewCpuMonitor(logger *slog.Logger, config config.ConfigModule, clientProvider k8sclient.K8sClientProvider) CpuMonitor {
	self := &cpuMonitor{}

	self.logger = logger
	self.config = config
	self.clientProvider = clientProvider
	self.collectorStarted = atomic.Bool{}
	self.lastMetrics = CpuMetrics{}
	self.lastMetricsLock = sync.RWMutex{}

	return self
}

func (self *cpuMonitor) CpuUsage() CpuMetrics {
	alreadyStarted := self.collectorStarted.Swap(true)
	if !alreadyStarted {
		self.startCollector()
	}

	self.lastMetricsLock.RLock()
	metrics := self.lastMetrics
	self.lastMetricsLock.RUnlock()

	return metrics
}

func (self *cpuMonitor) startCollector() {
	if runtime.GOOS != "linux" {
		return
	}
	go func() {
		firstRun := true
		var lastUser float64 = 0
		var lastSystem float64 = 0
		var lastIdle float64 = 0

		path := self.config.Get("MO_HOST_PROC_PATH") + "/stat"

		for {
			if !firstRun {
				time.Sleep(100 * time.Millisecond)
			}
			firstRun = false

			fileData, err := os.ReadFile(path)
			if err != nil {
				self.logger.Error("failed to read cpu stats", "path", path, "error", err)
				continue
			}

			lines := strings.Split(string(fileData), "\n")
			cpuLine := lines[0]
			assert.Assert(cpuLine != "")

			fields := strings.Fields(cpuLine)
			if fields[0] != "cpu" {
				self.logger.Error("failed to parse cpu stats", "error", "unexpected cpu stats file format: first field not 'cpu'")
				continue
			}

			var user, system, idle float64
			for i, field := range fields {
				if field == "" || field == "cpu" {
					continue
				}
				// println(i, field)
				val, err := strconv.ParseFloat(field, 32)
				if err != nil {
					self.logger.Error("failed to parse cpu field as float", "error", err)
					continue
				}
				// user & user-nice index 1 + 2
				if i == 1 || i == 2 {
					user += val
				}
				// system index 3
				if i == 3 {
					system = val
				}
				// idle index 4
				if i == 4 {
					idle = val
				}
			}

			user_delta := user - lastUser
			system_delta := system - lastSystem
			idle_delta := idle - lastIdle
			total_delta := (user + system + idle) - (lastUser + lastSystem + lastIdle)

			data := CpuMetrics{}
			data.User = ((lastUser - user) + 100) / 2
			data.System = ((lastSystem - system) + 100) / 2
			data.Idle = ((lastIdle - idle) + 100) / 2

			// PERCENTAGES
			if total_delta <= 0 {
				// no changes since last measurement
				continue
			}

			data.User = (user_delta / total_delta) * 100
			data.System = (system_delta / total_delta) * 100
			data.Idle = (idle_delta / total_delta) * 100

			lastUser = user
			lastSystem = system
			lastIdle = idle

			self.lastMetricsLock.Lock()
			self.lastMetrics = data
			self.lastMetricsLock.Unlock()
		}
	}()
}

type CpuMetrics struct {
	User   float64 `json:"user"`
	System float64 `json:"system"`
	Idle   float64 `json:"idle"`
}
