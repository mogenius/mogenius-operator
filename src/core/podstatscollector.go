package core

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/podstatscollector"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/structs"
	"strconv"
	"time"

	jsoniter "github.com/json-iterator/go"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	result := []podstatscollector.NodeMetrics{}
	for _, node := range store.GetNodes() {
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
	podMetricsList, err := self.clientProvider.MetricsClientSet().MetricsV1beta1().PodMetricses("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := []structs.PodStats{}
	// bene: I HATE THIS BUT I DONT SEE ANY OTHER SOLUTION! SPEND HOURS (to find something better) ON THIS UGGLY SHIT!!!!

	for _, podMetrics := range podMetricsList.Items {
		pod := pods[podMetrics.Name]

		for _, container := range pod.Spec.Containers {
			if pod.Status.StartTime == nil {
				continue
			}
			entry := structs.PodStats{}
			entry.Namespace = podMetrics.Namespace
			entry.PodName = podMetrics.Name
			entry.StartTime = pod.Status.StartTime.Time

			entry.ContainerName = container.Name
			entry.CpuLimit += container.Resources.Limits.Cpu().MilliValue()
			entry.MemoryLimit += container.Resources.Limits.Memory().Value()
			entry.EphemeralStorageLimit += container.Resources.Limits.StorageEphemeral().Value()

			for _, containerMetric := range podMetrics.Containers {
				if containerMetric.Name == container.Name {
					entry.Cpu += containerMetric.Usage.Cpu().MilliValue()
					entry.Memory += containerMetric.Usage.Memory().Value()
				}
			}
			for _, nodeMetric := range nodemetrics {
				for _, pod := range nodeMetric.Pods {
					for _, metricContainer := range pod.Containers {
						if metricContainer.Name == container.Name && pod.PodRef.Name == podMetrics.Name {
							entry.EphemeralStorage += int64(pod.EphemeralStorage.UsedBytes)
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

	resultData := restClient.Get().AbsPath(path).Do(context.Background())
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
