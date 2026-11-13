package networkmonitor

import (
	"fmt"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/containerenumerator"
	"os"
	"slices"
	"strconv"
	"sync/atomic"
	"time"
)

type networkStatsReader struct {
	logger   *slog.Logger
	procPath string

	metricsTx chan struct{}
	metricsRx chan map[containerenumerator.PodInfoIdentifier]ContainerInfo

	registerTx chan networkStatsReaderDeviceBaseInfo
	registerRx chan error

	removeTx chan containerenumerator.PodInfoIdentifier
	removeRx chan error

	running atomic.Bool

	cne containerenumerator.ContainerEnumerator
}

type networkStatsReaderDeviceBaseInfo struct {
	PodInfo     containerenumerator.PodInfo
	ContainerId ContainerId
	Pid         ProcessId
	Pids        []ProcessId
	StartInfos  []KernelNetworkInterfaceInfo
}

func NewNetworkStatsReader(logger *slog.Logger, cne containerenumerator.ContainerEnumerator, procPath string) SnoopyManager {
	self := &networkStatsReader{}

	self.logger = logger
	self.procPath = procPath
	self.running = atomic.Bool{}

	self.metricsTx = make(chan struct{})
	self.metricsRx = make(chan map[containerenumerator.PodInfoIdentifier]ContainerInfo)
	self.registerTx = make(chan networkStatsReaderDeviceBaseInfo)
	self.registerRx = make(chan error)
	self.removeTx = make(chan containerenumerator.PodInfoIdentifier)
	self.removeRx = make(chan error)

	self.cne = cne

	return self
}

// shim to be comptabile with Snoopy
func (self *networkStatsReader) Run() {
	self.startWorker()
}

func (self *networkStatsReader) startWorker() {
	wasRunning := self.running.Swap(true)
	if wasRunning {
		return
	}

	go func() {
		baseDeviceInfos := []networkStatsReaderDeviceBaseInfo{}
		metrics := map[containerenumerator.PodInfoIdentifier]ContainerInfo{}
		containers := self.cne.GetProcessesWithContainerIds()

		networkEnumeratorTicker := time.NewTicker(5 * time.Second)
		defer networkEnumeratorTicker.Stop()
		updateTicker := time.NewTicker(1 * time.Second)
		defer updateTicker.Stop()

		for {
			select {
			case <-self.metricsTx:
				self.metricsRx <- metrics
			case newDeviceInfo := <-self.registerTx:
				newPodId := newDeviceInfo.PodInfo.NamespaceAndName()
				alreadyRegistered := false
				for _, info := range baseDeviceInfos {
					if info.PodInfo.NamespaceAndName() == newPodId {
						alreadyRegistered = true
						break
					}
				}
				if alreadyRegistered {
					self.registerRx <- fmt.Errorf("pod network is already registered: %s", newPodId)
					continue
				}
				baseDeviceInfos = append(baseDeviceInfos, newDeviceInfo)
				self.registerRx <- nil
			case podId := <-self.removeTx:
				removeIdx := -1
				for idx, info := range baseDeviceInfos {
					if info.PodInfo.NamespaceAndName() == podId {
						removeIdx = idx
						break
					}
				}
				if removeIdx == -1 {
					self.removeRx <- fmt.Errorf("failed to find pod in tracked pod list: %s", podId)
					continue
				}
				baseDeviceInfos = slices.Delete(baseDeviceInfos, removeIdx, removeIdx+1)
				self.removeRx <- nil
			case <-networkEnumeratorTicker.C:
				containers = self.cne.GetProcessesWithContainerIds()
			case <-updateTicker.C:
				newMetrics := map[containerenumerator.PodInfoIdentifier]ContainerInfo{}

				baseDeviceIdsWithBrokenPid := []int{}
				for idx, baseInfo := range baseDeviceInfos {
					_, err := os.Stat(self.procPath + "/" + strconv.FormatUint(baseInfo.Pid, 10))
					if err == nil {
						// process still exists, no update necessary
						continue
					}

					self.logger.Debug("triggering update of pid", "baseInfo", baseInfo)
					baseDeviceIdsWithBrokenPid = append(baseDeviceIdsWithBrokenPid, idx)
				}

				for _, idx := range baseDeviceIdsWithBrokenPid {
					self.logger.Debug("updating pid", "baseInfo", baseDeviceInfos[idx], "containers", containers)
					pids, ok := containers[baseDeviceInfos[idx].ContainerId]
					if !ok {
						// not yet recoverable but thats ok
						continue
					}

					assert.Assert(len(pids) > 0, "every running container should have at least 1 active pid")
					baseDeviceInfos[idx].Pid = pids[0]
					baseDeviceInfos[idx].Pids = pids
					self.logger.Debug("updated pid", "baseInfo", baseDeviceInfos[idx])
				}

				// Iterate by index: the loop body appends late-discovered
				// interfaces to baseInfo.StartInfos, and on a value copy that
				// append was lost every tick — re-baselining those interfaces
				// to "now" so they never reported any traffic.
				for baseIdx := range baseDeviceInfos {
					baseInfo := &baseDeviceInfos[baseIdx]
					containerInfo := ContainerInfo{}
					containerInfo.PodInfo = baseInfo.PodInfo
					containerInfo.Metrics = map[InterfaceName]MetricSnapshot{}

					networkInterfaceInfos, err := getNetworkInterfaceInfo(self.procPath, strconv.FormatUint(baseInfo.Pid, 10))
					if err != nil {
						self.logger.Error("failed to read network interface metrics", "procPath", self.procPath, "baseDevice", baseInfo, "error", err)
						for _, baseStartInfo := range baseInfo.StartInfos {
							metricSnapshot := MetricSnapshot{}
							metricSnapshot.Ingress.StartBytes = baseStartInfo.ReceiveBytes
							metricSnapshot.Ingress.Bytes = 0
							metricSnapshot.Ingress.Packets = 0
							metricSnapshot.Egress.StartBytes = baseStartInfo.TransmitBytes
							metricSnapshot.Egress.Bytes = 0
							metricSnapshot.Egress.Packets = 0
							containerInfo.Metrics[baseStartInfo.Interface] = metricSnapshot
						}
						newMetrics[baseInfo.PodInfo.NamespaceAndName()] = containerInfo
						continue
					}

					for _, currentInfo := range networkInterfaceInfos {
						var baseStartInfo *KernelNetworkInterfaceInfo
						for _, info := range baseInfo.StartInfos {
							if info.Interface == currentInfo.Interface {
								baseStartInfo = &info
								break
							}
						}
						if baseStartInfo == nil {
							// this device was added after initially registering the container
							// the current metrics are appended as start info
							baseInfo.StartInfos = append(baseInfo.StartInfos, currentInfo)
							baseStartInfo = &currentInfo
						}
						metricSnapshot := MetricSnapshot{}
						metricSnapshot.Ingress.StartBytes = baseStartInfo.ReceiveBytes
						metricSnapshot.Ingress.Bytes = currentInfo.ReceiveBytes - baseStartInfo.ReceiveBytes
						metricSnapshot.Ingress.Packets = currentInfo.ReceivePackets - baseStartInfo.ReceivePackets
						metricSnapshot.Egress.StartBytes = baseStartInfo.TransmitBytes
						metricSnapshot.Egress.Bytes = currentInfo.TransmitBytes - baseStartInfo.TransmitBytes
						metricSnapshot.Egress.Packets = currentInfo.TransmitPackets - baseStartInfo.TransmitPackets
						containerInfo.Metrics[baseStartInfo.Interface] = metricSnapshot
					}

					newMetrics[baseInfo.PodInfo.NamespaceAndName()] = containerInfo
				}
				metrics = newMetrics
			}
		}
	}()
}

func (self *networkStatsReader) Metrics() map[containerenumerator.PodInfoIdentifier]ContainerInfo {
	self.Run()

	self.metricsTx <- struct{}{}
	metrics := <-self.metricsRx

	return metrics
}

// Register tracks the pod's network namespace once: all containers of a pod
// share it, so reading /proc/$pid/net/dev per container counted the same
// interfaces several times and reported the pod once per container.
func (self *networkStatsReader) Register(podInfo containerenumerator.PodInfo) []error {
	errors := []error{}
	for _, container := range podInfo.SortedContainersWithFirstPid() {
		kernelNetworkInterfaceInfos, err := getNetworkInterfaceInfo(self.procPath, strconv.FormatUint(container.Pid, 10))
		if err != nil {
			// the first process may already be gone; try the next container
			errors = append(errors, err)
			continue
		}

		self.registerTx <- networkStatsReaderDeviceBaseInfo{
			PodInfo:     podInfo,
			ContainerId: container.ContainerId,
			Pid:         container.Pid,
			StartInfos:  kernelNetworkInterfaceInfos,
		}
		if err := <-self.registerRx; err != nil {
			return []error{err}
		}
		return nil
	}

	return errors
}

func (self *networkStatsReader) Remove(podInfo containerenumerator.PodInfo) []error {
	self.removeTx <- podInfo.NamespaceAndName()
	if err := <-self.removeRx; err != nil {
		return []error{err}
	}
	return nil
}

func (self *networkStatsReader) SetArgs(args SnoopyArgs) {}

func (self *networkStatsReader) Status() SnoopyStatus {
	return SnoopyStatus{}
}
