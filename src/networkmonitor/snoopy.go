package networkmonitor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/containerenumerator"
	"mogenius-operator/src/shutdown"
	"os/exec"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"

	jsoniter "github.com/json-iterator/go"
)

type SnoopyManager interface {
	Run()
	Register(podInfo containerenumerator.PodInfo) []error
	Remove(podInfo containerenumerator.PodInfo) []error
	Metrics() map[containerenumerator.PodInfoIdentifier]ContainerInfo
	Status() SnoopyStatus
	SetArgs(args SnoopyArgs)
}

type snoopyManager struct {
	logger        *slog.Logger
	config        config.ConfigModule
	args          SnoopyArgs
	procPath      string
	snoopyBinName *string
	running       atomic.Bool

	statusEventTx chan SnoopyStatusEvent

	statusTx chan struct{}
	statusRx chan SnoopyStatus

	handlesLock *sync.RWMutex
	handles     map[containerenumerator.PodInfoIdentifier]*SnoopyHandle
}

type SnoopyArgs struct {
	NetworkDevicePollRate uint64
	MetricsRate           uint64
}

type SnoopyStatus struct {
	Initializing    []SnoopyStatusEventRegisterRequest `json:"initializing"`
	Failure         []SnoopyStatusEventRegisterFailure `json:"failures"`
	SnoopyProcesses []SnoopyStatusProcess              `json:"snoopy_processes"`
}

type SnoopyStatusProcess struct {
	Pid        ProcessId               `json:"pid"`
	Interfaces []SnoopyStatusInterface `json:"interfaces"`
}

type SnoopyStatusInterface struct {
	Name                  string `json:"name"`
	IngressImplementation string `json:"ingressImplementation"`
	EgressImplementation  string `json:"egressImplementation"`
}

type SnoopyStatusEventType = string

const (
	// mogenius-operator attempts to start a snoopy instance
	SnoopyStatusEventTypeRegisterRequest SnoopyStatusEventType = "register_request"
	// mogenius-operator successfully attached a snoopy instance to a linux network namespace
	SnoopyStatusEventTypeRegisterSuccess SnoopyStatusEventType = "register_failure"
	// mogenius-operator failed to attach a snoopy instance to a linux network namespace
	SnoopyStatusEventTypeRegisterFailure SnoopyStatusEventType = "register_success"
	// mogenius-operator stopped a snoopy instance it managed
	SnoopyStatusEventTypeRemove SnoopyStatusEventType = "remove"
	// mogenius-operator received an event message from a snoopy instance
	SnoopyStatusEventTypeSnoopyEvent SnoopyStatusEventType = "snoopy_event"
)

type SnoopyStatusEventRegisterRequest struct {
	PodNamespace string      `json:"podNamespace"`
	PodName      string      `json:"podName"`
	ContainerId  ContainerId `json:"containerId"`
	NsProcessPid ProcessId   `json:"NsProcessId"`
}

type SnoopyStatusEventRegisterSuccess struct {
	PodNamespace string
	PodName      string
	ContainerId  ContainerId
	NsProcessPid ProcessId
	SnoopyPid    ProcessId
}

type SnoopyStatusEventRegisterFailure struct {
	PodNamespace string      `json:"podNamespace"`
	PodName      string      `json:"podName"`
	ContainerId  ContainerId `json:"containerId"`
	NsProcessPid ProcessId   `json:"NsProcessId"`
	Err          error       `json:"error"`
}

type SnoopyStatusEventRemove struct {
	SnoopyPid ProcessId
}

type SnoopyStatusEventSnoopyEvent struct {
	SnoopyPid   ProcessId
	SnoopyEvent SnoopyEvent
}

type SnoopyStatusEvent struct {
	Type SnoopyStatusEventType

	RegisterRequest *SnoopyStatusEventRegisterRequest
	RegisterSuccess *SnoopyStatusEventRegisterSuccess
	RegisterFailure *SnoopyStatusEventRegisterFailure
	Remove          *SnoopyStatusEventRemove
	SnoopyEvent     *SnoopyStatusEventSnoopyEvent
}

type ContainerInfo struct {
	PodInfo containerenumerator.PodInfo
	Metrics map[InterfaceName]MetricSnapshot
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
	PodInfo   containerenumerator.PodInfo
	SnoopyPid ProcessId

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

type SnoopyEventType = string

const (
	SnoopyEventTypeInterfaceMetrics                 SnoopyEventType = "InterfaceMetrics"
	SnoopyEventTypeInterfaceChanged                 SnoopyEventType = "InterfaceChanged"
	SnoopyEventTypeInterfaceRemoved                 SnoopyEventType = "InterfaceRemoved"
	SnoopyEventTypeInterfaceAdded                   SnoopyEventType = "InterfaceAdded"
	SnoopyEventTypeInterfaceBpfInitialized          SnoopyEventType = "InterfaceBpfInitialized"
	SnoopyEventTypeInterfaceBpfInitializationFailed SnoopyEventType = "InterfaceBpfInitializationFailed"
)

type SnoopyEvent struct {
	Type SnoopyEventType

	InterfaceMetric                  *SnoopyInterfaceMetrics
	InterfaceAdded                   *SnoopyInterfaceAdded
	InterfaceRemoved                 *SnoopyInterfaceRemoved
	InterfaceChanged                 *SnoopyInterfaceChanged
	InterfaceBpfInitialized          *SnoopyInterfaceBpfInitialized
	InterfaceBpfInitializationFailed *SnoopyInterfaceBpfInitializationFailed
}

type SnoopyInterfaceBpfInitialized struct {
	Type                  string `json:"type"`
	Interface             string `json:"interface"`
	IngressImplementation string `json:"ingress_implementation"`
	EgressImplementation  string `json:"egress_implementation"`
}

type SnoopyInterfaceBpfInitializationFailed struct {
	Type      string `json:"type"`
	Interface string `json:"interface"`
	Error     string `json:"error"`
}

type SnoopyInterfaceMetrics struct {
	Type      string                        `json:"type"`
	Interface string                        `json:"interface"`
	Ingress   SnoopyInterfaceMetricsCounter `json:"ingress"`
	Egress    SnoopyInterfaceMetricsCounter `json:"egress"`
}

type SnoopyInterfaceMetricsCounter struct {
	Packets uint64 `json:"packets"`
	Bytes   uint64 `json:"bytes"`
}

func (self SnoopyInterfaceMetricsCounter) MarshalJSON() ([]byte, error) {
	type StringCounter struct {
		Packets string `json:"packets"`
		Bytes   string `json:"bytes"`
	}

	return json.Marshal(&StringCounter{
		Packets: strconv.FormatUint(self.Packets, 10),
		Bytes:   strconv.FormatUint(self.Bytes, 10),
	})
}

func (self *SnoopyInterfaceMetricsCounter) UnmarshalJSON(data []byte) error {
	type StringCounter struct {
		Packets string `json:"packets"`
		Bytes   string `json:"bytes"`
	}

	tmp := &StringCounter{}

	if err := json.Unmarshal(data, tmp); err != nil {
		return err
	}

	packets, err := strconv.ParseUint(tmp.Packets, 10, 64)
	if err != nil {
		return err
	}

	bytes, err := strconv.ParseUint(tmp.Bytes, 10, 64)
	if err != nil {
		return err
	}

	self.Packets = packets
	self.Bytes = bytes
	return nil
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
	self.procPath = config.Get("MO_HOST_PROC_PATH")
	self.handlesLock = &sync.RWMutex{}
	self.handles = map[containerenumerator.PodInfoIdentifier]*SnoopyHandle{}

	self.statusTx = make(chan struct{})
	self.statusRx = make(chan SnoopyStatus)

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
		self.logger.Error("failed to find nsenter in PATH")
		shutdown.SendShutdownSignal(true)
		select {}
	}

	return self
}

func (self *snoopyManager) Status() SnoopyStatus {
	assert.Assert(self.running.Load(), "snoopymanager has to be running before requesting its status")

	self.statusTx <- struct{}{}
	status := <-self.statusRx

	return status
}

func (self *snoopyManager) Run() {
	wasRunnning := self.running.Swap(true)
	assert.Assert(!wasRunnning, "leader elector should only be started once")

	self.statusEventTx = make(chan SnoopyStatusEvent)
	go self.startStatusEventHandler()
}

func (self *snoopyManager) startStatusEventHandler() {
	status := SnoopyStatus{
		Initializing:    []SnoopyStatusEventRegisterRequest{},
		Failure:         []SnoopyStatusEventRegisterFailure{},
		SnoopyProcesses: []SnoopyStatusProcess{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	shutdown.Add(cancel)
	for {
		select {
		case <-ctx.Done():
			return
		case <-self.statusTx:
			self.statusRx <- status
		case event := <-self.statusEventTx:
			switch event.Type {
			case SnoopyStatusEventTypeRegisterRequest:
				assert.Assert(event.RegisterRequest != nil, "event.RegisterRequest should be set", event)
				status.Initializing = append(status.Initializing, *event.RegisterRequest)
			case SnoopyStatusEventTypeRegisterSuccess:
				assert.Assert(event.RegisterSuccess != nil, "event.RegisterSuccess should be set", event)
				successEvent := *event.RegisterSuccess

				// remove the corresponding event from the initializing list
				initializingIdx := -1
				for idx, initEvent := range status.Initializing {
					if initEvent.ContainerId == successEvent.ContainerId && initEvent.NsProcessPid == successEvent.NsProcessPid && initEvent.PodName == successEvent.PodName && initEvent.PodNamespace == successEvent.PodNamespace {
						initializingIdx = idx
						break
					}
				}
				assert.Assert(initializingIdx >= 0, "a corresponding initializing event has to exist before success can happen")
				updatedInitializing := status.Initializing[:initializingIdx]
				updatedInitializing = append(updatedInitializing, status.Initializing[initializingIdx+1:]...)
				status.Initializing = updatedInitializing

				// add the process to the process list
				status.SnoopyProcesses = append(status.SnoopyProcesses, SnoopyStatusProcess{
					Pid:        ProcessId(successEvent.SnoopyPid),
					Interfaces: []SnoopyStatusInterface{},
				})
			case SnoopyStatusEventTypeRegisterFailure:
				assert.Assert(event.RegisterFailure != nil, "event.RegisterFailure should be set", event)
				failureEvent := *event.RegisterFailure

				// remove the corresponding event from the initializing list
				initializingIdx := -1
				for idx, initEvent := range status.Initializing {
					if initEvent.ContainerId == failureEvent.ContainerId && initEvent.NsProcessPid == failureEvent.NsProcessPid && initEvent.PodName == failureEvent.PodName && initEvent.PodNamespace == failureEvent.PodNamespace {
						initializingIdx = idx
						break
					}
				}
				assert.Assert(initializingIdx >= 0, "a corresponding initializing event has to exist before failure can happen")
				updatedInitializing := status.Initializing[:initializingIdx]
				updatedInitializing = append(updatedInitializing, status.Initializing[initializingIdx+1:]...)
				status.Initializing = updatedInitializing

				// add the failure to the failure list
				status.Failure = append(status.Failure, *event.RegisterFailure)
			case SnoopyStatusEventTypeRemove:
				assert.Assert(event.Remove != nil, "event.Remove should be set", event)
				removeEvent := *event.Remove

				// remove the process from the process list
				processIdx := -1
				for idx, snoopyProcess := range status.SnoopyProcesses {
					if removeEvent.SnoopyPid == snoopyProcess.Pid {
						processIdx = idx
						break
					}
				}
				assert.Assert(processIdx >= 0, "a corresponding initializing event has to exist before failure can happen")
				updatedProcesses := status.SnoopyProcesses[:processIdx]
				updatedProcesses = append(updatedProcesses, status.SnoopyProcesses[processIdx+1:]...)
				status.SnoopyProcesses = updatedProcesses
			case SnoopyStatusEventTypeSnoopyEvent:
				assert.Assert(event.SnoopyEvent != nil, "event.SnoopyEvent should be set", event)

				switch event.SnoopyEvent.SnoopyEvent.Type {
				case SnoopyEventTypeInterfaceChanged:
					// ignore
				case SnoopyEventTypeInterfaceMetrics:
					// ignore
				case SnoopyEventTypeInterfaceAdded:
					assert.Assert(
						event.SnoopyEvent.SnoopyEvent.InterfaceAdded != nil,
						"event.SnoopyEvent.SnoopyEvent.InterfaceAdded should be set",
						event,
					)
					snoopyEventInterfaceAdded := *event.SnoopyEvent.SnoopyEvent.InterfaceAdded
					var snoopyProcess *SnoopyStatusProcess
					for idx, process := range status.SnoopyProcesses {
						if process.Pid == event.SnoopyEvent.SnoopyPid {
							snoopyProcess = &status.SnoopyProcesses[idx]
							break
						}
					}
					if snoopyProcess == nil {
						continue
					}
					snoopyProcess.Interfaces = append(snoopyProcess.Interfaces, SnoopyStatusInterface{
						Name: snoopyEventInterfaceAdded.Interface.Name,
					})
				case SnoopyEventTypeInterfaceRemoved:
					assert.Assert(
						event.SnoopyEvent.SnoopyEvent.InterfaceRemoved != nil,
						"event.SnoopyEvent.SnoopyEvent.InterfaceRemoved should be set",
						event,
					)
					snoopyEventInterfaceRemoved := *event.SnoopyEvent.SnoopyEvent.InterfaceRemoved
					var snoopyProcess *SnoopyStatusProcess
					for idx, process := range status.SnoopyProcesses {
						if process.Pid == event.SnoopyEvent.SnoopyPid {
							snoopyProcess = &status.SnoopyProcesses[idx]
							break
						}
					}
					if snoopyProcess == nil {
						continue
					}
					deleteIdx := -1
					for idx, intf := range snoopyProcess.Interfaces {
						if snoopyEventInterfaceRemoved.Interface.Name == intf.Name {
							deleteIdx = idx
							break
						}
					}
					assert.Assert(
						deleteIdx >= 0,
						"the removed interface should be found in the running snoopy interface list",
						event.SnoopyEvent.SnoopyEvent,
						snoopyProcess,
					)
					cleanedInterfaces := append(snoopyProcess.Interfaces[:deleteIdx], snoopyProcess.Interfaces[deleteIdx+1:]...)
					snoopyProcess.Interfaces = cleanedInterfaces
				case SnoopyEventTypeInterfaceBpfInitialized:
					assert.Assert(
						event.SnoopyEvent.SnoopyEvent.InterfaceBpfInitialized != nil,
						"event.SnoopyEvent.SnoopyEvent.InterfaceBpfInitialized should be set",
						event,
					)
					payload := *event.SnoopyEvent.SnoopyEvent.InterfaceBpfInitialized
					var snoopyProcess *SnoopyStatusProcess
					for idx, process := range status.SnoopyProcesses {
						if process.Pid == event.SnoopyEvent.SnoopyPid {
							snoopyProcess = &status.SnoopyProcesses[idx]
							break
						}
					}
					if snoopyProcess == nil {
						continue
					}
					initializedIdx := -1
					for idx, intf := range snoopyProcess.Interfaces {
						if payload.Interface == intf.Name {
							initializedIdx = idx
							break
						}
					}
					assert.Assert(
						initializedIdx >= 0,
						"the initialized interface should be found in the running snoopy interface list",
						event.SnoopyEvent.SnoopyEvent,
						snoopyProcess,
					)
					snoopyProcess.Interfaces[initializedIdx].IngressImplementation = payload.IngressImplementation
					snoopyProcess.Interfaces[initializedIdx].EgressImplementation = payload.EgressImplementation
				case SnoopyEventTypeInterfaceBpfInitializationFailed:
					assert.Assert(
						event.SnoopyEvent.SnoopyEvent.InterfaceBpfInitializationFailed != nil,
						"event.SnoopyEvent.SnoopyEvent.InterfaceBpfInitializationFailed should be set",
						event,
					)
					payload := *event.SnoopyEvent.SnoopyEvent.InterfaceBpfInitializationFailed
					self.logger.Warn("failed to initialize eBPF module", "interface", payload.Interface, "error", payload.Error)
				default:
					assert.Assert(
						false,
						"Unhandled event.SnoopyEvent.SnoopyEvent.Type",
						event.SnoopyEvent.SnoopyEvent.Type,
						event,
					)
				}
			default:
				assert.Assert(
					false,
					"Unhandled event.Type",
					event.Type,
					event,
				)
			}
		}
	}
}

func (self *snoopyManager) Metrics() map[containerenumerator.PodInfoIdentifier]ContainerInfo {
	data := map[containerenumerator.PodInfoIdentifier]ContainerInfo{}

	self.handlesLock.RLock()
	podInfoIds := []containerenumerator.PodInfoIdentifier{}
	for podInfoId := range self.handles {
		podInfoIds = append(podInfoIds, podInfoId)
	}
	slices.Sort(podInfoIds)

	for _, podInfoId := range podInfoIds {
		handle := self.handles[podInfoId]
		containerInfo := ContainerInfo{}
		containerInfo.PodInfo = handle.PodInfo
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

		data[podInfoId] = containerInfo
	}
	self.handlesLock.RUnlock()

	return data
}

func (self *snoopyManager) SetArgs(args SnoopyArgs) {
	self.args = args
}

func (self *snoopyManager) Register(podInfo containerenumerator.PodInfo) []error {
	errors := []error{}
	for containerId, pid := range podInfo.ContainersWithFirstPid() {
		self.statusEventTx <- SnoopyStatusEvent{
			Type: SnoopyStatusEventTypeRegisterRequest,
			RegisterRequest: &SnoopyStatusEventRegisterRequest{
				podInfo.Namespace,
				podInfo.Name,
				containerId,
				pid,
			},
		}
		handle, err := self.attachToPidNamespace(pid)
		if err != nil {
			self.statusEventTx <- SnoopyStatusEvent{
				Type: SnoopyStatusEventTypeRegisterFailure,
				RegisterFailure: &SnoopyStatusEventRegisterFailure{
					podInfo.Namespace,
					podInfo.Name,
					containerId,
					pid,
					err,
				},
			}
			errors = append(errors, fmt.Errorf("failed to attach snoopy to containerId(%s) pid(%d): %v", containerId, pid, err))
			continue
		}
		self.statusEventTx <- SnoopyStatusEvent{
			Type: SnoopyStatusEventTypeRegisterSuccess,
			RegisterSuccess: &SnoopyStatusEventRegisterSuccess{
				podInfo.Namespace,
				podInfo.Name,
				containerId,
				pid,
				handle.SnoopyPid,
			},
		}

		handle.PodInfo = podInfo

		go func() {
			for {
				select {
				case <-handle.Ctx.Done():
					return
				case logMessage := <-handle.Stderr:
					switch logMessage.Level {
					case "DEBUG":
						self.logger.Debug("snoppy debug", "containerId", containerId, "snoopyPid", handle.SnoopyPid, "attachedProcessPid", pid, "level", logMessage.Level, "target", logMessage.Target, "msg", logMessage.Message)
					case "INFO":
						self.logger.Info("snoppy info", "containerId", containerId, "snoopyPid", handle.SnoopyPid, "attachedProcessPid", pid, "level", logMessage.Level, "target", logMessage.Target, "msg", logMessage.Message)
					case "WARN":
						self.logger.Warn("snoppy warning", "containerId", containerId, "snoopyPid", handle.SnoopyPid, "attachedProcessPid", pid, "level", logMessage.Level, "target", logMessage.Target, "msg", logMessage.Message)
					case "ERROR":
						self.logger.Error("snoppy error", "containerId", containerId, "snoopyPid", handle.SnoopyPid, "attachedProcessPid", pid, "level", logMessage.Level, "target", logMessage.Target, "msg", logMessage.Message)
					}
				case metrics := <-handle.Stdout:
					self.statusEventTx <- SnoopyStatusEvent{
						Type: SnoopyStatusEventTypeSnoopyEvent,
						SnoopyEvent: &SnoopyStatusEventSnoopyEvent{
							SnoopyPid:   handle.SnoopyPid,
							SnoopyEvent: metrics,
						},
					}
					switch metrics.Type {
					case SnoopyEventTypeInterfaceAdded:
						iface := metrics.InterfaceAdded.Interface.Name
						rxBytes := self.readRxBytesFromPidAndInterface(pid, iface)
						txBytes := self.readTxBytesFromPidAndInterface(pid, iface)
						handle.StartBytesLock.Lock()
						handle.IngressStartBytes[iface] = rxBytes
						handle.EgressStartBytes[iface] = txBytes
						handle.StartBytesLock.Unlock()
					case SnoopyEventTypeInterfaceRemoved:
						handle.StartBytesLock.Lock()
						delete(handle.IngressStartBytes, metrics.InterfaceRemoved.Interface.Name)
						delete(handle.EgressStartBytes, metrics.InterfaceRemoved.Interface.Name)
						handle.StartBytesLock.Unlock()
					case SnoopyEventTypeInterfaceChanged:
						// ignore
					case SnoopyEventTypeInterfaceMetrics:
						handle.LastMetricsLock.Lock()
						handle.LastMetrics[metrics.InterfaceMetric.Interface] = *metrics.InterfaceMetric
						handle.LastMetricsLock.Unlock()
					case SnoopyEventTypeInterfaceBpfInitialized:
						// ignore
					case SnoopyEventTypeInterfaceBpfInitializationFailed:
						// ignore
					default:
						self.logger.Warn("unknown metric type", "type", metrics.Type, "metric", metrics)
					}
				}
			}
		}()

		handleId := self.formatHandleId(podInfo.Namespace, podInfo.Name, containerId, pid)
		self.handlesLock.Lock()
		self.handles[handleId] = handle
		self.handlesLock.Unlock()
	}

	return errors
}

func (self *snoopyManager) formatHandleId(namespace string, name string, containerId string, pid ProcessId) string {
	return namespace + "/" + name + "/" + containerId + "/" + strconv.FormatUint(pid, 10)
}

func (self *snoopyManager) Remove(podInfo containerenumerator.PodInfo) []error {
	errors := []error{}

	self.handlesLock.Lock()
	defer self.handlesLock.Unlock()
	for containerId, pid := range podInfo.ContainersWithFirstPid() {
		handleId := self.formatHandleId(podInfo.Namespace, podInfo.Name, containerId, pid)
		handle, ok := self.handles[handleId]
		if ok {
			delete(self.handles, handleId)
			self.statusEventTx <- SnoopyStatusEvent{
				Type: SnoopyStatusEventTypeRemove,
				Remove: &SnoopyStatusEventRemove{
					SnoopyPid: handle.SnoopyPid,
				},
			}
			go func() {
				err := handle.Cmd.Process.Kill()
				if err != nil {
					self.logger.Debug("failed to kill process", "process", handle.Cmd.Process, "error", err)
					return
				}
				_, err = handle.Cmd.Process.Wait()
				if err != nil {
					self.logger.Debug("failed to wait for process", "process", handle.Cmd.Process, "error", err)
					return
				}
			}()
		}
	}

	return errors
}

func (self *snoopyManager) readRxBytesFromPidAndInterface(pid ProcessId, iface string) uint64 {
	pidS := strconv.FormatUint(pid, 10)
	infos, err := getNetworkInterfaceInfo(self.procPath, pidS)
	if err != nil {
		return 0
	}

	for _, info := range infos {
		if info.Interface == iface {
			return info.ReceiveBytes
		}
	}

	return 0
}

func (self *snoopyManager) readTxBytesFromPidAndInterface(pid ProcessId, iface string) uint64 {
	pidS := strconv.FormatUint(pid, 10)
	infos, err := getNetworkInterfaceInfo(self.procPath, pidS)
	if err != nil {
		return 0
	}

	for _, info := range infos {
		if info.Interface == iface {
			return info.TransmitBytes
		}
	}

	return 0
}

func (self *snoopyManager) attachToPidNamespace(pid ProcessId) (*SnoopyHandle, error) {
	assert.Assert(self.snoopyBinName != nil, "the binary path has to be found and set previously")

	pidS := strconv.FormatUint(pid, 10)
	cmd := exec.Command(
		"nsenter",
		"--net="+self.procPath+"/"+pidS+"/ns/net",
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
	snoopy.SnoopyPid = ProcessId(cmd.Process.Pid)
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
			case SnoopyEventTypeInterfaceAdded:
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
			case SnoopyEventTypeInterfaceRemoved:
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
			case SnoopyEventTypeInterfaceChanged:
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
			case SnoopyEventTypeInterfaceMetrics:
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
			case SnoopyEventTypeInterfaceBpfInitialized:
				var interfaceBpfInitialized SnoopyInterfaceBpfInitialized
				err := json.Unmarshal(output, &interfaceBpfInitialized)
				if err != nil {
					self.logger.Error("failed to parse `SnoopyInterfaceBpfInitialized`", "message", output, "error", err)
					continue
				}
				snoopy.Stdout <- SnoopyEvent{
					Type:                    interfaceBpfInitialized.Type,
					InterfaceBpfInitialized: &interfaceBpfInitialized,
				}
			case SnoopyEventTypeInterfaceBpfInitializationFailed:
				var interfaceBpfInitializationFailed SnoopyInterfaceBpfInitializationFailed
				err := json.Unmarshal(output, &interfaceBpfInitializationFailed)
				if err != nil {
					self.logger.Error("failed to parse `SnoopyInterfaceBpfInitialized`", "message", output, "error", err)
					continue
				}
				snoopy.Stdout <- SnoopyEvent{
					Type:                             interfaceBpfInitializationFailed.Type,
					InterfaceBpfInitializationFailed: &interfaceBpfInitializationFailed,
				}
			default:
				assert.Assert(
					false,
					"Unreachable",
					"all output types exposed by snoopy have to be explicitly handled",
					"if this crashes snoopy got a new feature and mogenius-operator has not been updated for it",
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
