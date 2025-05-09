package networkmonitor

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/shutdown"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"

	jsoniter "github.com/json-iterator/go"
)

type SnoopyManager interface {
	// containerId: the primary id for which network interfaces are watched
	// pid: a process id for initially connecting to the containerId -> if it dies snoopy stays connected to the containerId namespace
	Register(podNamespace string, podName string, containerId ContainerId, pid ProcessId) error

	Remove(containerId ContainerId) error

	Metrics() map[ContainerId]ContainerInfo

	SetArgs(args SnoopyArgs)
}

type snoopyManager struct {
	logger        *slog.Logger
	config        config.ConfigModule
	args          SnoopyArgs
	snoopyBinName *string

	handlesLock *sync.RWMutex
	handles     map[ContainerId]*SnoopyHandle
}

type SnoopyArgs struct {
	NetworkDevicePollRate uint64
	MetricsRate           uint64
}

type ContainerInfo struct {
	PodName      string
	PodNamespace string
	Metrics      map[InterfaceName]MetricSnapshot
}

type MetricSnapshot struct {
	Ingress struct {
		StartBytes uint64
		Packets    uint64
		Bytes      uint64
	}
	Egress struct {
		StartBytes uint64
		Packets    uint64
		Bytes      uint64
	}
}

type SnoopyLogMessage struct {
	Level   string `json:"level"`
	Target  string `json:"target"`
	Message string `json:"message"`
}

type SnoopyHandle struct {
	PodNamespace string
	PodName      string
	SnoopyPid    int

	StartBytesLock    *sync.RWMutex
	IngressStartBytes map[InterfaceName]uint64
	EgressStartBytes  map[InterfaceName]uint64

	LastMetricsLock *sync.RWMutex
	LastMetrics     map[InterfaceName]SnoopyInterfaceMetrics

	Cmd *exec.Cmd
	Ctx context.Context

	Stdout chan SnoopyEvent
	Stderr chan SnoopyLogMessage
}

type SnoopyOutput struct {
	Metric     *SnoopyEvent
	LogMessage *SnoopyLogMessage
}

type SnoopyEvent struct {
	Type string

	InterfaceMetric  *SnoopyInterfaceMetrics
	InterfaceAdded   *SnoopyInterfaceAdded
	InterfaceRemoved *SnoopyInterfaceRemoved
	InterfaceChanged *SnoopyInterfaceChanged
}

type SnoopyInterfaceMetrics struct {
	Type                  string `json:"type"`
	Interface             string `json:"interface"`
	IngressImplementation string `json:"ingress_implementation"`
	EgressImplementation  string `json:"egress_implementation"`
	Ingress               struct {
		Packets uint64 `json:"packets"`
		Bytes   uint64 `json:"bytes"`
	} `json:"ingress"`
	Egress struct {
		Packets uint64 `json:"packets"`
		Bytes   uint64 `json:"bytes"`
	} `json:"egress"`
}

type SnoopyInterfaceAdded struct {
	Type      string          `json:"type"`
	Interface SnoopyInterface `json:"interface"`
}

type SnoopyInterfaceRemoved struct {
	Type      string          `json:"type"`
	Interface SnoopyInterface `json:"interface"`
}

type SnoopyInterfaceChanged struct {
	Type     string          `json:"type"`
	Previous SnoopyInterface `json:"previous"`
	New      SnoopyInterface `json:"new"`
}

type SnoopyInterface struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Index       uint64   `json:"index"`
	Mac         *string  `json:"mac"`
	Ips         []string `json:"ips"`
	Flags       uint64   `json:"flags"`
}

func NewSnoopyManager(logger *slog.Logger, config config.ConfigModule) SnoopyManager {
	self := &snoopyManager{}

	self.logger = logger
	self.config = config
	self.handlesLock = &sync.RWMutex{}
	self.handles = map[ContainerId]*SnoopyHandle{}

	self.args = SnoopyArgs{}
	self.args.MetricsRate = 2000           // read network metrics from BPF every 2 seconds
	self.args.NetworkDevicePollRate = 1000 // update network devices list once per second

	// there are multiple possible names for this binary due to how the project was created
	// all possible binary names are lookup up and the first hit is used
	snoopyNames := []string{"snoopy", "mogenius-snoopy"}
	for _, name := range snoopyNames {
		binPath, err := exec.LookPath(name)
		if err == nil {
			self.snoopyBinName = &binPath
			break
		}
	}

	if self.snoopyBinName == nil {
		self.logger.Error("failed to find snoopy in PATH")
		shutdown.SendShutdownSignal(true)
		select {}
	}

	_, err := exec.LookPath("nsenter")
	if err != nil {
		self.logger.Error("failed to find snoopy in PATH")
		shutdown.SendShutdownSignal(true)
		select {}
	}

	return self
}

// func (self *snoopyManager) pids() []int {
// 	self.handlesLock.RLock()
// 	defer self.handlesLock.RUnlock()

// 	pids := []int{}

// 	for _, handle := range self.handles {
// 		pids = append(pids, handle.SnoopyPid)
// 	}
// 	slices.Sort(pids)

// 	return pids
// }

func (self *snoopyManager) Metrics() map[ContainerId]ContainerInfo {
	// fmt.Printf("%#v\n", self.pids())
	data := map[ContainerId]ContainerInfo{}

	self.handlesLock.RLock()
	containerIds := []ContainerId{}
	for containerId := range self.handles {
		containerIds = append(containerIds, containerId)
	}
	slices.Sort(containerIds)

	for _, containerId := range containerIds {
		handle := self.handles[containerId]
		containerInfo := ContainerInfo{}
		containerInfo.PodNamespace = handle.PodNamespace
		containerInfo.PodName = handle.PodName
		containerInfo.Metrics = map[InterfaceName]MetricSnapshot{}

		interfaceNames := []InterfaceName{}
		for interfaceName := range handle.IngressStartBytes {
			interfaceNames = append(interfaceNames, interfaceName)
		}
		slices.Sort(interfaceNames)

		for _, interfaceName := range interfaceNames {
			interfaceInfo := MetricSnapshot{}

			handle.StartBytesLock.RLock()
			interfaceInfo.Ingress.StartBytes = handle.IngressStartBytes[interfaceName]
			interfaceInfo.Egress.StartBytes = handle.EgressStartBytes[interfaceName]
			handle.StartBytesLock.RUnlock()

			handle.LastMetricsLock.RLock()
			interfaceInfo.Ingress.Bytes = handle.LastMetrics[interfaceName].Ingress.Bytes
			interfaceInfo.Ingress.Packets = handle.LastMetrics[interfaceName].Ingress.Packets
			interfaceInfo.Egress.Bytes = handle.LastMetrics[interfaceName].Egress.Bytes
			interfaceInfo.Egress.Packets = handle.LastMetrics[interfaceName].Egress.Packets
			handle.LastMetricsLock.RUnlock()

			containerInfo.Metrics[interfaceName] = interfaceInfo
		}

		data[containerId] = containerInfo
	}
	self.handlesLock.RUnlock()

	return data
}

func (self *snoopyManager) SetArgs(args SnoopyArgs) {
	self.args = args
}

func (self *snoopyManager) Register(podNamespace string, podName string, containerId ContainerId, nsProcessPid ProcessId) error {
	handle, err := self.AttachToPidNamespace(nsProcessPid)
	if err != nil {
		return fmt.Errorf("failed to attach snoopy to containerId(%s) pid(%d): %v", containerId, nsProcessPid, err)
	}

	handle.PodNamespace = podNamespace
	handle.PodName = podName

	go func() {
		for {
			select {
			case <-handle.Ctx.Done():
				return
			case logMessage := <-handle.Stderr:
				switch logMessage.Level {
				case "DEBUG":
					self.logger.Debug("snoppy debug", "containerId", containerId, "snoopyPid", handle.SnoopyPid, "attachedProcessPid", nsProcessPid, "level", logMessage.Level, "target", logMessage.Target, "msg", logMessage.Message)
				case "INFO":
					self.logger.Info("snoppy info", "containerId", containerId, "snoopyPid", handle.SnoopyPid, "attachedProcessPid", nsProcessPid, "level", logMessage.Level, "target", logMessage.Target, "msg", logMessage.Message)
				case "WARN":
					self.logger.Warn("snoppy warning", "containerId", containerId, "snoopyPid", handle.SnoopyPid, "attachedProcessPid", nsProcessPid, "level", logMessage.Level, "target", logMessage.Target, "msg", logMessage.Message)
				case "ERROR":
					self.logger.Error("snoppy error", "containerId", containerId, "snoopyPid", handle.SnoopyPid, "attachedProcessPid", nsProcessPid, "level", logMessage.Level, "target", logMessage.Target, "msg", logMessage.Message)
				}
			case metrics := <-handle.Stdout:
				switch metrics.Type {
				case "InterfaceAdded":
					iface := metrics.InterfaceAdded.Interface.Name
					rxBytes := self.readRxBytesFromPidAndInterface(nsProcessPid, iface)
					txBytes := self.readTxBytesFromPidAndInterface(nsProcessPid, iface)
					handle.StartBytesLock.Lock()
					handle.IngressStartBytes[iface] = rxBytes
					handle.EgressStartBytes[iface] = txBytes
					handle.StartBytesLock.Unlock()
				case "InterfaceRemoved":
					handle.StartBytesLock.Lock()
					delete(handle.IngressStartBytes, metrics.InterfaceRemoved.Interface.Name)
					delete(handle.EgressStartBytes, metrics.InterfaceRemoved.Interface.Name)
					handle.StartBytesLock.Unlock()
				case "InterfaceChanged":
					// ignore
				case "InterfaceMetrics":
					handle.LastMetricsLock.Lock()
					handle.LastMetrics[metrics.InterfaceMetric.Interface] = *metrics.InterfaceMetric
					handle.LastMetricsLock.Unlock()
				default:
					self.logger.Warn("unknown metric type", "type", metrics.Type, "metric", metrics)
				}
			}
		}
	}()

	self.handlesLock.Lock()
	self.handles[containerId] = handle
	self.handlesLock.Unlock()

	return nil
}

func (self *snoopyManager) Remove(containerId ContainerId) error {
	self.handlesLock.Lock()
	defer self.handlesLock.Unlock()
	handle, ok := self.handles[containerId]
	if ok {
		delete(self.handles, containerId)
		self.logger.Info("snoopy.Remove", "podNamespace", handle.PodNamespace, "podName", handle.PodName, "containerId", containerId)
		go func() {
			handle.Cmd.Process.Kill()
			handle.Cmd.Process.Wait()
		}()
	}

	return nil
}

func (self *snoopyManager) readRxBytesFromPidAndInterface(pid ProcessId, iface string) uint64 {
	pidS := strconv.FormatUint(pid, 10)
	cmd := exec.Command("nsenter", "--mount=/proc/"+pidS+"/ns/mnt", "cat", "/sys/class/net/"+iface+"/statistics/rx_bytes")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	numberString := strings.TrimSpace(string(output))
	rxBytes, err := strconv.ParseUint(numberString, 10, 64)
	if err != nil {
		return 0
	}

	return rxBytes
}

func (self *snoopyManager) readTxBytesFromPidAndInterface(pid ProcessId, iface string) uint64 {
	pidS := strconv.FormatUint(pid, 10)
	cmd := exec.Command("nsenter", "--mount=/proc/"+pidS+"/ns/mnt", "cat", "/sys/class/net/"+iface+"/statistics/tx_bytes")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	numberString := strings.TrimSpace(string(output))
	txBytes, err := strconv.ParseUint(numberString, 10, 64)
	if err != nil {
		return 0
	}

	return txBytes
}

func (self *snoopyManager) AttachToPidNamespace(pid ProcessId) (*SnoopyHandle, error) {
	assert.Assert(self.snoopyBinName != nil, "the binary path has to be found and set previously")

	pidS := strconv.FormatUint(pid, 10)
	cmd := exec.Command(
		"nsenter",
		"--net="+self.config.Get("MO_HOST_PROC_PATH")+"/"+pidS+"/ns/net",
		"--",
		*self.snoopyBinName,
		"--metrics-rate",
		strconv.FormatUint(self.args.MetricsRate, 10),
		"--network-device-poll-rate",
		strconv.FormatUint(self.args.NetworkDevicePollRate, 10),
	)

	var json = jsoniter.ConfigCompatibleWithStandardLibrary

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	snoopy := &SnoopyHandle{}
	snoopy.Cmd = cmd
	snoopy.SnoopyPid = cmd.Process.Pid
	snoopy.Ctx = ctx
	snoopy.Stdout = make(chan SnoopyEvent)
	snoopy.Stderr = make(chan SnoopyLogMessage)
	snoopy.StartBytesLock = &sync.RWMutex{}
	snoopy.IngressStartBytes = map[InterfaceName]uint64{}
	snoopy.EgressStartBytes = map[InterfaceName]uint64{}
	snoopy.LastMetricsLock = &sync.RWMutex{}
	snoopy.LastMetrics = map[InterfaceName]SnoopyInterfaceMetrics{}

	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			output := scanner.Bytes()
			var parsedOutputType struct {
				Type string `json:"type"`
			}
			err := json.Unmarshal(output, &parsedOutputType)
			if err != nil {
				self.logger.Error("failed to parse snoopy metrics message", "message", output, "error", err)
				continue
			}
			switch parsedOutputType.Type {
			case "InterfaceAdded":
				var data SnoopyInterfaceAdded
				err := json.Unmarshal(output, &data)
				if err != nil {
					self.logger.Error("failed to parse `SnoopyInterfaceAdded`", "message", output, "error", err)
					continue
				}
				snoopy.Stdout <- SnoopyEvent{
					Type:           data.Type,
					InterfaceAdded: &data,
				}
			case "InterfaceRemoved":
				var data SnoopyInterfaceRemoved
				err := json.Unmarshal(output, &data)
				if err != nil {
					self.logger.Error("failed to parse `SnoopyInterfaceRemoved`", "message", output, "error", err)
					continue
				}
				snoopy.Stdout <- SnoopyEvent{
					Type:             data.Type,
					InterfaceRemoved: &data,
				}
			case "InterfaceChanged":
				var data SnoopyInterfaceChanged
				err := json.Unmarshal(output, &data)
				if err != nil {
					self.logger.Error("failed to parse `SnoopyInterfaceChanged`", "message", output, "error", err)
					continue
				}
				snoopy.Stdout <- SnoopyEvent{
					Type:             data.Type,
					InterfaceChanged: &data,
				}
			case "InterfaceMetrics":
				var interfaceMetrics SnoopyInterfaceMetrics
				err := json.Unmarshal(output, &interfaceMetrics)
				if err != nil {
					self.logger.Error("failed to parse `SnoopyInterfaceMetrics`", "message", output, "error", err)
					continue
				}
				snoopy.Stdout <- SnoopyEvent{
					Type:            interfaceMetrics.Type,
					InterfaceMetric: &interfaceMetrics,
				}
			default:
				assert.Assert(
					false,
					"Unreachable",
					"all output types exposed by snoopy have to be explicitly handled",
					"if this crashes snoopy got a new feature and mogenius-k8s-manager has not been updated for it",
					parsedOutputType.Type,
				)
			}
		}
		cancel()
	}()

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			output := scanner.Bytes()
			if bytes.HasPrefix(output, []byte("nsenter")) {
				self.logger.Error("nsenter failed to execute snoopy", "error", string(output))
				continue
			}
			var msg SnoopyLogMessage
			err := json.Unmarshal(output, &msg)
			if err != nil {
				self.logger.Error("failed to parse snoopy log message", "message", string(output), "error", err)
				continue
			}
			snoopy.Stderr <- msg
		}
	}()

	return snoopy, nil
}
