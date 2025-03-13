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
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeyclient"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	jsoniter "github.com/json-iterator/go"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodStatsCollector interface {
	Run()
}

type podStatsCollector struct {
	logger         *slog.Logger
	clientProvider k8sclient.K8sClientProvider
	valkey         valkeyclient.ValkeyClient
	config         config.ConfigModule

	startTime      time.Time
	updateInterval uint64
}

func NewPodStatsCollector(
	logger *slog.Logger,
	configModule config.ConfigModule,
	clientProviderModule k8sclient.K8sClientProvider,
	valkey valkeyclient.ValkeyClient,
) PodStatsCollector {
	self := &podStatsCollector{}

	self.logger = logger
	self.config = configModule
	self.clientProvider = clientProviderModule
	self.valkey = valkey
	self.startTime = time.Now()
	self.updateInterval = 60

	return self
}

func (self *podStatsCollector) Run() {
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
					continue
				}
				err = self.valkey.SetObject(podsResult, time.Duration(0), "stats", "pod-stats-collector", "pods")
				if err != nil {
					self.logger.Error("failed to store pod stats", "error", err)
					continue
				}

				nodesResult := self.nodeStats(nodemetrics)
				err = self.valkey.SetObject(nodesResult, time.Duration(0), "stats", "pod-stats-collector", "nodes")
				if err != nil {
					self.logger.Error("failed to store node stats", "error", err)
					continue
				}

				self.printEntriesTable(podsResult)

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
	if !self.clientProvider.RunsInCluster() {
		self.logger.Warn("MAKE SURE YOU RUN 'kubectl proxy' to use this function in local config mode.")
	}

	nodeList, err := self.clientProvider.K8sClientSet().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		self.logger.Error("failed to getRealNodeMetrics", "error", err)
	}

	result := []podstatscollector.NodeMetrics{}
	for _, node := range nodeList.Items {
		self.logger.Info("requesting metrics from node", "node", node.Name)
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
	podMetricsList, err := self.clientProvider.MetricsClientSet().MetricsV1beta1().PodMetricses("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := []structs.PodStats{}
	// bene: I HATE THIS BUT I DONT SEE ANY OTHER SOLUTION! SPEND HOURS (to find something better) ON THIS UGGLY SHIT!!!!

	for _, podMetrics := range podMetricsList.Items {
		pod := pods[podMetrics.Name]

		for _, container := range pod.Spec.Containers {
			entry := structs.PodStats{}
			entry.Namespace = podMetrics.Namespace
			entry.PodName = podMetrics.Name
			entry.StartTime = pod.Status.StartTime.Format(time.RFC3339)

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

// Periodically print Information to stdout (statistics/debug/general information)
func (self *podStatsCollector) printEntriesTable(stats []structs.PodStats) {
	var totalCpu int64 = 0
	var totalCpuLimit int64 = 0
	var totalMemory int64 = 0
	var totalMemoryLimit int64 = 0
	var totalEphemeral int64 = 0
	var totalEphemeralLimit int64 = 0

	for _, data := range stats {
		totalCpu += data.Cpu
		totalCpuLimit += data.CpuLimit
		totalMemory += data.Memory
		totalMemoryLimit += data.MemoryLimit
		totalEphemeral += data.EphemeralStorage
		totalEphemeralLimit += data.EphemeralStorageLimit
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(
		table.Row{
			"Namespace",
			fmt.Sprintf("PODS (since %s)",
				utils.HumanDuration(time.Since(self.startTime))),
			"Container",
			"CPU %",
			"Cpu",
			"CpuLimit",
			"Memory",
			"MemoryLimit",
			"Ephemeral",
			"EphemeralLimit",
			"Started",
		},
	)
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Cpu < stats[j].Cpu
	})
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Namespace < stats[j].Namespace
	})
	for _, entry := range stats {
		var usagePercent float64 = 0
		if entry.CpuLimit > 0 {
			usagePercent = float64(entry.Cpu) / float64(entry.CpuLimit) * 100
		}

		t.AppendRow(
			table.Row{
				entry.Namespace,
				entry.PodName,
				entry.ContainerName,
				fmt.Sprintf("%.2f", usagePercent),
				entry.Cpu,
				entry.CpuLimit,
				utils.BytesToHumanReadable(entry.Memory),
				utils.BytesToHumanReadable(entry.MemoryLimit),
				utils.BytesToHumanReadable(entry.EphemeralStorage),
				utils.BytesToHumanReadable(entry.EphemeralStorageLimit),
				utils.JsonStringToHumanDuration(entry.StartTime),
			},
		)
	}
	t.AppendSeparator()
	t.AppendFooter(
		table.Row{
			"Namespaces",
			"Pods",
			"Containers",
			"",
			"Cpu",
			"CpuLimit",
			"Memory",
			"MemoryLimit",
			"Ephemeral",
			"EphemeralLimit",
			"",
		},
	)
	t.AppendFooter(
		table.Row{
			self.countNamespaces(stats),
			self.countPods(stats),
			len(stats),
			"",
			totalCpu,
			totalCpuLimit,
			utils.BytesToHumanReadable(totalMemory),
			utils.BytesToHumanReadable(totalMemoryLimit),
			utils.BytesToHumanReadable(totalEphemeral),
			utils.BytesToHumanReadable(totalEphemeralLimit),
			"",
		},
	)
	t.Render()

	debugTable := table.NewWriter()
	debugTable.SetOutputMirror(os.Stdout)
	debugTable.AppendHeader(table.Row{"since", "Pods"})
	debugTable.AppendSeparator()
	debugTable.AppendRow(
		table.Row{utils.HumanDuration(time.Since(self.startTime)), len(stats)},
	)
	debugTable.Render()
}

func (self *podStatsCollector) countPods(stats []structs.PodStats) int {
	mapPods := make(map[string]bool)
	for _, container := range stats {
		mapPods[container.PodName] = true
	}
	return len(mapPods)
}

func (self *podStatsCollector) countNamespaces(stats []structs.PodStats) int {
	mapNamespaces := make(map[string]bool)
	for _, container := range stats {
		mapNamespaces[container.Namespace] = true
	}
	return len(mapNamespaces)
}
