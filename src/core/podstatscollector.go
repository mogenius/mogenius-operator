package core

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/podstatscollector"
	"mogenius-k8s-manager/src/structs"
	"strconv"
	"time"

	jsoniter "github.com/json-iterator/go"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
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
				pods := self.listAllPods()
				for _, pod := range pods {
					currentPods[pod.Name] = pod
				}

				podsResult, err := self.podStats(nodemetrics, currentPods)
				if err != nil {
					self.logger.Error("failed to get podStats", "error", err)
					time.Sleep(time.Duration(self.updateInterval) * time.Second)
					continue
				}
				err = self.statsDb.AddPodStatsToDb(podsResult)
				if err != nil {
					self.logger.Error("failed to store pod stats", "error", err)
					time.Sleep(time.Duration(self.updateInterval) * time.Second)
					continue
				}

				nodesResult := self.nodeStats(nodemetrics)
				err = self.statsDb.AddNodeStatsToDb(nodesResult)
				if err != nil {
					self.logger.Error("failed to store node stats", "error", err)
					time.Sleep(time.Duration(self.updateInterval) * time.Second)
					continue
				}

				time.Sleep(time.Duration(self.updateInterval) * time.Second)
			}
		}()
	}
}

func (self *podStatsCollector) listAllPods() []v1.Pod {
	result := []v1.Pod{}

	pods, err := self.clientProvider.K8sClientSet().CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		self.logger.Error("failed to listAllPods", "error", err)
		return result
	}

	return pods.Items
}

func (self *podStatsCollector) nodeStats(nodemetrics []podstatscollector.NodeMetrics) []structs.NodeStats {
	result := []structs.NodeStats{}

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
	nodeList, err := self.clientProvider.K8sClientSet().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		self.logger.Error("failed to getRealNodeMetrics", "error", err)
	}

	result := []podstatscollector.NodeMetrics{}
	for _, node := range nodeList.Items {
		nodeStats, err := self.requestMetricsDataFromNode(node.Name)
		if err != nil {
			self.logger.Error("failed to request metrics from node", "error", err)
			continue
		}
		result = append(result, *nodeStats)
	}

	return result
}

func (self *podStatsCollector) podStats(nodemetrics []podstatscollector.NodeMetrics, pods map[string]v1.Pod) ([]structs.PodStats, error) {
	// Create a cache for pod metrics to avoid redundant lookups
	podMetricsCache := make(map[string]*metricsv1beta1.PodMetrics)
	
	// Batch fetch all pod metrics at once
	podMetricsList, err := self.clientProvider.MetricsClientSet().MetricsV1beta1().PodMetricses("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Build cache
	for i := range podMetricsList.Items {
		podMetricsCache[podMetricsList.Items[i].Name] = &podMetricsList.Items[i]
	}

	result := make([]structs.PodStats, 0, len(pods))
	
	// Process pods in batches
	for podName, pod := range pods {
		podMetrics, exists := podMetricsCache[podName]
		if !exists {
			continue
		}

		// Create a map of container metrics for quick lookup
		containerMetricsMap := make(map[string]metricsv1beta1.ContainerMetrics)
		for _, cm := range podMetrics.Containers {
			containerMetricsMap[cm.Name] = cm
		}

		// Process all containers for this pod at once
		for _, container := range pod.Spec.Containers {
			entry := structs.PodStats{
				Namespace:     podMetrics.Namespace,
				PodName:      podName,
				ContainerName: container.Name,
				StartTime:    pod.Status.StartTime.Format(time.RFC3339),
			}

			// Set resource limits
			entry.CpuLimit = container.Resources.Limits.Cpu().MilliValue()
			entry.MemoryLimit = container.Resources.Limits.Memory().Value()
			entry.EphemeralStorageLimit = container.Resources.Limits.StorageEphemeral().Value()

			// Get container metrics from cache
			if cm, ok := containerMetricsMap[container.Name]; ok {
				entry.Cpu = cm.Usage.Cpu().MilliValue()
				entry.Memory = cm.Usage.Memory().Value()
			}

			// Find ephemeral storage usage from node metrics
			for _, nodeMetric := range nodemetrics {
				for _, pod := range nodeMetric.Pods {
					if pod.PodRef.Name == podName {
						for _, metricContainer := range pod.Containers {
							if metricContainer.Name == container.Name {
								entry.EphemeralStorage = int64(pod.EphemeralStorage.UsedBytes)
								break
							}
						}
					}
				}
			}

			result = append(result, entry)
		}
	}

	return result, nil
}

func (self *podStatsCollector) requestMetricsDataFromNode(nodeName string) (*podstatscollector.NodeMetrics, error) {
	restClient := self.clientProvider.K8sClientSet().CoreV1().RESTClient()
	path := fmt.Sprintf("/api/v1/nodes/%s/proxy/stats/summary", nodeName)

	resultData := restClient.Get().AbsPath(path).Do(context.TODO())
	if err := resultData.Error(); err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}

	rawResponse, err := resultData.Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to get raw response: %v", err)
	}

	result := &podstatscollector.NodeMetrics{}
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(rawResponse, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal metrics from Node(%s): %v", nodeName, err)
	}

	return result, nil
}
