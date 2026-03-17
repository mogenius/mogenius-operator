package core

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/podstatscollector"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"strconv"
	"time"

	json "github.com/goccy/go-json"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

type PodStatsCollector interface {
	Run()
	Link(statsDb ValkeyStatsDb)
}

type podStatsCollector struct {
	logger         *slog.Logger
	clientProvider k8sclient.K8sClientProvider
	statsDb        ValkeyStatsDb
	config         config.ConfigModule

	startTime      time.Time
	updateInterval uint64
}

func NewPodStatsCollector(
	logger *slog.Logger,
	configModule config.ConfigModule,
	clientProviderModule k8sclient.K8sClientProvider,
) PodStatsCollector {
	self := &podStatsCollector{}

	self.logger = logger
	self.config = configModule
	self.clientProvider = clientProviderModule
	self.startTime = time.Now()
	self.updateInterval = 60

	return self
}

func (self *podStatsCollector) Link(statsDb ValkeyStatsDb) {
	assert.Assert(statsDb != nil)

	self.statsDb = statsDb
}

func (self *podStatsCollector) Run() {
	assert.Assert(self.logger != nil)
	assert.Assert(self.config != nil)
	assert.Assert(self.clientProvider != nil)
	assert.Assert(self.statsDb != nil)

	enabled, err := strconv.ParseBool(self.config.Get("MO_ENABLE_POD_STATS_COLLECTOR"))
	assert.Assert(err == nil, err)
	if enabled {
		go func() {
			for {
				nodemetrics := self.getRealNodeMetrics()

				currentPods := map[string]v1.Pod{}
				pods := store.GetPods("*")
				for _, pod := range pods {
					currentPods[pod.Name] = pod
				}

				podsResult, err := self.podStats(nodemetrics, currentPods)
				if err != nil {
					time.Sleep(time.Duration(self.updateInterval) * time.Second)
					continue
				}
				err = self.statsDb.AddPodStatsToDb(podsResult)
				if err != nil {
					self.logger.Debug("failed to store pod stats", "error", err)
					time.Sleep(time.Duration(self.updateInterval) * time.Second)
					continue
				}

				nodesResult := self.nodeStats(nodemetrics)
				err = self.statsDb.AddNodeStatsToDb(nodesResult)
				if err != nil {
					self.logger.Error("failed to store node stats", "stats", nodesResult, "error", err)
					time.Sleep(time.Duration(self.updateInterval) * time.Second)
					continue
				}

				time.Sleep(time.Duration(self.updateInterval) * time.Second)
			}
		}()
	}
}

func (self *podStatsCollector) nodeStats(nodemetrics []podstatscollector.NodeMetrics) []structs.NodeStats {
	result := make([]structs.NodeStats, 0, len(nodemetrics))

	for _, nodeMetric := range nodemetrics {
		entry := structs.NodeStats{}
		entry.Name = nodeMetric.Node.NodeName
		entry.StartTime = nodeMetric.Node.StartTime.Format(time.RFC3339)
		entry.PodCount = len(nodeMetric.Pods)
		entry.CpuUsageNanoCores = int64(nodeMetric.Node.CPU.UsageNanoCores)
		entry.MemoryUsageBytes = int64(nodeMetric.Node.Memory.UsageBytes)
		entry.MemoryAvailableBytes = int64(nodeMetric.Node.Memory.AvailableBytes)
		entry.MemoryWorkingSetBytes = int64(nodeMetric.Node.Memory.WorkingSetBytes)
		entry.NetworkTxBytes = int64(nodeMetric.Node.Network.TXBytes)
		entry.NetworkRxBytes = int64(nodeMetric.Node.Network.RXBytes)
		entry.FsAvailableBytes = int64(nodeMetric.Node.FS.AvailableBytes)
		entry.FsCapacityBytes = int64(nodeMetric.Node.FS.CapacityBytes)
		entry.FsUsedBytes = int64(nodeMetric.Node.FS.UsedBytes)
		result = append(result, entry)
	}

	return result
}

func (self *podStatsCollector) getRealNodeMetrics() []podstatscollector.NodeMetrics {
	result := []podstatscollector.NodeMetrics{}
	for _, node := range store.GetNodes() {
		nodeStats, err := self.requestMetricsDataFromNode(node.Name)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				self.logger.Warn("node no longer exists, removing from store", "node", node.Name)
				_ = store.DeleteNode(node.Name)
			} else {
				self.logger.Error("failed to request metrics from node", "error", err)
			}
			continue
		}
		result = append(result, *nodeStats)
	}

	return result
}

func (self *podStatsCollector) podStats(nodemetrics []podstatscollector.NodeMetrics, pods map[string]v1.Pod) ([]structs.PodStats, error) {
	// Pre-build ephemeral storage map for O(1) lookup instead of O(n³) nested loops
	// Key: "podName:containerName" -> ephemeralStorage in bytes
	ephemeralStorageMap := make(map[string]int64)
	for _, nodeMetric := range nodemetrics {
		for _, pod := range nodeMetric.Pods {
			for _, container := range pod.Containers {
				key := pod.PodRef.Name + ":" + container.Name
				ephemeralStorageMap[key] += int64(pod.EphemeralStorage.UsedBytes)
			}
		}
	}

	result := []structs.PodStats{}

	for _, nodeMetric := range nodemetrics {
		for _, kubeletPod := range nodeMetric.Pods {
			pod, exists := pods[kubeletPod.PodRef.Name]
			if !exists {
				continue
			}

			for _, container := range pod.Spec.Containers {
				if pod.Status.StartTime == nil {
					continue
				}
				entry := structs.PodStats{}
				entry.Namespace = kubeletPod.PodRef.Namespace
				entry.PodName = kubeletPod.PodRef.Name
				entry.StartTime = pod.Status.StartTime.Time

				entry.ContainerName = container.Name
				entry.CpuLimit += container.Resources.Limits.Cpu().MilliValue()
				entry.MemoryLimit += container.Resources.Limits.Memory().Value()
				entry.EphemeralStorageLimit += container.Resources.Limits.StorageEphemeral().Value()

				// Find matching container in kubelet stats and convert units
				for _, kubeletContainer := range kubeletPod.Containers {
					if kubeletContainer.Name == container.Name {
						// Convert nanocores to millicores (1 millicore = 1,000,000 nanocores)
						entry.Cpu += int64(kubeletContainer.CPU.UsageNanoCores) / 1_000_000
						// WorkingSetBytes matches what metrics-server reported for memory
						entry.Memory += int64(kubeletContainer.Memory.WorkingSetBytes)
						break
					}
				}

				// O(1) lookup instead of O(n³) nested loops
				key := kubeletPod.PodRef.Name + ":" + container.Name
				entry.EphemeralStorage = ephemeralStorageMap[key]

				result = append(result, entry)
			}
		}
	}

	return result, nil
}

func (self *podStatsCollector) requestMetricsDataFromNode(nodeName string) (*podstatscollector.NodeMetrics, error) {
	restClient := self.clientProvider.K8sClientSet().CoreV1().RESTClient()
	path := fmt.Sprintf("/api/v1/nodes/%s/proxy/stats/summary", nodeName)

	resultData := restClient.Get().AbsPath(path).Do(context.Background())
	if err := resultData.Error(); err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	rawResponse, err := resultData.Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to get raw response: %w", err)
	}

	result := &podstatscollector.NodeMetrics{}
	err = json.Unmarshal(rawResponse, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal metrics from Node(%s): %w", nodeName, err)
	}

	return result, nil
}
