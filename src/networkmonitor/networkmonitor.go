package networkmonitor

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
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

		updateDataTicker := time.NewTicker(3 * time.Second)
		defer updateDevicesTicker.Stop()

		// holds the context of all network interfaces which are being watched
		// the list has to be updated regularly for:
		// - deleted interfaces where the handled is not valid anymore
		// - added interfaces where new handles have to be created
		ebpfDataHandles := map[int]ebpfCounterHandle{}
		defer func() {
			handles := []int{}
			for interfaceId := range ebpfDataHandles {
				handles = append(handles, interfaceId)
			}
			for _, interfaceId := range handles {
				ebpfDataHandles[interfaceId].cancel()
				delete(ebpfDataHandles, interfaceId)
			}
		}()

		podList := &v1.PodList{}
		startBytes := map[InterfaceId][2]uint64{}
		fieldSelector := "metadata.namespace!=kube-system"
		ownNodeName := self.config.Get("OWN_NODE_NAME")
		if ownNodeName != "" {
			fieldSelector = fmt.Sprintf("metadata.namespace!=kube-system,spec.nodeName=%s", ownNodeName)
		}

		// init
		rootNetworkInterfaces, err := self.cne.RequestInterfaceDescription(self.procFsMountPath)
		if err != nil {
			self.logger.Error("failed to request root network interfaces", "error", err)
		}
		networkInterfaceMap := self.cne.List(self.procFsMountPath)
		ebpfDataHandles = self.updateEbpfDataHandles(&rootNetworkInterfaces, ebpfDataHandles)
		podList = self.updatePodList(fieldSelector)
		collectedStats, err := self.updateCollectedStats(
			&rootNetworkInterfaces,
			&networkInterfaceMap,
			&ebpfDataHandles,
			&podList,
			&startBytes,
		)
		if err != nil {
			self.logger.Error("failed to update collect network interface stats", "error", err)
		}

		// loop
		for {
			select {
			case <-self.ctx.Done():
				return
			case <-updateDevicesTicker.C:
				rootNetworkInterfaces, err := self.cne.RequestInterfaceDescription(self.procFsMountPath)
				if err != nil {
					self.logger.Error("failed to request root network interfaces", "error", err)
				}
				networkInterfaceMap = self.cne.List(self.procFsMountPath)
				ebpfDataHandles = self.updateEbpfDataHandles(&rootNetworkInterfaces, ebpfDataHandles)
				podList = self.updatePodList(fieldSelector)
			case <-updateDataTicker.C:
				collectedStats, err = self.updateCollectedStats(
					&rootNetworkInterfaces,
					&networkInterfaceMap,
					&ebpfDataHandles,
					&podList,
					&startBytes,
				)
				if err != nil {
					self.logger.Error("failed to update collect network interface stats", "error", err)
				}
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
	rootNetworkInterfaces *[]IpLinkInfo,
	dataHandles map[int]ebpfCounterHandle,
) map[int]ebpfCounterHandle {
	rootNetworkInterfaceIds := []int{}
	for _, rootIPLinkInfo := range *rootNetworkInterfaces {
		rootInterfaceId := rootIPLinkInfo.Ifindex
		rootNetworkInterfaceIds = append(rootNetworkInterfaceIds, rootInterfaceId)
		_, ok := dataHandles[rootInterfaceId]

		// create a new handle for interface if it is not in the map
		if !ok {
			ctx, cancel := context.WithCancel(context.Background())
			dataChan, err := self.ebpfApi.WatchInterface(
				ctx,
				rootIPLinkInfo.Ifindex,
				250*time.Millisecond,
			)
			if err != nil {
				self.logger.Warn("unable to watch network interface", "id", rootInterfaceId, "linkIndex", rootIPLinkInfo.LinkIndex, "error", err)
				continue
			}
			self.logger.Debug("started watch network interface", "id", rootInterfaceId, "linkIndex", rootIPLinkInfo.LinkIndex, "ifName", rootIPLinkInfo.Ifname, "error", err)
			dataHandles[rootInterfaceId] = ebpfCounterHandle{dataChan, ctx, cancel}
		}
	}

	// cancel and delete handles which are not found by the interface enumerator anymore
	handlesToDelete := []int{}
	for handleInterfaceId := range dataHandles {
		if !slices.Contains(rootNetworkInterfaceIds, handleInterfaceId) {
			handlesToDelete = append(handlesToDelete, handleInterfaceId)
		}
	}
	for _, interfaceId := range handlesToDelete {
		dataHandles[interfaceId].cancel()
		delete(dataHandles, interfaceId)
	}

	return dataHandles
}

func (self *networkMonitor) updateCollectedStats(
	rootNetworkInterfaces *[]IpLinkInfo,
	networkInterfaceMap *map[ContainerId]InterfaceDescription,
	ebpfDataHandles *map[int]ebpfCounterHandle,
	podList **v1.PodList,
	startBytes *map[InterfaceId][2]uint64,
) ([]PodNetworkStats, error) {
	// requesting interface data
	// every handle has a poll rate so we wait for all of them to push once
	// the map gets all keys pre-allocated to prevent resizing while filling up the data from multiple go-routines in parallel
	lastInterfaceData := map[int]CountState{}
	lastInterfaceDataMutex := sync.Mutex{}
	var wg sync.WaitGroup
	for interfaceId, handle := range *ebpfDataHandles {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data := <-handle.dataChan
			lastInterfaceDataMutex.Lock()
			lastInterfaceData[interfaceId] = data
			lastInterfaceDataMutex.Unlock()
		}()
	}
	wg.Wait()

	newCollectedStats := []PodNetworkStats{}
	for _, pod := range (*podList).Items {
		containerMap := self.buildContainerIdsMap(pod)
		for podContainerId, pod := range containerMap {
			for containerId, iDesc := range *networkInterfaceMap {
				if containerId != podContainerId {
					continue
				}
				for _, virtualInterface := range iDesc.LinkInfo {
					if !virtualInterface.IsUp() {
						continue
					}
					if virtualInterface.IsLoopback() {
						continue
					}
					var rootInterface *IpLinkInfo = nil
					if virtualInterface.LinkIndex == 0 {
						// the virtual interface is also the root interface as there is no parent
						rootInterface = &virtualInterface
					} else {
						// has parent interface
						parentInterfaceId := virtualInterface.LinkIndex
						assert.Assert(parentInterfaceId != 0, "since this is always a virtualized id there should always be a parent", virtualInterface)
						for _, rootInterfaceInfo := range *rootNetworkInterfaces {
							if parentInterfaceId == rootInterfaceInfo.Ifindex {
								rootInterface = &rootInterfaceInfo
								break
							}
						}
					}
					assert.Assert(rootInterface != nil, "the root index has to be resolved succesfully")

					count, ok := lastInterfaceData[rootInterface.Ifindex]
					if !ok {
						self.logger.Warn("failed to read interface data for interface id", "rootInterface", rootInterface, "virtualInterface", virtualInterface)
						continue
					}

					interfaceStartBytes, ok := (*startBytes)[rootInterface.Ifindex]
					if !ok {
						rx, err := self.loadUint64FromFile("/sys/class/net/" + rootInterface.Ifname + "/statistics/rx_bytes")
						if err != nil {
							self.logger.Debug("failed to read rx start bytes", "error", err)
						}
						tx, err := self.loadUint64FromFile("/sys/class/net/" + rootInterface.Ifname + "/statistics/tx_bytes")
						if err != nil {
							self.logger.Debug("failed to read tx start bytes", "error", err)
						}
						interfaceStartBytes = [2]uint64{rx, tx}
						(*startBytes)[rootInterface.Ifindex] = interfaceStartBytes
					}
					stats := PodNetworkStats{}
					stats.Ip = pod.Status.PodIP
					stats.Pod = pod.GetName()
					stats.Interface = rootInterface.Ifname
					stats.VirtualInterface = virtualInterface.Ifname
					stats.InterfaceId = rootInterface.Ifindex
					stats.Namespace = pod.GetNamespace()
					stats.ReceivedPackets = count.IngressPackets
					stats.ReceivedBytes = count.IngressBytes
					stats.ReceivedStartBytes = interfaceStartBytes[0]
					stats.TransmitPackets = count.EgressPackets
					stats.TransmitBytes = count.EgressBytes
					stats.TransmitStartBytes = interfaceStartBytes[1]
					stats.StartTime = pod.Status.StartTime.Format(time.RFC3339)
					stats.CreatedAt = time.Now().Format(time.RFC3339)
					newCollectedStats = append(newCollectedStats, stats)
				}

			}
		}
	}

	podNames := []string{}
	for _, stat := range newCollectedStats {
		if !slices.Contains(podNames, stat.Pod) {
			podNames = append(podNames, stat.Pod)
		}
	}
	slices.Sort(podNames)

	interfaceIds := []int{}
	for _, info := range *rootNetworkInterfaces {
		if !slices.Contains(interfaceIds, info.Ifindex) {
			interfaceIds = append(interfaceIds, info.Ifindex)
		}
	}
	slices.Sort(interfaceIds)

	sortedCollectedStats := []PodNetworkStats{}
	for _, podName := range podNames {
		for _, interfaceId := range interfaceIds {
			for _, stats := range newCollectedStats {
				if stats.Pod == podName && stats.InterfaceId == interfaceId {
					sortedCollectedStats = append(sortedCollectedStats, stats)
				}
			}
		}
	}
	assert.Assert(len(sortedCollectedStats) == len(newCollectedStats), "this mapping should preserve all elements")

	return sortedCollectedStats, nil
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
	VirtualInterface   string `json:"virtualInterface"`
	InterfaceId        int    `json:"interfaceId"`
	Namespace          string `json:"namespace"`
	ReceivedPackets    uint64 `json:"receivedPackets"`
	ReceivedBytes      uint64 `json:"receivedBytes"`
	ReceivedStartBytes uint64 `json:"receivedStartBytes"` // auslesen aus /sys
	TransmitPackets    uint64 `json:"transmitPackets"`
	TransmitBytes      uint64 `json:"transmitBytes"`
	TransmitStartBytes uint64 `json:"transmitStartBytes"` // auslesen aus /sys
	StartTime          string `json:"startTime"`          // start time of the Interface/Pod
	CreatedAt          string `json:"createdAt"`          // when the entry was written into the storage <- timestamp of write to redis
}

func (self *PodNetworkStats) Sum(other *PodNetworkStats) {
	self.ReceivedPackets += other.ReceivedPackets
	self.ReceivedBytes += other.ReceivedBytes
	self.ReceivedStartBytes += other.ReceivedStartBytes
	self.TransmitPackets += other.TransmitPackets
	self.TransmitBytes += other.TransmitBytes
	self.TransmitStartBytes += other.TransmitStartBytes
}

func (self *PodNetworkStats) SumOrReplace(other *PodNetworkStats) {
	if other.TransmitStartBytes > self.TransmitStartBytes || other.ReceivedStartBytes > self.ReceivedStartBytes {
		// new startRX+startTX means an reset of the counters
		self.TransmitStartBytes = other.TransmitStartBytes
		self.ReceivedStartBytes = other.ReceivedStartBytes
		self.ReceivedPackets = other.ReceivedPackets
		self.ReceivedBytes = other.ReceivedBytes
		self.ReceivedStartBytes = other.ReceivedStartBytes
		self.TransmitPackets = other.TransmitPackets
		self.TransmitBytes = other.TransmitBytes
		self.TransmitStartBytes = other.TransmitStartBytes
	} else {
		// just sum the values if startRX+startTX is the same (it changes if the traffic collector restarts)
		self.ReceivedPackets += other.ReceivedPackets
		self.ReceivedBytes += other.ReceivedBytes
		self.TransmitPackets += other.TransmitPackets
		self.TransmitBytes += other.TransmitBytes
	}
}
