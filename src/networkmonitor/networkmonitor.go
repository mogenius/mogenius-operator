package networkmonitor

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/containerenumerator"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type NetworkMonitor interface {
	Run()
	Snoopy() SnoopyManager
	GetPodNetworkUsage() []PodNetworkStats
	// BTF is the format used by BPF modules to be loaded across most linux distributions.
	// Without BTF support we are not able to use BPF modules to implement features. This
	// means limited feature availability. In some cases we might implement slower
	// fallback solutions. In others it is impossible to support features if this is not
	// available.
	BtfAvailable() bool
}

type networkMonitor struct {
	logger *slog.Logger
	ctx    context.Context
	cancel context.CancelFunc
	config config.ConfigModule

	collectorStarted atomic.Bool
	procFsMountPath  string
	cne              containerenumerator.ContainerEnumerator
	snoopy           SnoopyManager

	networkUsageTx chan struct{}
	networkUsageRx chan []PodNetworkStats
}

type ContainerId = string
type PodName = string
type ProcessId = uint64
type InterfaceName = string

func NewNetworkMonitor(
	logger *slog.Logger,
	config config.ConfigModule,
	containerEnumerator containerenumerator.ContainerEnumerator,
	procFsMountPath string,
) NetworkMonitor {
	self := &networkMonitor{}

	self.logger = logger
	self.config = config
	self.collectorStarted = atomic.Bool{}
	ctx, cancel := context.WithCancel(context.Background())
	self.ctx = ctx
	self.cancel = cancel
	cne := containerEnumerator
	self.cne = cne

	switch config.Get("MO_SNOOPY_IMPLEMENTATION") {
	case "auto":
		if self.BtfAvailable() {
			self.snoopy = NewSnoopyManager(self.logger.With("scope", "snoopy-manager"), config)
		} else {
			self.snoopy = NewNetworkStatsReader(self.logger.With("scope", "network-stats-reader"), cne, procFsMountPath)
		}
	case "snoopy":
		self.snoopy = NewSnoopyManager(self.logger.With("scope", "snoopy-manager"), config)
	case "procdev":
		self.snoopy = NewNetworkStatsReader(self.logger.With("scope", "network-stats-reader"), cne, procFsMountPath)
	default:
		assert.Assert(false, "UNREACHABLE", "config parser should not let any unexpected variant pass", config.Get("MO_SNOOPY_IMPLEMENTATION"))
	}

	self.procFsMountPath = procFsMountPath
	self.networkUsageTx = make(chan struct{})
	self.networkUsageRx = make(chan []PodNetworkStats)

	return self
}

func (self *networkMonitor) Run() {
	wasStarted := self.collectorStarted.Swap(true)
	if wasStarted {
		return
	}

	self.snoopy.Run()

	go func() {
		podInfoList := self.cne.GetPodsWithContainerIds()

		for _, podInfo := range podInfoList {
			errs := self.snoopy.Register(podInfo)
			if len(errs) > 0 {
				self.logger.Error("failed to register snoopy", "podInfo", podInfo, "error", errs)
				continue
			}
		}

		// get initial collected stats
		var metrics map[ContainerId]ContainerInfo
		var collectedStats []PodNetworkStats
		metrics = self.snoopy.Metrics()
		collectedStats = self.metricsToPodstats(metrics)

		// timers
		updatePodAndContainersTicker := time.NewTicker(5 * time.Second)
		defer updatePodAndContainersTicker.Stop()
		updateCollectedStatsTicker := time.NewTicker(1 * time.Second)
		defer updateCollectedStatsTicker.Stop()

		// enter update loop
		for {
			select {
			case <-self.ctx.Done():
				return
			case <-updatePodAndContainersTicker.C:
				// get a new list of all pods and containers on the current node
				nextPodInfoList := self.cne.GetPodsWithContainerIds()

				// build lookup maps for O(n) comparisons instead of O(n²) nested loops
				oldByID := make(map[containerenumerator.PodInfoIdentifier]containerenumerator.PodInfo, len(podInfoList))
				for _, p := range podInfoList {
					oldByID[p.NamespaceAndName()] = p
				}
				nextByID := make(map[containerenumerator.PodInfoIdentifier]containerenumerator.PodInfo, len(nextPodInfoList))
				for _, p := range nextPodInfoList {
					nextByID[p.NamespaceAndName()] = p
				}

				// unregister removed pods
				for id, podInfo := range oldByID {
					if _, exists := nextByID[id]; !exists {
						self.logger.Info("Remove Pod", "podInfo", podInfo)
						errs := self.snoopy.Remove(podInfo)
						if len(errs) > 0 {
							self.logger.Error("failed to remove old pod", "id", id, "errors", errs)
						}
					}
				}

				// register new pods and update changed pods
				for id, nextPodInfo := range nextByID {
					if podInfo, exists := oldByID[id]; !exists {
						self.logger.Info("Register Pod", "nextPodInfo", nextPodInfo)
						errs := self.snoopy.Register(nextPodInfo)
						if len(errs) > 0 {
							self.logger.Error("failed to register new pod", "id", id, "errors", errs)
						}
					} else if !podInfo.Equals(&nextPodInfo) {
						self.logger.Info("Update Pod", "podInfo", podInfo, "nextPodInfo", nextPodInfo)
						errs := self.snoopy.Remove(podInfo)
						if len(errs) > 0 {
							self.logger.Error("failed to remove old pod", "id", id, "errors", errs)
						}
						errs = self.snoopy.Register(nextPodInfo)
						if len(errs) > 0 {
							self.logger.Error("failed to register new pod", "id", id, "errors", errs)
						}
					}
				}

				// set the new podList as active podList
				podInfoList = nextPodInfoList

			case <-updateCollectedStatsTicker.C:
				metrics = self.snoopy.Metrics()
				collectedStats = self.metricsToPodstats(metrics)
			case <-self.networkUsageTx:
				self.networkUsageRx <- collectedStats
			}
		}
	}()
}

func (self *networkMonitor) BtfAvailable() bool {
	_, err := os.Stat("/sys/kernel/btf")
	return !errors.Is(err, os.ErrNotExist)
}

func (self *networkMonitor) Snoopy() SnoopyManager {
	return self.snoopy
}

func (self *networkMonitor) metricsToPodstats(
	metrics map[ContainerId]ContainerInfo,
) []PodNetworkStats {
	data := []PodNetworkStats{}

	containerIds := []ContainerId{}
	for containerId := range metrics {
		containerIds = append(containerIds, containerId)
	}
	slices.Sort(containerIds)

	for _, containerId := range containerIds {
		containerInfo := metrics[containerId]

		interfaceNames := []InterfaceName{}
		for interfaceName := range containerInfo.Metrics {
			interfaceNames = append(interfaceNames, interfaceName)
		}
		slices.Sort(interfaceNames)

		// Aggregate all interfaces for this container into a single entry.
		// Creating one entry per interface caused cross-interface delta calculations
		// in AddInterfaceStatsToDb because lastStatsMap is keyed by pod name only.
		podNetworkStat := PodNetworkStats{
			Pod:       containerInfo.PodInfo.Name,
			Namespace: containerInfo.PodInfo.Namespace,
			CreatedAt: time.Now(),
		}
		for _, interfaceName := range interfaceNames {
			iMetrics := containerInfo.Metrics[interfaceName]
			// filter blacklisted interface names e.g. loopback
			if slices.Contains([]string{"lo"}, interfaceName) {
				continue
			}
			podNetworkStat.ReceivedPackets += iMetrics.Ingress.Packets
			podNetworkStat.ReceivedBytes += iMetrics.Ingress.Bytes
			podNetworkStat.ReceivedStartBytes += iMetrics.Ingress.StartBytes
			podNetworkStat.TransmitPackets += iMetrics.Egress.Packets
			podNetworkStat.TransmitBytes += iMetrics.Egress.Bytes
			podNetworkStat.TransmitStartBytes += iMetrics.Egress.StartBytes
		}
		// filter empty metrics (if all observed values are 0)
		if podNetworkStat.ReceivedPackets == 0 && podNetworkStat.ReceivedBytes == 0 &&
			podNetworkStat.TransmitPackets == 0 && podNetworkStat.TransmitBytes == 0 {
			continue
		}
		data = append(data, podNetworkStat)
	}

	return data
}

func (self *networkMonitor) GetPodNetworkUsage() []PodNetworkStats {
	select {
	case <-self.ctx.Done():
		self.logger.Warn("requested metrics from network monitor after it was closed")
		return []PodNetworkStats{}
	case self.networkUsageTx <- struct{}{}:
		select {
		case <-self.ctx.Done():
			self.logger.Warn("requested metrics from network monitor after it was closed")
			return []PodNetworkStats{}
		case result := <-self.networkUsageRx:
			return result
		}
	}
}

type PodNetworkStats struct {
	// Ip                 string    `json:"ip"`
	Pod       string `json:"pod"`
	Namespace string `json:"namespace"`
	// Interface          string    `json:"interface"`
	ReceivedPackets    uint64 `json:"receivedPackets"`
	ReceivedBytes      uint64 `json:"receivedBytes"`
	ReceivedStartBytes uint64 `json:"receivedStartBytes"`
	TransmitPackets    uint64 `json:"transmitPackets"`
	TransmitBytes      uint64 `json:"transmitBytes"`
	TransmitStartBytes uint64 `json:"transmitStartBytes"`
	// StartTime          time.Time `json:"startTime"` // start time of the Interface/Pod
	CreatedAt time.Time `json:"createdAt"` // when the entry was written into the storage <- timestamp of write to redis
}

type KernelNetworkInterfaceInfo struct {
	Interface          string
	ReceiveBytes       uint64
	ReceivePackets     uint64
	ReceiveErrs        uint64
	ReceiveDrop        uint64
	ReceiveFifo        uint64
	ReceiveFrame       uint64
	ReceiveCompressed  uint64
	ReceiveMulticast   uint64
	TransmitBytes      uint64
	TransmitPackets    uint64
	TransmitErrs       uint64
	TransmitDrop       uint64
	TransmitFifo       uint64
	TransmitColls      uint64
	TransmitCarrier    uint64
	TransmitCompressed uint64
}

// read and parse `$procPath/$pid/net/dev` to get interface information from the kernel
func getNetworkInterfaceInfo(procPath string, pid string) ([]KernelNetworkInterfaceInfo, error) {
	// File Format of `/proc/$pid/net/dev`
	// ===================================
	//
	// ```
	// Inter-|   Receive                                                |  Transmit
	// face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
	//    lo:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
	//  eth0:   37252     547    0    0    0     0          0         0     1802      25    0    0    0     0       0          0
	// ```
	//
	// Parsing Rules
	// =============
	//
	// - the first 2 lines have to be skipped
	// - spaces have to be skipped
	// - there are 17 fields in a fixed order
	// - first field is a string
	// - every other field is a number

	processPath := filepath.Join(procPath, pid)
	deviceInfoPath := filepath.Join(processPath, "net", "dev")
	if _, err := os.Stat(processPath); err != nil {
		return []KernelNetworkInterfaceInfo{}, err
	}

	file, err := os.Open(deviceInfoPath)
	if err != nil {
		return []KernelNetworkInterfaceInfo{}, err
	}
	defer file.Close()

	toUint64 := func(data string) uint64 {
		val, err := strconv.ParseUint(data, 10, 64)
		assert.Assert(err == nil, "infallible conversion", err)

		return val
	}

	interfaceInfos := []KernelNetworkInterfaceInfo{}

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		if lineNumber == 1 || lineNumber == 2 {
			continue
		}

		tokens := strings.Fields(scanner.Text())

		assert.Assert(len(tokens) == 17, "line should contain exactly 17 tokens", tokens)

		tokens[0] = strings.TrimSuffix(tokens[0], ":")
		interfaceInfos = append(interfaceInfos, KernelNetworkInterfaceInfo{
			Interface:          tokens[0],
			ReceiveBytes:       toUint64(tokens[1]),
			ReceivePackets:     toUint64(tokens[2]),
			ReceiveErrs:        toUint64(tokens[3]),
			ReceiveDrop:        toUint64(tokens[4]),
			ReceiveFifo:        toUint64(tokens[5]),
			ReceiveFrame:       toUint64(tokens[6]),
			ReceiveCompressed:  toUint64(tokens[7]),
			ReceiveMulticast:   toUint64(tokens[8]),
			TransmitBytes:      toUint64(tokens[9]),
			TransmitPackets:    toUint64(tokens[10]),
			TransmitErrs:       toUint64(tokens[11]),
			TransmitDrop:       toUint64(tokens[12]),
			TransmitFifo:       toUint64(tokens[13]),
			TransmitColls:      toUint64(tokens[14]),
			TransmitCarrier:    toUint64(tokens[15]),
			TransmitCompressed: toUint64(tokens[16]),
		})
	}

	return interfaceInfos, nil
}
