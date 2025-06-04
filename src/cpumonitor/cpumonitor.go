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
		var lastNice float64 = 0
		var lastSystem float64 = 0
		var lastIdle float64 = 0
		var lastIowait float64 = 0
		var lastIrq float64 = 0
		var lastSoftirq float64 = 0
		var lastSteal float64 = 0

		path := self.config.Get("MO_HOST_PROC_PATH") + "/stat"

		for {
			if !firstRun {
				time.Sleep(1000 * time.Millisecond)
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

			// Parse all CPU time fields
			// Format: cpu user nice system idle iowait irq softirq steal [guest] [guest_nice]
			if len(fields) < 8 {
				self.logger.Error("failed to parse cpu stats", "error", "insufficient fields in cpu line")
				continue
			}

			var values []float64
			for i := 1; i < len(fields) && i < 9; i++ { // Read up to 8 values (skip guest fields to avoid double counting)
				val, err := strconv.ParseFloat(fields[i], 64)
				if err != nil {
					self.logger.Error("failed to parse cpu field as float", "field", fields[i], "error", err)
					continue
				}
				values = append(values, val)
			}

			if len(values) < 7 {
				self.logger.Error("failed to parse cpu stats", "error", "could not parse enough cpu fields")
				continue
			}

			user := values[0]    // user
			nice := values[1]    // nice
			system := values[2]  // system
			idle := values[3]    // idle
			iowait := values[4]  // iowait
			irq := values[5]     // irq
			softirq := values[6] // softirq
			steal := float64(0)
			if len(values) > 7 {
				steal = values[7] // steal
			}

			// Skip first measurement (no previous values to compare)
			if firstRun {
				lastUser = user
				lastNice = nice
				lastSystem = system
				lastIdle = idle
				lastIowait = iowait
				lastIrq = irq
				lastSoftirq = softirq
				lastSteal = steal
				firstRun = false
				continue
			}

			// Calculate deltas
			userDelta := user - lastUser
			niceDelta := nice - lastNice
			systemDelta := system - lastSystem
			idleDelta := idle - lastIdle
			iowaitDelta := iowait - lastIowait
			irqDelta := irq - lastIrq
			softirqDelta := softirq - lastSoftirq
			stealDelta := steal - lastSteal

			// Total time delta
			totalDelta := userDelta + niceDelta + systemDelta + idleDelta +
				iowaitDelta + irqDelta + softirqDelta + stealDelta

			if totalDelta <= 0 {
				// No changes since last measurement
				continue
			}

			// Calculate percentages
			data := CpuMetrics{}
			data.User = ((userDelta + niceDelta) / totalDelta) * 100                   // user + nice
			data.System = ((systemDelta + irqDelta + softirqDelta) / totalDelta) * 100 // system + irq + softirq
			data.Idle = ((idleDelta + iowaitDelta) / totalDelta) * 100                 // idle + iowait

			// Store current values for next iteration
			lastUser = user
			lastNice = nice
			lastSystem = system
			lastIdle = idle
			lastIowait = iowait
			lastIrq = irq
			lastSoftirq = softirq
			lastSteal = steal

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
