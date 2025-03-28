package networkmonitor

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/k8sclient"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NetworkMonitor interface {
	Run()
	GetPodNetworkUsage() []PodNetworkStats
}

type networkMonitor struct {
	logger         *slog.Logger
	clientProvider k8sclient.K8sClientProvider
	ctx            context.Context
	cancel         context.CancelFunc
	config         config.ConfigModule

	collectorStarted atomic.Bool
	procFsMountPath  string
	cne              ContainerNetworkEnumerator
	ebpfApi          EbpfApi

	networkUsageTx chan struct{}
	networkUsageRx chan []PodNetworkStats
}

func NewNetworkMonitor(logger *slog.Logger, config config.ConfigModule, clientProvider k8sclient.K8sClientProvider, procFsMountPath string) NetworkMonitor {
	self := &networkMonitor{}

	self.logger = logger
	self.config = config
	self.clientProvider = clientProvider
	self.collectorStarted = atomic.Bool{}
	ctx, cancel := context.WithCancel(context.Background())
	self.ctx = ctx
	self.cancel = cancel
	self.cne = NewContainerNetworkEnumerator(logger.With("scope", "network-enumerator"))
	self.ebpfApi = NewEbpfApi(self.logger.With("scope", "ebpf"))
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

	go func() {
		defer self.cancel()

		updateDevicesTicker := time.NewTicker(30 * time.Second)
		defer updateDevicesTicker.Stop()

		updateDataTicker := time.NewTicker(1 * time.Second)
		defer updateDevicesTicker.Stop()

		// holds the context of all network interfaces which are being watched
		// the list has to be updated regularly for:
		// - deleted interfaces where the handled is not valid anymore
		// - added interfaces where new handles have to be created
		ebpfDataHandles := map[string]ebpfCounterHandle{}
		defer func() {
			handles := []string{}
			for iName := range ebpfDataHandles {
				handles = append(handles, iName)
			}
			for _, iName := range handles {
				ebpfDataHandles[iName].cancel()
				delete(ebpfDataHandles, iName)
			}
		}()

		podList := &v1.PodList{}
		startBytes := map[InterfaceName][2]uint64{}
		fieldSelector := "metadata.namespace!=kube-system"
		ownNodeName := self.config.Get("OWN_NODE_NAME")
		if ownNodeName != "" {
			fieldSelector = fmt.Sprintf("metadata.namespace!=kube-system,spec.nodeName=%s", ownNodeName)
		}

		// init
		networkInterfaceMap := self.cne.List(self.procFsMountPath)
		ebpfDataHandles = self.updateEbpfDataHandles(&networkInterfaceMap, ebpfDataHandles)
		podList = self.updatePodList(fieldSelector)
		collectedStats := self.updateCollectedStats(&networkInterfaceMap, &ebpfDataHandles, &podList, &startBytes)

		// loop
		for {
			select {
			case <-self.ctx.Done():
				return
			case <-updateDevicesTicker.C:
				networkInterfaceMap = self.cne.List(self.procFsMountPath)
				ebpfDataHandles = self.updateEbpfDataHandles(&networkInterfaceMap, ebpfDataHandles)
				podList = self.updatePodList(fieldSelector)
			case <-updateDataTicker.C:
				collectedStats = self.updateCollectedStats(&networkInterfaceMap, &ebpfDataHandles, &podList, &startBytes)
			case <-self.networkUsageTx:
				self.networkUsageRx <- collectedStats
			}
		}
	}()
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

type ebpfCounterHandle struct {
	dataChan chan CountState
	ctx      context.Context
	cancel   context.CancelFunc
}

//nolint:govet
func (self *networkMonitor) updateEbpfDataHandles(
	networkInterfaceMap *map[InterfaceName]InterfaceDescription,
	dataHandles map[string]ebpfCounterHandle,
) map[string]ebpfCounterHandle {
	networkInterfaceList := []string{}
	for iName, iDesc := range *networkInterfaceMap {
		networkInterfaceList = append(networkInterfaceList, iName)
		_, ok := dataHandles[iName]

		// create a new handle for interface if it is not in the map
		if !ok {
			ctx, cancel := context.WithCancel(context.Background())
			dataChan, err := self.ebpfApi.WatchInterface(
				ctx,
				iDesc.LinkInfo.Ifindex,
				250*time.Millisecond,
			)
			if err != nil {
				self.logger.Debug("unable to watch network interface", "name", iName, "index", iDesc.LinkInfo.Ifindex, "error", err)
				continue
			}
			self.logger.Debug("started watch network interface", "name", iName, "index", iDesc.LinkInfo.Ifindex, "error", err)
			dataHandles[iName] = ebpfCounterHandle{dataChan, ctx, cancel}
		}
	}

	// cancel and delete handles which are not found by the interface enumerator anymore
	handlesToDelete := []string{}
	for handleInterfaceName := range dataHandles {
		if !slices.Contains(networkInterfaceList, handleInterfaceName) {
			handlesToDelete = append(handlesToDelete, handleInterfaceName)
		}
	}
	for _, iName := range handlesToDelete {
		dataHandles[iName].cancel()
		delete(dataHandles, iName)
	}

	return dataHandles
}

func (self *networkMonitor) updateCollectedStats(
	networkInterfaceMap *map[InterfaceName]InterfaceDescription,
	ebpfDataHandles *map[string]ebpfCounterHandle,
	podList **v1.PodList,
	startBytes *map[InterfaceName][2]uint64,
) []PodNetworkStats {
	// requesting interface data
	// every handle has a poll rate so we wait for all of them to push once
	// the map gets all keys pre-allocated to prevent resizing while filling up the data from multiple go-routines in parallel
	lastInterfaceData := map[string]CountState{}
	for iName := range *ebpfDataHandles {
		lastInterfaceData[iName] = CountState{}
	}
	var wg sync.WaitGroup
	for iName, handle := range *ebpfDataHandles {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lastInterfaceData[iName] = <-handle.dataChan
		}()
	}
	wg.Wait()

	newCollectedStats := []PodNetworkStats{}
	for _, pod := range (*podList).Items {
		containerMap := self.buildContainerIdsMap(pod)
		for containerId, pod := range containerMap {
			for iName, iDesc := range *networkInterfaceMap {
				_, ok := iDesc.Containers[containerId]
				if !ok {
					continue
				}
				count, ok := lastInterfaceData[iName]
				if !ok {
					continue
				}
				interfaceStartBytes, ok := (*startBytes)[iName]
				if !ok {
					rx, err := self.loadUint64FromFile("/sys/class/net/" + iName + "/statistics/rx_bytes")
					if err != nil {
						self.logger.Debug("failed to read rx start bytes", "error", err)
					}
					tx, err := self.loadUint64FromFile("/sys/class/net/" + iName + "/statistics/tx_bytes")
					if err != nil {
						self.logger.Debug("failed to read tx start bytes", "error", err)
					}
					interfaceStartBytes = [2]uint64{rx, tx}
					(*startBytes)[iName] = interfaceStartBytes
				}
				stats := PodNetworkStats{}
				stats.Ip = pod.Status.PodIP
				stats.Pod = pod.GetName()
				stats.Interface = iName
				stats.Namespace = pod.GetNamespace()
				stats.PacketCount = count.PacketCount
				stats.TransferredBytes = count.Bytes
				stats.ReceivedStartBytes = interfaceStartBytes[0]
				stats.TransmitStartBytes = interfaceStartBytes[1]
				stats.StartTime = pod.Status.StartTime.Format(time.RFC3339)
				stats.CreatedAt = time.Now().Format(time.RFC3339)
				newCollectedStats = append(newCollectedStats, stats)
			}
		}
	}
	return newCollectedStats
}

func (self *networkMonitor) updatePodList(fieldSelector string) *v1.PodList {
	listOpts := metav1.ListOptions{FieldSelector: fieldSelector}
	newPodList, err := self.clientProvider.K8sClientSet().CoreV1().Pods("").List(context.TODO(), listOpts)
	if err != nil {
		self.logger.Error("failed to list pods", "listOptions", listOpts, "error", err)
		return &v1.PodList{}
	}
	return newPodList
}

// FOLLOWING CODE HAS BEEN COPIED FROM https://github.com/up9inc/mizu/tree/main Thanks for the great work @UP9 Inc
func (self *networkMonitor) buildContainerIdsMap(pod v1.Pod) map[string]v1.Pod {
	result := make(map[string]v1.Pod)
	for _, container := range pod.Status.ContainerStatuses {
		parsedUrl, err := url.Parse(container.ContainerID)
		if err != nil {
			self.logger.Warn("Expecting URL like container ID", "container", container.ContainerID)
			continue
		}

		result[parsedUrl.Host] = pod
	}

	return result
}

func (self *networkMonitor) loadUint64FromFile(filePath string) (uint64, error) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return uint64(0), err
	}

	var stringData = strings.TrimSuffix(string(fileContent), "\n")
	number, err := strconv.ParseUint(stringData, 10, 64)
	if err != nil {
		return uint64(0), err
	}

	return number, nil
}

type PodNetworkStats struct {
	Ip                 string `json:"ip"`
	Pod                string `json:"pod"`
	Interface          string `json:"interface"`
	Namespace          string `json:"namespace"`
	PacketCount        uint64 `json:"packetCount"`
	TransferredBytes   uint64 `json:"transferredBytes"`
	ReceivedStartBytes uint64 `json:"receivedStartBytes"` // auslesen aus /sys
	TransmitStartBytes uint64 `json:"transmitStartBytes"` // auslesen aus /sys
	StartTime          string `json:"startTime"`          // start time of the Interface/Pod
	CreatedAt          string `json:"createdAt"`          // when the entry was written into the storage <- timestamp of write to redis
}

func (self *PodNetworkStats) Sum(other *PodNetworkStats) {
	self.PacketCount += other.PacketCount
	self.TransmitStartBytes += other.TransmitStartBytes
	self.ReceivedStartBytes += other.ReceivedStartBytes
}

func (data *PodNetworkStats) SumOrReplace(dataToAdd *PodNetworkStats) {
	if dataToAdd.TransmitStartBytes > data.TransmitStartBytes || dataToAdd.ReceivedStartBytes > data.ReceivedStartBytes {
		// new startRX+startTX means an reset of the counters
		data.TransmitStartBytes = dataToAdd.TransmitStartBytes
		data.ReceivedStartBytes = dataToAdd.ReceivedStartBytes
		data.PacketCount = dataToAdd.PacketCount
	} else {
		// just sum the values if startRX+startTX is the same (it changes if the traffic collector restarts)
		data.PacketCount += dataToAdd.PacketCount
	}
}
