package networkmonitor

import (
	"context"
	"fmt"
	"log/slog"
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

const (
	defaultSnapLen                     = 65536   // Max Size of TCP Packets (65536 - Wireshark uses 262144)
	BYTES_CHANGE_SEND_TRESHHOLD uint64 = 1048576 // wait until X bytes are gathered until we send an update to the API server
)

type NetworkMonitor interface {
	Run()
	NetworkUsage() []InterfaceStats
}

type networkMonitor struct {
	logger         *slog.Logger
	clientProvider k8sclient.K8sClientProvider
	ctx            context.Context
	cancel         context.CancelFunc

	collectorStarted atomic.Bool
	procFsMountPath  string
	cne              ContainerNetworkEnumerator
	ebpfApi          EbpfApi

	networkUsageTx chan struct{}
	networkUsageRx chan []InterfaceStats
}

func NewNetworkMonitor(logger *slog.Logger, clientProvider k8sclient.K8sClientProvider, procFsMountPath string) NetworkMonitor {
	self := &networkMonitor{}

	self.logger = logger
	self.clientProvider = clientProvider
	self.collectorStarted = atomic.Bool{}
	ctx, cancel := context.WithCancel(context.Background())
	self.ctx = ctx
	self.cancel = cancel
	self.cne = NewContainerNetworkEnumerator()
	self.ebpfApi = NewEbpfApi(self.logger.With("scope", "ebpf"))
	self.procFsMountPath = procFsMountPath
	self.networkUsageTx = make(chan struct{})
	self.networkUsageRx = make(chan []InterfaceStats)

	return self
}

func (self *networkMonitor) Run() {
	wasStarted := self.collectorStarted.Swap(true)
	if wasStarted {
		return
	}

	type ebpfCounterHandle struct {
		dataChan chan CountState
		ctx      context.Context
		cancel   context.CancelFunc
	}

	go func() {
		defer self.cancel()

		scanTimeout := 0 * time.Second // first run is made instantly

		// holds the context of all network interfaces which are being watched
		// the list has to be updated regularly for:
		// - deleted interfaces where the handled is not valid anymore
		// - added interfaces where new handles have to be created
		ebpfDataHandles := map[string]ebpfCounterHandle{}

		ownNodeName := os.Getenv("OWN_NODE_NAME")
		fieldSelector := "metadata.namespace!=kube-system"
		if ownNodeName != "" {
			fieldSelector = fmt.Sprintf("metadata.namespace!=kube-system,spec.nodeName=%s", ownNodeName)
		}

		collectedStats := []InterfaceStats{}
		startBytes := map[InterfaceName][2]uint64{}

		for {
			select {
			case <-self.ctx.Done():
				for _, handle := range ebpfDataHandles {
					handle.cancel()
				}
				return
			case <-time.After(scanTimeout):
				scanTimeout = 30 * time.Second
				networkInterfaceMap := self.cne.List(self.procFsMountPath)
				networkInterfaceList := []string{}
				for iName, iDesc := range networkInterfaceMap {
					networkInterfaceList = append(networkInterfaceList, iName)
					_, ok := ebpfDataHandles[iName]

					// create a new handle for interface if it is not in the map
					if !ok {
						ebpfCtx, ebpfCancel := context.WithCancel(context.Background())
						dataChan, err := self.ebpfApi.WatchInterface(
							ebpfCtx,
							iDesc.LinkInfo.Ifindex,
							1*time.Second,
						)
						if err != nil {
							self.logger.Warn("unable to watch network interface", "name", iName, "index", iDesc.LinkInfo.Ifindex, "error", err)
							continue
						}
						self.logger.Info("started watch network interface", "name", iName, "index", iDesc.LinkInfo.Ifindex, "error", err)
						ebpfDataHandles[iName] = ebpfCounterHandle{dataChan, ebpfCtx, ebpfCancel}
					}
				}

				// cancel and delete handles which are not found by the interface enumerator anymore
				handlesToDelete := []string{}
				for handleInterfaceName, handle := range ebpfDataHandles {
					if !slices.Contains(networkInterfaceList, handleInterfaceName) {
						handlesToDelete = append(handlesToDelete, handleInterfaceName)
						handle.cancel()
					}
				}
				for _, iName := range handlesToDelete {
					delete(ebpfDataHandles, iName)
				}

				// store data
				for iName, handle := range ebpfDataHandles {
					select {
					case <-handle.ctx.Done():
						delete(ebpfDataHandles, iName)
						continue
					case data := <-handle.dataChan:
						_ = data // this has to be put in redis with stuff
					default:
					}
				}

				// requesting interface data
				// every handle has a poll rate so we wait for all of them to push once
				// the map gets all keys pre-allocated to prevent resizing while filling up the data from multiple go-routines in parallel
				lastInterfaceData := map[string]CountState{}
				for iName := range ebpfDataHandles {
					lastInterfaceData[iName] = CountState{}
				}
				var wg sync.WaitGroup
				for iName, handle := range ebpfDataHandles {
					wg.Add(1)
					go func() {
						defer wg.Done()
						lastInterfaceData[iName] = <-handle.dataChan
					}()
				}
				wg.Wait()

				listOpts := metav1.ListOptions{FieldSelector: fieldSelector}
				podList, err := self.clientProvider.K8sClientSet().CoreV1().Pods("").List(context.TODO(), listOpts)
				if err != nil {
					self.logger.Error("failed to list pods", "node", ownNodeName, "listOptions", listOpts, "error", err)
					continue
				}
				newCollectedStats := []InterfaceStats{}
				for _, pod := range podList.Items {
					containerMap := self.buildContainerIdsMap(pod)
					for containerId, pod := range containerMap {
						for iName, iDesc := range networkInterfaceMap {
							_, ok := iDesc.Containers[containerId]
							if !ok {
								continue
							}
							count, ok := lastInterfaceData[iName]
							if !ok {
								continue
							}
							interfaceStartBytes, ok := startBytes[iName]
							if !ok {
								rx, err := self.loadUint64FromFile("/sys/class/net/" + iName + "/statistics/rx_bytes")
								if err != nil {
									self.logger.Warn("failed to read rx start bytes", "error", err)
								}
								tx, err := self.loadUint64FromFile("/sys/class/net/" + iName + "/statistics/tx_bytes")
								if err != nil {
									self.logger.Warn("failed to read tx start bytes", "error", err)
								}
								interfaceStartBytes = [2]uint64{rx, tx}
								startBytes[iName] = interfaceStartBytes
							}
							stats := InterfaceStats{}
							stats.Ip = pod.Status.PodIP
							stats.Pod = pod.GetName()
							stats.Interface = iName
							stats.Namespace = pod.GetNamespace()
							stats.PacketCount = count.PacketCount
							stats.TransferredByteCount = count.Bytes
							stats.ReceivedStartBytes = interfaceStartBytes[0]
							stats.TransmitStartBytes = interfaceStartBytes[1]
							stats.StartTime = pod.Status.StartTime.Format(time.RFC3339)
							stats.CreatedAt = time.Now().Format(time.RFC3339)
							newCollectedStats = append(newCollectedStats, stats)
						}
					}
				}
				collectedStats = newCollectedStats
			case <-self.networkUsageTx:
				self.networkUsageRx <- collectedStats
			}
		}
	}()
}

func (self *networkMonitor) NetworkUsage() []InterfaceStats {
	select {
	case <-self.ctx.Done():
		self.logger.Warn("requested metrics from network monitor after it was closed")
		return []InterfaceStats{}
	case self.networkUsageTx <- struct{}{}:
		select {
		case <-self.ctx.Done():
			self.logger.Warn("requested metrics from network monitor after it was closed")
			return []InterfaceStats{}
		case result := <-self.networkUsageRx:
			return result
		}
	}
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

type InterfaceStats struct {
	Ip                   string `json:"ip"`
	Pod                  string `json:"pod"`
	Interface            string `json:"interface"`
	Namespace            string `json:"namespace"`
	PacketCount          uint64 `json:"packetCount"`
	TransferredByteCount uint64 `json:"transferredByteCount"`
	ReceivedStartBytes   uint64 `json:"receivedStartBytes"` // auslesen aus /sys
	TransmitStartBytes   uint64 `json:"transmitStartBytes"` // auslesen aus /sys
	StartTime            string `json:"startTime"`          // start time of the Interface/Pod
	CreatedAt            string `json:"createdAt"`          // when the entry was written into the storage <- timestamp of write to redis
}

func (self *InterfaceStats) Sum(other *InterfaceStats) {
	self.PacketCount += other.PacketCount
	self.TransmitStartBytes += other.TransmitStartBytes
	self.ReceivedStartBytes += other.ReceivedStartBytes
}

func (data *InterfaceStats) SumOrReplace(dataToAdd *InterfaceStats) {
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
