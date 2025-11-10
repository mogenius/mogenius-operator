package rammonitor

import (
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
	ramUsageProcessesRx chan map[containerenumerator.ContainerId][]ProcPidStatm
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
	self.ramUsageProcessesRx = make(chan map[containerenumerator.ContainerId][]ProcPidStatm)

	return self
}

func (self *ramMonitor) startCollector() {
	wasRunning := self.running.Swap(true)
	if wasRunning {
		return
	}
	if runtime.GOOS != "linux" {
		return
	}
	go func() {
		nodeName := self.config.Get("OWN_NODE_NAME")
		assert.Assert(nodeName != "", "OWN_NODE_NAME has to be defined and non-empty", nodeName)

		globalMetricsUpdater := time.NewTicker(1 * time.Second)
		defer globalMetricsUpdater.Stop()

		processMetricsUpdater := time.NewTicker(1 * time.Second)
		defer processMetricsUpdater.Stop()

		globalMetrics := self.collectGlobalMetrics(nodeName)
		processMetrics := self.collectProcessMetrics()

		for {
			select {
			case <-globalMetricsUpdater.C:
				globalMetrics = self.collectGlobalMetrics(nodeName)
			case <-processMetricsUpdater.C:
				processMetrics = self.collectProcessMetrics()
			case <-self.ramUsageGlobalTx:
				self.ramUsageGlobalRx <- globalMetrics
			case <-self.ramUsageProcessesTx:
				self.ramUsageProcessesRx <- processMetrics
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

	return data
}

func (self *ramMonitor) RamUsageGlobal() RamMetrics {
	self.startCollector()

	self.ramUsageGlobalTx <- struct{}{}
	data := <-self.ramUsageGlobalRx

	return data
}

func (self *ramMonitor) collectProcessMetrics() map[containerenumerator.ContainerId][]ProcPidStatm {
	data := make(map[containerenumerator.ContainerId][]ProcPidStatm)
	containers := self.containerEnumerator.GetProcessesWithContainerIds()

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

	return data
}

func (self *ramMonitor) RamUsageProcesses() []PodRamStats {
	self.startCollector()

	self.ramUsageProcessesTx <- struct{}{}
	data := <-self.ramUsageProcessesRx

	pods := self.containerEnumerator.GetPodsWithContainerIds()

	stats := []PodRamStats{}
	for _, pod := range pods {
		for containerId := range pod.Containers {
			procPidStatm, ok := data[containerId]
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

	data, err := os.ReadFile(deviceInfoPath)
	if err != nil {
		return ProcPidStatm{}, err
	}

	statcontent := strings.Split(strings.TrimSpace(string(data)), " ")

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
