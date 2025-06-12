package core

import (
	"fmt"
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/networkmonitor"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeyclient"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	DB_STATS_TRAFFIC_BUCKET_NAME       = "traffic-stats"
	DB_STATS_POD_STATS_BUCKET_NAME     = "pod-stats"
	DB_STATS_NODE_STATS_BUCKET_NAME    = "node-stats"
	DB_STATS_MACHINE_STATS_BUCKET_NAME = "machine-stats"
	DB_STATS_SOCKET_STATS_BUCKET       = "socket-stats"
	DB_STATS_CNI_BUCKET_NAME           = "cluster-cni-configuration"
	DB_STATS_LIVE_BUCKET_NAME          = "live-stats"
	DB_STATS_TRAFFIC_NAME              = "traffic"
	DB_STATS_CPU_NAME                  = "cpu"
	DB_STATS_MEMORY_NAME               = "memory"
)

var DefaultMaxSize int64 = 60 * 24 * 7
var DefaultMaxSizeSocketConnections int64 = 60

type ValkeyStatsDb interface {
	Run()
	AddInterfaceStatsToDb(stats []networkmonitor.PodNetworkStats)
	AddNodeStatsToDb(stats []structs.NodeStats) error
	AddMachineStatsToDb(nodeName string, stats structs.MachineStats) error
	AddPodStatsToDb(stats []structs.PodStats) error
	AddNodeRamMetricsToDb(nodeName string, data interface{}) error
	AddNodeCpuMetricsToDb(nodeName string, data interface{}) error
	AddNodeTrafficMetricsToDb(nodeName string, data interface{}) error
	AddSnoopyStatusToDb(nodeName string, data networkmonitor.SnoopyStatus) error
	GetCniData() ([]structs.CniData, error)
	GetLastPodStatsEntriesForNamespace(namespace string) []structs.PodStats
	GetLastPodStatsEntryForController(controller kubernetes.K8sController) *structs.PodStats
	GetMachineStatsForNode(nodeName string) (*structs.MachineStats, error)
	GetMachineStatsForNodes(nodeNames []string) []structs.MachineStats
	GetPodStatsEntriesForController(kind string, name string, namespace string, timeOffsetMinutes int64) *[]structs.PodStats
	GetPodStatsEntriesForNamespace(namespace string) *[]structs.PodStats
	GetSocketConnectionsForController(controller kubernetes.K8sController) *structs.SocketConnections
	GetTrafficStatsEntriesForController(kind string, name string, namespace string, timeOffsetMinutes int64) *[]networkmonitor.PodNetworkStats
	GetTrafficStatsEntriesForNamespace(namespace string) *[]networkmonitor.PodNetworkStats
	GetTrafficStatsEntriesSumForNamespace(namespace string) []networkmonitor.PodNetworkStats
	GetTrafficStatsEntrySumForController(controller kubernetes.K8sController, includeSocketConnections bool) *networkmonitor.PodNetworkStats
	GetWorkspaceStatsCpuUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error)
	GetWorkspaceStatsMemoryUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error)
	GetWorkspaceStatsTrafficUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error)
	ReplaceCniData(data []structs.CniData)
	Publish(data interface{}, keys ...string)
}

type valkeyStatsDb struct {
	config cfg.ConfigModule
	logger *slog.Logger
	valkey valkeyclient.ValkeyClient
}

func NewValkeyStatsModule(logger *slog.Logger, config cfg.ConfigModule, valkey valkeyclient.ValkeyClient) ValkeyStatsDb {
	dbStatsModule := valkeyStatsDb{
		config: config,
		logger: logger,
		valkey: valkey,
	}

	return &dbStatsModule
}

func (self *valkeyStatsDb) Run() {
	// clean valkey on every startup
	err := store.DropAllResourcesFromValkey(self.valkey, self.logger)
	if err != nil {
		self.logger.Error("Error dropping all resources from valkey", "error", err)
	}
	err = store.DropAllPodEventsFromValkey(self.valkey, self.logger)
	if err != nil {
		self.logger.Error("Error dropping all pod events from valkey", "error", err)
	}
}

func (self *valkeyStatsDb) AddMachineStatsToDb(nodeName string, stats structs.MachineStats) error {
	return self.valkey.SetObject(stats, 0, DB_STATS_MACHINE_STATS_BUCKET_NAME, nodeName)
}

func (self *valkeyStatsDb) GetMachineStatsForNode(nodeName string) (*structs.MachineStats, error) {
	machineStats, err := valkeyclient.GetObjectForKey[structs.MachineStats](self.valkey, DB_STATS_MACHINE_STATS_BUCKET_NAME, nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data as MachineStats: %s", err)
	}

	return machineStats, nil
}

func (self *valkeyStatsDb) AddInterfaceStatsToDb(stats []networkmonitor.PodNetworkStats) {
	for _, stat := range stats {
		controller := kubernetes.ControllerForPod(stat.Namespace, stat.Pod)
		if controller == nil {
			// in case we cannot determine a controller
			controller = &kubernetes.K8sController{
				Kind:      "Pod",
				Name:      stat.Pod,
				Namespace: stat.Namespace,
			}
		}

		err := self.valkey.AddToBucket(DefaultMaxSize, stat, DB_STATS_TRAFFIC_BUCKET_NAME, stat.Namespace, controller.Name)
		if err != nil {
			self.logger.Error("Error adding interface stats", "namespace", stat.Namespace, "podName", stat.Pod, "error", err)
		}
	}
}

func (self *valkeyStatsDb) ReplaceCniData(data []structs.CniData) {
	for _, v := range data {
		err := self.valkey.SetObject(data, 0, DB_STATS_CNI_BUCKET_NAME, v.Node)
		if err != nil {
			self.logger.Error("Error adding cni data", "node", v.Node, "error", err)
		}
	}
}

func (self *valkeyStatsDb) GetCniData() ([]structs.CniData, error) {
	result, err := valkeyclient.GetObjectsByPrefix[structs.CniData](self.valkey, valkeyclient.ORDER_NONE, DB_STATS_CNI_BUCKET_NAME)
	return result, err
}

func (self *valkeyStatsDb) GetPodStatsEntriesForController(kind string, name string, namespace string, timeOffsetMinutes int64) *[]structs.PodStats {
	result, err := valkeyclient.LastNEntryFromBucketWithType[structs.PodStats](self.valkey, timeOffsetMinutes, DB_STATS_POD_STATS_BUCKET_NAME, namespace, name)
	if err != nil {
		self.logger.Error("GetPodStatsEntriesForController", "error", err)
	}
	return &result
}

func (self *valkeyStatsDb) GetLastPodStatsEntryForController(controller kubernetes.K8sController) *structs.PodStats {
	values, err := valkeyclient.LastNEntryFromBucketWithType[structs.PodStats](self.valkey, 1, DB_STATS_POD_STATS_BUCKET_NAME, controller.Namespace, controller.Name)
	if err != nil {
		self.logger.Error(err.Error())
	}
	if len(values) > 0 {
		return &values[0]
	}
	return nil
}

func (self *valkeyStatsDb) GetTrafficStatsEntriesForController(kind string, name string, namespace string, timeOffsetMinutes int64) *[]networkmonitor.PodNetworkStats {
	result, err := valkeyclient.LastNEntryFromBucketWithType[networkmonitor.PodNetworkStats](self.valkey, timeOffsetMinutes, DB_STATS_TRAFFIC_BUCKET_NAME, namespace, name)
	if err != nil {
		self.logger.Error(err.Error())
	}
	return &result
}

func (self *valkeyStatsDb) GetTrafficStatsEntrySumForController(controller kubernetes.K8sController, includeSocketConnections bool) *networkmonitor.PodNetworkStats {
	entries, err := valkeyclient.GetObjectsByPrefix[networkmonitor.PodNetworkStats](self.valkey, valkeyclient.ORDER_DESC, DB_STATS_TRAFFIC_BUCKET_NAME, controller.Namespace, controller.Name)
	if err != nil {
		self.logger.Error(err.Error())
	}

	result := &networkmonitor.PodNetworkStats{}
	for _, entry := range entries {
		if result.Pod == "" {
			result = &entry
		}
		if result.Pod != entry.Pod {
			// everytime the podName changes, sum up the values
			result.Sum(&entry)
			result.Pod = entry.Pod
		} else {
			// if the podName is the same, replace the values because it will be newer
			result.SumOrReplace(&entry)
		}
	}

	return result
}

func (self *valkeyStatsDb) GetWorkspaceStatsCpuUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error) {
	// setup min value
	if timeOffsetInMinutes < 5 {
		timeOffsetInMinutes = 5
	}
	if timeOffsetInMinutes > 60*24*7 {
		timeOffsetInMinutes = 60 * 24 * 7 // 7 days
	}

	result := make(map[string]GenericChartEntry)
	for _, controller := range resources {
		values, err := valkeyclient.LastNEntryFromBucketWithType[structs.PodStats](self.valkey, int64(timeOffsetInMinutes), DB_STATS_POD_STATS_BUCKET_NAME, controller.GetNamespace(), controller.GetName())
		if err != nil {
			self.logger.Error(err.Error())
		}
		for index, entry := range values {
			parsedDate, err := time.Parse(time.RFC3339, entry.CreatedAt)
			if err != nil {
				continue
			}
			formattedDate := parsedDate.Round(time.Minute).Format(time.RFC3339)

			if _, exists := result[formattedDate]; !exists {
				result[formattedDate] = GenericChartEntry{Time: formattedDate, Value: 0.0}
			}
			result[formattedDate] = GenericChartEntry{Time: formattedDate, Value: result[formattedDate].Value + float64(entry.Cpu)}

			if index >= timeOffsetInMinutes {
				break
			}
		}
	}

	// SORT
	var keys []string
	for key := range result {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Step 2: Build a sorted slice of GenericChartEntry
	var sortedEntries []GenericChartEntry
	for _, key := range keys {
		sortedEntries = append(sortedEntries, result[key])
	}

	return sortedEntries, nil
}

func (self *valkeyStatsDb) GetWorkspaceStatsMemoryUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error) {
	// setup min value
	if timeOffsetInMinutes < 5 {
		timeOffsetInMinutes = 5
	}
	if timeOffsetInMinutes > 60*24*7 {
		timeOffsetInMinutes = 60 * 24 * 7 // 7 days
	}

	result := make(map[string]GenericChartEntry)
	for _, controller := range resources {
		values, err := valkeyclient.LastNEntryFromBucketWithType[structs.PodStats](self.valkey, int64(timeOffsetInMinutes), DB_STATS_POD_STATS_BUCKET_NAME, controller.GetNamespace(), controller.GetName())
		if err != nil {
			self.logger.Error(err.Error())
		}
		for index, entry := range values {
			parsedDate, err := time.Parse(time.RFC3339, entry.CreatedAt)
			if err != nil {
				continue
			}
			formattedDate := parsedDate.Round(time.Minute).Format(time.RFC3339)

			if _, exists := result[formattedDate]; !exists {
				result[formattedDate] = GenericChartEntry{Time: formattedDate, Value: 0.0}
			}
			result[formattedDate] = GenericChartEntry{Time: formattedDate, Value: result[formattedDate].Value + float64(entry.Memory)}

			if index >= timeOffsetInMinutes {
				break
			}
		}
	}

	// SORT
	var keys []string
	for key := range result {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Step 2: Build a sorted slice of GenericChartEntry
	var sortedEntries []GenericChartEntry
	for _, key := range keys {
		sortedEntries = append(sortedEntries, result[key])
	}

	return sortedEntries, nil
}

func (self *valkeyStatsDb) GetWorkspaceStatsTrafficUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error) {
	// Clamp to valid range
	if timeOffsetInMinutes < 5 {
		timeOffsetInMinutes = 5
	}
	if timeOffsetInMinutes > 60*24*7 {
		timeOffsetInMinutes = 60 * 24 * 7
	}

	// Aggregate all traffic by minute
	trafficByMinute := make(map[string]float64)

	for _, controller := range resources {
		values, err := valkeyclient.LastNEntryFromBucketWithType[networkmonitor.PodNetworkStats](
			self.valkey, int64(timeOffsetInMinutes), DB_STATS_TRAFFIC_BUCKET_NAME,
			controller.GetNamespace(), controller.GetName())
		if err != nil {
			self.logger.Error(err.Error())
			continue
		}

		// Add traffic for each minute
		for i, entry := range values {
			if i >= timeOffsetInMinutes {
				break
			}
			minute := entry.CreatedAt.Round(time.Minute).Format(time.RFC3339)
			trafficByMinute[minute] += float64(entry.TransmitBytes + entry.ReceivedBytes)
		}
	}

	// Sort minutes
	var minutes []string
	for minute := range trafficByMinute {
		minutes = append(minutes, minute)
	}
	sort.Strings(minutes)

	// Calculate deltas between consecutive minutes
	var result []GenericChartEntry
	for i := 0; i < len(minutes)-1; i++ {
		delta := trafficByMinute[minutes[i+1]] - trafficByMinute[minutes[i]]
		if delta < 0 {
			delta = 0
		}
		result = append(result, GenericChartEntry{
			Time:  minutes[i],
			Value: delta,
		})
	}

	return result, nil
}

func (self *valkeyStatsDb) GetSocketConnectionsForController(controller kubernetes.K8sController) *structs.SocketConnections {
	value, err := valkeyclient.GetObjectForKey[structs.SocketConnections](self.valkey, DB_STATS_SOCKET_STATS_BUCKET, controller.Namespace, controller.Name)
	if err != nil {
		self.logger.Error(err.Error())
	}

	return value
}

func (self *valkeyStatsDb) GetPodStatsEntriesForNamespace(namespace string) *[]structs.PodStats {
	values, err := valkeyclient.GetObjectsByPrefix[structs.PodStats](self.valkey, valkeyclient.ORDER_NONE, DB_STATS_POD_STATS_BUCKET_NAME, namespace)
	if err != nil {
		self.logger.Error(err.Error())
	}

	return &values
}

func (self *valkeyStatsDb) GetLastPodStatsEntriesForNamespace(namespace string) []structs.PodStats {
	values, err := valkeyclient.LastNEntryFromBucketWithType[structs.PodStats](self.valkey, 1, DB_STATS_POD_STATS_BUCKET_NAME, namespace)
	if err != nil {
		self.logger.Error(err.Error())
	}
	return values
}

func (self *valkeyStatsDb) GetTrafficStatsEntriesForNamespace(namespace string) *[]networkmonitor.PodNetworkStats {
	values, err := valkeyclient.GetObjectsByPrefix[networkmonitor.PodNetworkStats](self.valkey, valkeyclient.ORDER_DESC, DB_STATS_TRAFFIC_BUCKET_NAME, namespace)
	if err != nil {
		self.logger.Error(err.Error())
	}

	return &values
}

func (self *valkeyStatsDb) GetTrafficStatsEntriesSumForNamespace(namespace string) []networkmonitor.PodNetworkStats {
	result := []networkmonitor.PodNetworkStats{}

	// all keys in this namespace
	keys, err := self.valkey.Keys(DB_STATS_TRAFFIC_BUCKET_NAME + ":" + namespace + ":*")
	if err != nil {
		self.logger.Error("GetTrafficStatsEntriesSumForNamespace", "error", err)
		return result
	}

	for _, entry := range keys {
		controllerName := entry[len(DB_STATS_TRAFFIC_BUCKET_NAME)+1+len(namespace)+1:]
		controller := kubernetes.NewK8sController("", string(controllerName), namespace)
		entry := self.GetTrafficStatsEntrySumForController(controller, false)
		if entry != nil {
			result = append(result, *entry)
		}

	}
	return result
}

func (self *valkeyStatsDb) AddPodStatsToDb(stats []structs.PodStats) error {
	for _, stat := range stats {
		controller := kubernetes.ControllerForPod(stat.Namespace, stat.PodName)
		if controller == nil {
			return fmt.Errorf("controller not found for pod %s in namespace %s", stat.PodName, stat.Namespace)
		}

		stat.CreatedAt = time.Now().Format(time.RFC3339)
		err := self.valkey.AddToBucket(DefaultMaxSize, stat, DB_STATS_POD_STATS_BUCKET_NAME, stat.Namespace, controller.Name)
		if err != nil {
			return fmt.Errorf("error adding pod stats: %s", err)
		}
	}
	return nil
}

func (self *valkeyStatsDb) AddNodeRamMetricsToDb(nodeName string, data interface{}) error {
	self.Publish(data, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_MEMORY_NAME, nodeName)
	return self.valkey.SetObject(data, 0, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_MEMORY_NAME, nodeName)
}

func (self *valkeyStatsDb) AddNodeCpuMetricsToDb(nodeName string, data interface{}) error {
	self.Publish(data, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_CPU_NAME, nodeName)
	return self.valkey.SetObject(data, 0, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_CPU_NAME, nodeName)
}

func (self *valkeyStatsDb) AddNodeTrafficMetricsToDb(nodeName string, data interface{}) error {
	self.Publish(data, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_TRAFFIC_NAME, nodeName)
	return self.valkey.SetObject(data, 0, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_TRAFFIC_NAME, nodeName)
}

func (self *valkeyStatsDb) AddSnoopyStatusToDb(nodeName string, data networkmonitor.SnoopyStatus) error {
	return self.valkey.SetObject(data, 0, "status", nodeName, "snoopy")
}

func (self *valkeyStatsDb) AddNodeStatsToDb(stats []structs.NodeStats) error {
	for _, stat := range stats {
		stat.CreatedAt = time.Now().Format(time.RFC3339)
		err := self.valkey.AddToBucket(DefaultMaxSize, stat, DB_STATS_NODE_STATS_BUCKET_NAME, stat.Name)
		if err != nil {
			return fmt.Errorf("error adding node stats: %s", err)
		}
	}
	return nil
}

func (self *valkeyStatsDb) Publish(data interface{}, keys ...string) {
	key := strings.Join(keys, ":")
	err := self.valkey.GetRedisClient().Publish(self.valkey.GetContext(), key, utils.PrintJson(data)).Err()

	if err != nil {
		self.logger.Error("Error publishing to Redis", "error", err, "key", key)
	}
}

type GenericChartEntry struct {
	Time  string  `json:"time"`
	Value float64 `json:"value"`
}

func (self *valkeyStatsDb) GetMachineStatsForNodes(nodes []string) []structs.MachineStats {
	result := []structs.MachineStats{}

	for _, node := range nodes {
		stat, err := self.GetMachineStatsForNode(node)
		if err != nil {
			self.logger.Error("failed to get machine stats for node", "node", node, "error", err)
			continue
		}
		if stat != nil {
			result = append(result, *stat)
		}
	}

	return result
}
