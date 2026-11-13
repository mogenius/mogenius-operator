package rammonitor

import (
	"fmt"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/containerenumerator"
	"mogenius-operator/src/k8sclient"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type RamMonitor interface {
	RamUsageGlobal() RamMetrics
	RamUsageProcesses() []PodRamStats
}

type ramMonitor struct {
	logger              *slog.Logger
	config              config.ConfigModule
	procPath            string
	clientProvider      k8sclient.K8sClientProvider
	containerEnumerator containerenumerator.ContainerEnumerator

	running atomic.Bool

	ramUsageGlobalTx chan struct{}
	ramUsageGlobalRx chan RamMetrics

	ramUsageProcessesTx chan struct{}
	ramUsageProcessesRx chan ramProcessMetricsSnapshot
}

// ramProcessMetricsSnapshot bundles the pod list together with the per-container
// stats collected from the same enumerator read. Returning them separately would
// require RamUsageProcesses to query the container enumerator a second time,
// which could race against the enumerator's own 5s cache refresh and return a
// pods list from a different generation than the stats it's being matched against.
type ramProcessMetricsSnapshot struct {
	pods []containerenumerator.PodInfo
	data map[containerenumerator.ContainerId][]ProcPidStatm
}

func NewRamMonitor(logger *slog.Logger, config config.ConfigModule, clientProvider k8sclient.K8sClientProvider, containerEnumerator containerenumerator.ContainerEnumerator) RamMonitor {
	self := &ramMonitor{}

	self.logger = logger
	self.config = config
	self.clientProvider = clientProvider
	self.containerEnumerator = containerEnumerator
	self.procPath = config.Get("MO_HOST_PROC_PATH")
	self.running = atomic.Bool{}

	self.ramUsageGlobalTx = make(chan struct{})
	self.ramUsageGlobalRx = make(chan RamMetrics)
	self.ramUsageProcessesTx = make(chan struct{})
	self.ramUsageProcessesRx = make(chan ramProcessMetricsSnapshot)

	return self
}

func (self *ramMonitor) startCollector() {
	wasRunning := self.running.Swap(true)
	if wasRunning {
		return
	}
	if runtime.GOOS != "linux" {
		// Still serve the request channels (with zero values) — without a
		// reader the first RamUsageGlobal()/RamUsageProcesses() call would
		// block forever on the unbuffered channel.
		go func() {
			for {
				select {
				case <-self.ramUsageGlobalTx:
					self.ramUsageGlobalRx <- RamMetrics{}
				case <-self.ramUsageProcessesTx:
					self.ramUsageProcessesRx <- ramProcessMetricsSnapshot{}
				}
			}
		}()
		return
	}
	go func() {
		nodeName := self.config.Get("OWN_NODE_NAME")
		assert.Assert(nodeName != "", "OWN_NODE_NAME has to be defined and non-empty", nodeName)

		globalMetricsUpdater := time.NewTicker(1 * time.Second)
		defer globalMetricsUpdater.Stop()

		globalMetrics := self.collectGlobalMetrics(nodeName)

		for {
			select {
			case <-globalMetricsUpdater.C:
				globalMetrics = self.collectGlobalMetrics(nodeName)
			case <-self.ramUsageGlobalTx:
				self.ramUsageGlobalRx <- globalMetrics
			case <-self.ramUsageProcessesTx:
				// Collected on request: the per-pid values are absolute
				// counters, and reading /proc/$pid/statm for every container
				// process each second was wasted while nobody asked.
				self.ramUsageProcessesRx <- self.collectProcessMetrics()
			}
		}
	}()
}

type RamMetrics struct {
	TotalKb  float64 `json:"totalKb"`
	UsedKb   float64 `json:"usedKb"`
	NodeName string  `json:"nodeName"`
}

func (self *ramMonitor) collectGlobalMetrics(nodeName string) RamMetrics {
	path := self.procPath + "/meminfo"
	data := RamMetrics{}

	data.NodeName = nodeName
	fileData, err := os.ReadFile(path)
	if err != nil {
		self.logger.Error("failed to read ram stats", "path", path, "error", err)
		return RamMetrics{}
	}

	var memAvailable float64
	foundTotal, foundAvailable := false, false

	// strings.Lines iterates without allocating a slice for all ~50 lines;
	// the loop exits after MemTotal/MemAvailable (the first few lines).
	for line := range strings.Lines(string(fileData)) {
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
			foundTotal = true
		case "MemAvailable:":
			memAvailable = val
			foundAvailable = true
		}
		if foundTotal && foundAvailable {
			break
		}
	}
	data.UsedKb = data.TotalKb - memAvailable

	return data
}

func (self *ramMonitor) RamUsageGlobal() RamMetrics {
	self.startCollector()

	self.ramUsageGlobalTx <- struct{}{}
	data := <-self.ramUsageGlobalRx

	return data
}

func (self *ramMonitor) collectProcessMetrics() ramProcessMetricsSnapshot {
	pods := self.containerEnumerator.GetPodsWithContainerIds()
	containers := containerenumerator.PodProcesses(pods)

	data := make(map[containerenumerator.ContainerId][]ProcPidStatm)
	for containerId, pids := range containers {
		infos := []ProcPidStatm{}
		for _, pid := range pids {
			info, err := getMemoryUsageInfo(self.procPath, strconv.FormatUint(pid, 10))
			if err != nil {
				continue
			}
			infos = append(infos, info)
		}
		data[containerId] = infos
	}

	return ramProcessMetricsSnapshot{pods: pods, data: data}
}

func (self *ramMonitor) RamUsageProcesses() []PodRamStats {
	self.startCollector()

	self.ramUsageProcessesTx <- struct{}{}
	snapshot := <-self.ramUsageProcessesRx

	stats := []PodRamStats{}
	for _, pod := range snapshot.pods {
		for containerId := range pod.Containers {
			procPidStatm, ok := snapshot.data[containerId]
			if !ok {
				continue
			}
			podRamStats := self.procPidStatmToRamUsage(pod, procPidStatm)
			stats = append(stats, podRamStats)
		}
	}

	return stats
}

func (self *ramMonitor) procPidStatmToRamUsage(pod containerenumerator.PodInfo, stats []ProcPidStatm) PodRamStats {
	data := PodRamStats{}
	data.Name = pod.Name
	data.Namespace = pod.Namespace
	data.StartTime = pod.StartTime
	for _, stat := range stats {
		pidData := RamUsagePodPid{}
		pidData.Pid = stat.Pid
		pidData.MemVirtual = stat.Size
		pidData.MemResident = stat.Resident
		pidData.MemShared = stat.Shared
		data.Pids = append(data.Pids, pidData)
	}
	return data
}

type PodRamStats struct {
	Name      string           `json:"name"`
	Namespace string           `json:"namespace"`
	StartTime string           `json:"start_time"`
	Pids      []RamUsagePodPid `json:"Pids"`
}

type RamUsagePodPid struct {
	Pid         uint64 `json:"pid"`
	MemVirtual  uint64 `json:"mem_virtual"`
	MemResident uint64 `json:"mem_resident"`
	MemShared   uint64 `json:"mem_shared"`
}

type ProcPidStatm struct {
	Pid uint64 `json:"pid"`
	//   size       (1) total program size
	//              (same as VmSize in /proc/pid/status)
	Size uint64 `json:"size"`
	//   resident   (2) resident set size
	//              (inaccurate; same as VmRSS in /proc/pid/status)
	Resident uint64 `json:"resident"`
	//   shared     (3) number of resident shared pages
	//              (i.e., backed by a file)
	//              (inaccurate; same as RssFile+RssShmem in
	//              /proc/pid/status)
	Shared uint64 `json:"shared"`
	//   text       (4) text (code)
	Text uint64 `json:"text"`
	//   lib        (5) library (unused since Linux 2.6; always 0)
	Lib uint64 `json:"lib"`
	//   data       (6) data + stack
	Data uint64 `json:"data"`
	//   dt         (7) dirty pages (unused since Linux 2.6; always 0)
	Dt uint64 `json:"dt"`
}

// a common statm line is well under 100 bytes; this buffer size is a guess to
// be *always* larger in *any* environment than what `/proc/$pid/statm` contains.
// The buffer is pooled: this runs per PID per second and a fresh allocation
// each time is pure GC churn (mirrors the pattern in cpumonitor.getCpuUsageInfo).
const statmReadBufSize = 256

var statmReadBufPool = sync.Pool{
	New: func() any { return make([]byte, statmReadBufSize) },
}

// read and parse `$procPath/$pid/statm` to read process memory usage information from the kernel
func getMemoryUsageInfo(procPath string, pid string) (ProcPidStatm, error) {
	// File Format of `/proc/$pid/statm`
	// ===================================
	//
	// ```
	// 19869 2428 1692 1466 0 1289 0
	// ```
	//
	// Parsing Rules
	// =============
	//
	// - values are separated by spaces

	asUint64 := func(val string) uint64 {
		intval, err := strconv.ParseUint(val, 10, 64)
		assert.Assert(err == nil, "val is expected to be an uint64", val, err)
		return intval
	}

	processPath := filepath.Join(procPath, pid)
	deviceInfoPath := filepath.Join(processPath, "statm")

	f, err := os.Open(deviceInfoPath)
	if err != nil {
		return ProcPidStatm{}, err
	}
	defer func() { _ = f.Close() }()

	data := statmReadBufPool.Get().([]byte)
	defer statmReadBufPool.Put(data) //nolint:staticcheck // fixed-size slice, no pointer indirection concern

	bytesRead, err := f.Read(data)
	if err != nil {
		return ProcPidStatm{}, err
	}
	if bytesRead >= statmReadBufSize {
		return ProcPidStatm{}, fmt.Errorf("buffersize(%d) exhausted while reading File(%s)", statmReadBufSize, deviceInfoPath)
	}

	// copies into a string, so returning the buffer to the pool is safe
	statcontent := strings.Fields(string(data[:bytesRead]))

	info := ProcPidStatm{}
	info.Pid = asUint64(pid)
	info.Size = asUint64(statcontent[0])
	info.Resident = asUint64(statcontent[1])
	info.Shared = asUint64(statcontent[2])
	info.Text = asUint64(statcontent[3])
	info.Lib = asUint64(statcontent[4])
	info.Data = asUint64(statcontent[5])
	info.Dt = asUint64(statcontent[6])

	return info, nil
}
