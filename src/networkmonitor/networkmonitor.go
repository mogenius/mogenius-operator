package networkmonitor

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/k8sclient"
	"net/url"
	"slices"
	"sync/atomic"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NetworkMonitor interface {
	Run()
	GetPodNetworkUsage() []PodNetworkStats
	SetSnoopyArgs(args SnoopyArgs)
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
	snoopy           SnoopyManager

	networkUsageTx chan struct{}
	networkUsageRx chan []PodNetworkStats
}

type ContainerId = string
type PodName = string
type ProcessId = uint64
type InterfaceName = string

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
	self.snoopy = NewSnoopyManager(self.logger.With("scope", "snoopy-manager"))
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
		fieldSelector := "metadata.namespace!=kube-system"
		ownNodeName := self.config.Get("OWN_NODE_NAME")
		if ownNodeName != "" {
			fieldSelector = fmt.Sprintf("metadata.namespace!=kube-system,spec.nodeName=%s", ownNodeName)
		}

		// first load of all pods on the current node
		podList := self.updatePodList(fieldSelector)

		// first load of all containers running on the current node
		nodeContainersWithProcesses := self.cne.FindProcessesWithContainerIds(self.procFsMountPath)

		// register all initially found containers which are also pods by indexing
		// the nodeContainersWithProcesses map with all podContainerIds extracted from podList
		for _, pod := range podList.Items {
			podContainerIds := self.readContainerIds(pod)
			for _, containerId := range podContainerIds {
				pids, ok := nodeContainersWithProcesses[containerId]
				if !ok {
					continue
				}
				assert.Assert(len(pids) > 0, "every container is expected to have at least 1 active pid")
				pid := pids[0]
				err := self.snoopy.Register(pod.Namespace, pod.Name, containerId, pid)
				if err != nil {
					self.logger.Error("failed to register snoopy", "containerId", containerId, "pid", pid, "error", err)
					continue
				}
			}
		}

		// get initial collected stats
		metrics := self.snoopy.Metrics()
		collectedStats := self.metricsToPodstats(metrics, podList)

		// timers
		updatePodAndContainersTicker := time.NewTicker(5 * time.Second)
		defer updatePodAndContainersTicker.Stop()
		updateCollectedStatsTicker := time.NewTicker(1 * time.Second)
		defer updateCollectedStatsTicker.Stop()

		// enter update loop
		for {
			select {
			case <-self.ctx.Done():
				break
			case <-updatePodAndContainersTicker.C:
				// get a new list of all pods and containers on the current node
				newPodList := self.updatePodList(fieldSelector)
				nodeContainersWithProcesses = self.cne.FindProcessesWithContainerIds(self.procFsMountPath)

				// check for created and removed pods
				oldPodContainerIds := []string{}
				for _, pod := range podList.Items {
					containerIds := self.readContainerIds(pod)
					oldPodContainerIds = append(oldPodContainerIds, containerIds...)
				}
				newPodContainerIds := []string{}
				for _, pod := range newPodList.Items {
					containerIds := self.readContainerIds(pod)
					newPodContainerIds = append(newPodContainerIds, containerIds...)
				}
				deletedPodContainerIds := []string{}
				for _, uid := range oldPodContainerIds {
					if !slices.Contains(newPodContainerIds, uid) {
						deletedPodContainerIds = append(deletedPodContainerIds, uid)
					}
				}
				createdPodContainerIds := []string{}
				for _, uid := range newPodContainerIds {
					if !slices.Contains(oldPodContainerIds, uid) {
						createdPodContainerIds = append(createdPodContainerIds, uid)
					}
				}

				// register new containers
				for _, containerId := range createdPodContainerIds {
					for _, pod := range newPodList.Items {
						containerIds := self.readContainerIds(pod)
						if !slices.Contains(containerIds, containerId) {
							// this pod does not have the container id we are looking for
							continue
						}
						pids, ok := nodeContainersWithProcesses[containerId]
						if !ok {
							continue
						}
						assert.Assert(len(pids) > 0, "every container is expected to have at least 1 active pid")
						pid := pids[0]
						err := self.snoopy.Register(pod.Namespace, pod.Name, containerId, pid)
						if err != nil {
							self.logger.Error("failed to register snoopy", "containerId", containerId, "pid", pid, "error", err)
							continue
						}
					}
				}

				// unregister old containers
				for _, containerId := range deletedPodContainerIds {
					for _, pod := range podList.Items {
						containerIds := self.readContainerIds(pod)
						if !slices.Contains(containerIds, containerId) {
							// this pod does not have the container id we are looking for
							continue
						}
						err := self.snoopy.Remove(containerId)
						if err != nil {
							self.logger.Error("failed to remove snoopy", "containerId", containerId, "error", err)
							continue
						}
					}
				}

				// set the new podList as active podList
				podList = newPodList

			case <-updateCollectedStatsTicker.C:
				metrics = self.snoopy.Metrics()
				collectedStats = self.metricsToPodstats(metrics, podList)
			case <-self.networkUsageTx:
				self.networkUsageRx <- collectedStats
			}
		}
	}()
}

func (self *networkMonitor) SetSnoopyArgs(args SnoopyArgs) {
	self.snoopy.SetArgs(args)
}

func (self *networkMonitor) metricsToPodstats(
	metrics map[ContainerId]ContainerInfo,
	podList *v1.PodList,
) []PodNetworkStats {
	data := []PodNetworkStats{}

	containerIds := []ContainerId{}
	for containerId := range metrics {
		containerIds = append(containerIds, containerId)
	}
	slices.Sort(containerIds)

	for _, containerId := range containerIds {
		containerInfo := metrics[containerId]
		var pod *v1.Pod
		for _, podListItem := range podList.Items {
			cids := self.readContainerIds(podListItem)
			if slices.Contains(cids, containerId) {
				pod = &podListItem
				break
			}
		}
		assert.Assert(pod != nil, "pod has to exist in podList")

		interfaceNames := []InterfaceName{}
		for interfaceName := range containerInfo.Metrics {
			interfaceNames = append(interfaceNames, interfaceName)
		}
		slices.Sort(interfaceNames)

		for _, interfaceName := range interfaceNames {
			metrics := containerInfo.Metrics[interfaceName]
			// filter empty metrics (if all observed values are 0)
			if metrics.Ingress.Packets == 0 && metrics.Ingress.Bytes == 0 && metrics.Egress.Packets == 0 && metrics.Egress.Bytes == 0 {
				continue
			}
			// filter blacklisted interface names e.g. loopback
			if slices.Contains([]string{"lo"}, interfaceName) {
				continue
			}
			podNetworkStat := PodNetworkStats{}
			podNetworkStat.Ip = pod.Status.PodIP
			podNetworkStat.Pod = containerInfo.PodName
			podNetworkStat.Namespace = containerInfo.PodNamespace
			podNetworkStat.Interface = interfaceName
			podNetworkStat.ReceivedPackets = metrics.Ingress.Packets
			podNetworkStat.ReceivedBytes = metrics.Ingress.Bytes
			podNetworkStat.ReceivedStartBytes = metrics.Ingress.StartBytes
			podNetworkStat.TransmitPackets = metrics.Egress.Packets
			podNetworkStat.TransmitBytes = metrics.Egress.Bytes
			podNetworkStat.TransmitStartBytes = metrics.Egress.StartBytes
			podNetworkStat.StartTime = pod.Status.StartTime.Format(time.RFC3339)
			podNetworkStat.CreatedAt = time.Now().Format(time.RFC3339)
			data = append(data, podNetworkStat)
		}
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

func (self *networkMonitor) updatePodList(fieldSelector string) *v1.PodList {
	listOpts := metav1.ListOptions{FieldSelector: fieldSelector}
	newPodList, err := self.clientProvider.K8sClientSet().CoreV1().Pods("").List(context.TODO(), listOpts)
	if err != nil {
		self.logger.Error("failed to list pods", "listOptions", listOpts, "error", err)
		return &v1.PodList{}
	}

	// important step: Remove all pods with HostNetwork=true
	filteredItems := []v1.Pod{}
	for idx := 0; idx < len(newPodList.Items); idx++ {
		pod := newPodList.Items[idx]
		if pod.Spec.HostNetwork == false {
			filteredItems = append(filteredItems, pod)
		}
	}
	newPodList.Items = filteredItems

	return newPodList
}

func (self *networkMonitor) readContainerIds(pod v1.Pod) []ContainerId {
	result := []ContainerId{}
	for _, container := range pod.Status.ContainerStatuses {
		parsedUrl, err := url.Parse(container.ContainerID)
		if err != nil {
			self.logger.Warn("Expecting URL like container ID", "container", container.ContainerID)
			continue
		}

		result = append(result, parsedUrl.Host)
	}
	slices.Sort(result)

	return result
}

type PodNetworkStats struct {
	Ip                 string `json:"ip"`
	Pod                string `json:"pod"`
	Namespace          string `json:"namespace"`
	Interface          string `json:"interface"`
	ReceivedPackets    uint64 `json:"receivedPackets"`
	ReceivedBytes      uint64 `json:"receivedBytes"`
	ReceivedStartBytes uint64 `json:"receivedStartBytes"`
	TransmitPackets    uint64 `json:"transmitPackets"`
	TransmitBytes      uint64 `json:"transmitBytes"`
	TransmitStartBytes uint64 `json:"transmitStartBytes"`
	StartTime          string `json:"startTime"` // start time of the Interface/Pod
	CreatedAt          string `json:"createdAt"` // when the entry was written into the storage <- timestamp of write to redis
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
