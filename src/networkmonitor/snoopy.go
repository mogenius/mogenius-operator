package networkmonitor

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
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
}

type snoopyManager struct {
	logger *slog.Logger

	handlesLock *sync.RWMutex
	handles     map[ContainerId]*SnoopyHandle
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

	StartBytesLock    *sync.RWMutex
	IngressStartBytes map[InterfaceName]uint64
	EgressStartBytes  map[InterfaceName]uint64

	LastMetricsLock *sync.RWMutex
	LastMetrics     map[InterfaceName]SnoopyInterfaceMetrics

	Ctx    context.Context
	Cancel context.CancelFunc
	Cmd    *exec.Cmd

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
	Type      string `json:"type"`
	Interface string `json:"interface"`
	Ingress   struct {
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

func NewSnoopyManager(logger *slog.Logger) SnoopyManager {
	self := &snoopyManager{}

	self.logger = logger
	self.handlesLock = &sync.RWMutex{}
	self.handles = map[ContainerId]*SnoopyHandle{}

	return self
}

func (self *snoopyManager) Metrics() map[ContainerId]ContainerInfo {
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

// StartBytesLock    *sync.RWMutex
// IngressStartBytes map[InterfaceName]uint64
// EgressStartBytes  map[InterfaceName]uint64

// LastMetricsLock *sync.RWMutex
// LastMetrics     map[InterfaceName]SnoopyInterfaceMetrics

// Ctx    context.Context
// Cancel context.CancelFunc
// Cmd    *exec.Cmd

// Stdout chan SnoopyEvent
// Stderr chan SnoopyLogMessage

func (self *snoopyManager) Register(podNamespace string, podName string, containerId ContainerId, pid ProcessId) error {
	handle, err := self.AttachToPidNamespace(pid)
	if err != nil {
		return fmt.Errorf("failed to attach snoopy to containerId(%s) pid(%d): %v", containerId, pid, err)
	}

	handle.PodNamespace = podNamespace
	handle.PodName = podName

	go func() {
		for {
			select {
			case <-handle.Ctx.Done():
				break
			case logMessage := <-handle.Stderr:
				switch logMessage.Level {
				case "DEBUG", "INFO":
					self.logger.Debug("snoppy log message", "containerId", containerId, "pid", pid, "level", logMessage.Level, "target", logMessage.Target, "msg", logMessage.Message)
				case "WARN":
					self.logger.Warn("snoppy warning", "containerId", containerId, "pid", pid, "level", logMessage.Level, "target", logMessage.Target, "msg", logMessage.Message)
				case "ERROR":
					self.logger.Error("snoppy error", "containerId", containerId, "pid", pid, "level", logMessage.Level, "target", logMessage.Target, "msg", logMessage.Message)
				}
			case metrics := <-handle.Stdout:
				switch metrics.Type {
				case "InterfaceAdded":
					iface := metrics.InterfaceAdded.Interface.Name
					rxBytes := self.readRxBytesFromPidAndInterface(pid, iface)
					txBytes := self.readTxBytesFromPidAndInterface(pid, iface)
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
	handle := self.handles[containerId]
	delete(self.handles, containerId)
	self.handlesLock.Unlock()

	handle.Cancel()

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
	pidS := strconv.FormatUint(pid, 10)
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "nsenter", "--net=/proc/"+pidS+"/ns/net", "snoopy")

	var json = jsoniter.ConfigCompatibleWithStandardLibrary

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		cancel()
		return nil, err
	}

	snoopy := &SnoopyHandle{}
	snoopy.Ctx = ctx
	snoopy.Cancel = cancel
	snoopy.Cmd = cmd
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
	}()

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			output := scanner.Bytes()
			var msg SnoopyLogMessage
			err := json.Unmarshal(output, &msg)
			if err != nil {
				self.logger.Error("failed to parse snoopy log message", "message", output, "error", err)
				continue
			}
			snoopy.Stderr <- msg
		}
	}()

	return snoopy, nil
}
