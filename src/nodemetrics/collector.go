package nodemetrics

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/cpumonitor"
	"mogenius-operator/src/networkmonitor"
	"mogenius-operator/src/rammonitor"
	"mogenius-operator/src/shutdown"
	"mogenius-operator/src/structs"
	"sync/atomic"
	"time"
)

// snoopyStatusRefreshInterval rewrites an unchanged snoopy status so its key
// (liveStatsTTL) never expires while the node-metrics pod is running.
const snoopyStatusRefreshInterval = 5 * time.Minute

// processStatsDemandInterval is how often node-metrics checks for live
// stream subscribers of the per-process stats; it bounds how long a newly
// opened live view waits for its first frame.
const processStatsDemandInterval = 2 * time.Second

// sleepCtx returns false when ctx is done so the caller can exit its loop.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// statsWriter is the part of statsdb.ValkeyStatsDb the collector writes to.
type statsWriter interface {
	AddMachineStatsToDb(nodeName string, stats structs.MachineStats) error
	AddInterfaceStatsToDb(stats []networkmonitor.PodNetworkStats)
	AddNodeTrafficMetricsToDb(nodeName string, data any) error
	AddSnoopyStatusToDb(nodeName string, data networkmonitor.SnoopyStatus) error
	AddNodeCpuMetricsToDb(nodeName string, data any) error
	AddNodeCpuProcessMetricsToDb(nodeName string, data any) error
	AddNodeRamMetricsToDb(nodeName string, data any) error
	AddNodeRamProcessMetricsToDb(nodeName string, data any) error
	LiveProcessStatsSubscribed(nodeName string) (cpu bool, memory bool)
}

// Collector samples cpu, memory and network usage of its node and writes
// them to Valkey: live values every second, history every minute.
type Collector struct {
	logger         *slog.Logger
	nodeName       string
	statsDb        statsWriter
	cpuMonitor     cpumonitor.CpuMonitor
	ramMonitor     rammonitor.RamMonitor
	networkMonitor networkmonitor.NetworkMonitor
}

// NewCollector creates the collector for nodeName.
func NewCollector(
	logger *slog.Logger,
	nodeName string,
	statsDb statsWriter,
	cpuMonitor cpumonitor.CpuMonitor,
	ramMonitor rammonitor.RamMonitor,
	networkMonitor networkmonitor.NetworkMonitor,
) *Collector {
	assert.Assert(logger != nil)
	assert.Assert(nodeName != "", "OWN_NODE_NAME has to be defined and non-empty")
	assert.Assert(statsDb != nil)
	assert.Assert(cpuMonitor != nil)
	assert.Assert(ramMonitor != nil)
	assert.Assert(networkMonitor != nil)

	return &Collector{
		logger:         logger,
		nodeName:       nodeName,
		statsDb:        statsDb,
		cpuMonitor:     cpuMonitor,
		ramMonitor:     ramMonitor,
		networkMonitor: networkMonitor,
	}
}

// Run starts the sampling loops; they stop on shutdown.
func (self *Collector) Run() {
	nodeName := self.nodeName

	ctx, cancel := context.WithCancel(context.Background())
	shutdown.Add(cancel)

	// Per-process cpu and memory stats only feed live streams. Track whether
	// one is open and skip the /proc scans and publishes while none is.
	var cpuProcessStatsWanted, ramProcessStatsWanted atomic.Bool
	updateProcessStatsDemand := func() {
		cpu, memory := self.statsDb.LiveProcessStatsSubscribed(nodeName)
		cpuProcessStatsWanted.Store(cpu)
		ramProcessStatsWanted.Store(memory)
	}
	updateProcessStatsDemand()
	go func() {
		for sleepCtx(ctx, processStatsDemandInterval) {
			updateProcessStatsDemand()
		}
	}()

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
			if !sleepCtx(ctx, 1*time.Minute) {
				return
			}
		}
	}()

	// network monitor
	go func() {
		self.networkMonitor.Run()
		go func() {
			for {
				metrics := self.networkMonitor.GetPodNetworkUsage()
				self.statsDb.AddInterfaceStatsToDb(metrics)
				if !sleepCtx(ctx, 60*time.Second) {
					return
				}
			}
		}()
		go func() {
			// offset: 0ms
			for {
				metrics := self.networkMonitor.GetPodNetworkUsage()
				err := self.statsDb.AddNodeTrafficMetricsToDb(nodeName, metrics)
				if err != nil {
					self.logger.Error("failed to add node traffic metrics", "error", err)
				}
				if !sleepCtx(ctx, 1*time.Second) {
					return
				}
			}
		}()
		go func() {
			// offset: 200ms
			if !sleepCtx(ctx, 200*time.Millisecond) {
				return
			}
			// The status only changes when pods or interfaces come and go;
			// rewriting the same JSON every second was pure Valkey write
			// load. Write on change, and refresh well before the key's TTL.
			var lastStatus []byte
			var lastWrite time.Time
			for {
				status := self.networkMonitor.Snoopy().Status()
				encoded, err := json.Marshal(status)
				if err != nil {
					self.logger.Error("failed to encode snoopy status", "error", err)
				} else if !bytes.Equal(encoded, lastStatus) || time.Since(lastWrite) >= snoopyStatusRefreshInterval {
					err := self.statsDb.AddSnoopyStatusToDb(nodeName, status)
					if err != nil {
						self.logger.Error("failed to store snoopy status", "error", err)
					} else {
						lastStatus = encoded
						lastWrite = time.Now()
					}
				}
				if !sleepCtx(ctx, 1*time.Second) {
					return
				}
			}
		}()
	}()

	// cpu usage
	go func() {
		// offset: 400ms
		if !sleepCtx(ctx, 400*time.Millisecond) {
			return
		}
		for {
			metrics := self.cpuMonitor.CpuUsageGlobal()
			err := self.statsDb.AddNodeCpuMetricsToDb(nodeName, metrics)
			if err != nil {
				self.logger.Error("failed to add node cpu metrics", "error", err)
			}
			if !sleepCtx(ctx, 1*time.Second) {
				return
			}
		}
	}()
	go func() {
		// offset: 600ms
		if !sleepCtx(ctx, 600*time.Millisecond) {
			return
		}
		for {
			if cpuProcessStatsWanted.Load() {
				metrics := self.cpuMonitor.CpuUsageProcesses()
				err := self.statsDb.AddNodeCpuProcessMetricsToDb(nodeName, metrics)
				if err != nil {
					self.logger.Error("failed to add node cpu proc metrics", "error", err)
				}
			}
			if !sleepCtx(ctx, 1*time.Second) {
				return
			}
		}
	}()

	// ram usage
	go func() {
		// offset: 800ms
		if !sleepCtx(ctx, 800*time.Millisecond) {
			return
		}
		for {
			metrics := self.ramMonitor.RamUsageGlobal()
			err := self.statsDb.AddNodeRamMetricsToDb(nodeName, metrics)
			if err != nil {
				self.logger.Error("failed to add node ram metrics", "error", err)
			}
			if !sleepCtx(ctx, 1*time.Second) {
				return
			}
		}
	}()
	go func() {
		// offset: 1000ms
		if !sleepCtx(ctx, 1000*time.Millisecond) {
			return
		}
		for {
			if ramProcessStatsWanted.Load() {
				metrics := self.ramMonitor.RamUsageProcesses()
				err := self.statsDb.AddNodeRamProcessMetricsToDb(nodeName, metrics)
				if err != nil {
					self.logger.Error("failed to add node ram proc metrics", "error", err)
				}
			}
			if !sleepCtx(ctx, 1*time.Second) {
				return
			}
		}
	}()
}
