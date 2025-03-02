package kubernetes

import (
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/redisstore"
	"mogenius-k8s-manager/src/structs"
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	DB_STATS_TRAFFIC_BUCKET_NAME    = "traffic-stats"
	DB_STATS_POD_STATS_BUCKET_NAME  = "pod-stats"
	DB_STATS_POD_EVENTS_NAME        = "pod-events"
	DB_STATS_NODE_STATS_BUCKET_NAME = "node-stats"
	DB_STATS_SOCKET_STATS_BUCKET    = "socket-stats"
	DB_STATS_CNI_BUCKET_NAME        = "cluster-cni-configuration"
)

var DefaultMaxSize int64 = 60 * 24 * 7

type RedisStatsDb interface {
	Start() error
	AddInterfaceStatsToDb(stats structs.InterfaceStats)
	AddNodeStatsToDb(stats structs.NodeStats)
	AddPodStatsToDb(stats structs.PodStats)
	GetCniData() ([]structs.CniData, error)
	GetLastPodStatsEntriesForNamespace(namespace string) []structs.PodStats
	GetLastPodStatsEntryForController(controller K8sController) *structs.PodStats
	GetPodStatsEntriesForController(controller K8sController) *[]structs.PodStats
	GetPodStatsEntriesForNamespace(namespace string) *[]structs.PodStats
	GetSocketConnectionsForController(controller K8sController) *structs.SocketConnections
	GetTrafficStatsEntriesForController(controller K8sController) *[]structs.InterfaceStats
	GetTrafficStatsEntriesForNamespace(namespace string) *[]structs.InterfaceStats
	GetTrafficStatsEntriesSumForNamespace(namespace string) []structs.InterfaceStats
	GetTrafficStatsEntrySumForController(controller K8sController, includeSocketConnections bool) *structs.InterfaceStats
	GetWorkspaceStatsCpuUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error)
	GetWorkspaceStatsMemoryUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error)
	GetWorkspaceStatsTrafficUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error)
	ReplaceCniData(data []structs.CniData)
}

type redisStatsDbModule struct {
	config cfg.ConfigModule
	logger *slog.Logger
	redis  redisstore.RedisStore
}

func NewRedisStatsModule(config cfg.ConfigModule, logger *slog.Logger) RedisStatsDb {
	redisStore := redisstore.NewRedis(logger)

	dbStatsModule := redisStatsDbModule{
		config: config,
		logger: logger,
		redis:  redisStore,
	}

	return &dbStatsModule
}

func (self *redisStatsDbModule) Start() error {
	err := self.redis.Connect()
	if err != nil {
		self.logger.Error("could not connect to Redis", "error", err)
	}
	return err
}

func (self *redisStatsDbModule) AddInterfaceStatsToDb(stats structs.InterfaceStats) {
	controller := ControllerForPod(stats.Namespace, stats.PodName)
	if controller == nil {
		return
	}

	stats.CreatedAt = time.Now().Format(time.RFC3339)
	err := self.redis.AddToBucket(DefaultMaxSize, stats.SocketConnections, DB_STATS_SOCKET_STATS_BUCKET, stats.Namespace, controller.Name)
	if err != nil {
		self.logger.Error("Error adding interface stats socketconnections", "namespace", stats.Namespace, "podName", stats.PodName, "error", err)
	}
	stats.SocketConnections = nil
	err = self.redis.AddToBucket(DefaultMaxSize, stats, DB_STATS_TRAFFIC_BUCKET_NAME, stats.Namespace, controller.Name)
	if err != nil {
		self.logger.Error("Error adding interface stats", "namespace", stats.Namespace, "podName", stats.PodName, "error", err)
	}
}

func (self *redisStatsDbModule) ReplaceCniData(data []structs.CniData) {
	for _, v := range data {
		err := self.redis.SetObject(data, 0, DB_STATS_CNI_BUCKET_NAME, v.Node)
		if err != nil {
			self.logger.Error("Error adding cni data", "node", v.Node, "error", err)
		}
	}
}

func (self *redisStatsDbModule) GetCniData() ([]structs.CniData, error) {
	result, err := redisstore.GetObjectsByPrefix[structs.CniData](self.redis.GetContext(), self.redis.GetClient(), redisstore.ORDER_NONE, DB_STATS_CNI_BUCKET_NAME)
	return result, err
}

func (self *redisStatsDbModule) GetPodStatsEntriesForController(controller K8sController) *[]structs.PodStats {
	result, err := redisstore.GetObjectsByPrefix[structs.PodStats](self.redis.GetContext(), self.redis.GetClient(), redisstore.ORDER_NONE, DB_STATS_POD_STATS_BUCKET_NAME, controller.Namespace, controller.Name)
	if err != nil {
		self.logger.Error("GetPodStatsEntriesForController", "error", err)
	}
	return &result
}

func (self *redisStatsDbModule) GetLastPodStatsEntryForController(controller K8sController) *structs.PodStats {
	values, err := redisstore.LastNEntryFromBucketWithType[structs.PodStats](self.redis.GetContext(), self.redis.GetClient(), 1, DB_STATS_POD_STATS_BUCKET_NAME, controller.Namespace, controller.Name)
	if err != nil {
		self.logger.Error(err.Error())
	}
	if len(values) > 0 {
		return &values[0]
	}
	return nil
}

func (self *redisStatsDbModule) GetTrafficStatsEntriesForController(controller K8sController) *[]structs.InterfaceStats {
	result, err := redisstore.GetObjectsByPrefix[structs.InterfaceStats](self.redis.GetContext(), self.redis.GetClient(), redisstore.ORDER_NONE, DB_STATS_TRAFFIC_BUCKET_NAME, controller.Namespace, controller.Name)
	if err != nil {
		self.logger.Error(err.Error())
	}
	return &result
}

func (self *redisStatsDbModule) GetTrafficStatsEntrySumForController(controller K8sController, includeSocketConnections bool) *structs.InterfaceStats {
	entries, err := redisstore.GetObjectsByPrefix[structs.InterfaceStats](self.redis.GetContext(), self.redis.GetClient(), redisstore.ORDER_DESC, DB_STATS_TRAFFIC_BUCKET_NAME, controller.Namespace, controller.Name)
	if err != nil {
		self.logger.Error(err.Error())
	}

	result := &structs.InterfaceStats{}
	for _, entry := range entries {
		if result.PodName == "" {
			result = &entry
		}
		if result.PodName != entry.PodName {
			// everytime the podName changes, sum up the values
			result.Sum(&entry)
			result.PodName = entry.PodName
		} else {
			// if the podName is the same, replace the values because it will be newer
			result.SumOrReplace(&entry)
		}
	}

	return result
}

func (self *redisStatsDbModule) GetWorkspaceStatsCpuUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error) {
	// setup min value
	if timeOffsetInMinutes < 5 {
		timeOffsetInMinutes = 5
	}
	if timeOffsetInMinutes > 60*24*7 {
		timeOffsetInMinutes = 60 * 24 * 7 // 7 days
	}

	result := make(map[string]GenericChartEntry)
	for _, controller := range resources {
		values, err := redisstore.LastNEntryFromBucketWithType[structs.PodStats](self.redis.GetContext(), self.redis.GetClient(), int64(timeOffsetInMinutes), DB_STATS_POD_STATS_BUCKET_NAME, controller.GetNamespace(), controller.GetName())
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

func (self *redisStatsDbModule) GetWorkspaceStatsMemoryUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error) {
	// setup min value
	if timeOffsetInMinutes < 5 {
		timeOffsetInMinutes = 5
	}
	if timeOffsetInMinutes > 60*24*7 {
		timeOffsetInMinutes = 60 * 24 * 7 // 7 days
	}

	result := make(map[string]GenericChartEntry)
	for _, controller := range resources {
		values, err := redisstore.LastNEntryFromBucketWithType[structs.PodStats](self.redis.GetContext(), self.redis.GetClient(), int64(timeOffsetInMinutes), DB_STATS_POD_STATS_BUCKET_NAME, controller.GetNamespace(), controller.GetName())
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

func (self *redisStatsDbModule) GetWorkspaceStatsTrafficUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error) {
	// setup min value
	if timeOffsetInMinutes < 5 {
		timeOffsetInMinutes = 5
	}
	if timeOffsetInMinutes > 60*24*7 {
		timeOffsetInMinutes = 60 * 24 * 7 // 7 days
	}

	result := make(map[string]GenericChartEntry)
	for _, controller := range resources {
		values, err := redisstore.LastNEntryFromBucketWithType[structs.InterfaceStats](self.redis.GetContext(), self.redis.GetClient(), int64(timeOffsetInMinutes), DB_STATS_TRAFFIC_BUCKET_NAME, controller.GetNamespace(), controller.GetName())
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
			result[formattedDate] = GenericChartEntry{Time: formattedDate, Value: result[formattedDate].Value + float64(entry.ReceivedBytes+entry.TransmitBytes)}

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

	// the entries in traffic are always incremental, so we need to normalize the values (11, 14, 16, 19 -> 3, 2, 3)
	for i := 0; i < len(sortedEntries); i++ {
		if i+1 < len(result) {
			normalized := sortedEntries[i].Value - sortedEntries[i+1].Value
			if normalized > 0 {
				sortedEntries[i].Value = normalized
			}
		}
	}
	// delete last entry of the array because it cannot be calculated correctly (because of the subtraction of the next value)
	if len(result) > 0 {
		sortedEntries = sortedEntries[:len(result)-1]
	}

	return sortedEntries, nil
}

func (self *redisStatsDbModule) GetSocketConnectionsForController(controller K8sController) *structs.SocketConnections {
	value, err := redisstore.GetObjectForKey[structs.SocketConnections](self.redis.GetContext(), self.redis.GetClient(), DB_STATS_SOCKET_STATS_BUCKET, controller.Namespace, controller.Name)
	if err != nil {
		self.logger.Error(err.Error())
	}

	return value
}

func (self *redisStatsDbModule) GetPodStatsEntriesForNamespace(namespace string) *[]structs.PodStats {
	values, err := redisstore.GetObjectsByPrefix[structs.PodStats](self.redis.GetContext(), self.redis.GetClient(), redisstore.ORDER_NONE, DB_STATS_POD_STATS_BUCKET_NAME, namespace)
	if err != nil {
		self.logger.Error(err.Error())
	}

	return &values
}

func (self *redisStatsDbModule) GetLastPodStatsEntriesForNamespace(namespace string) []structs.PodStats {
	values, err := redisstore.LastNEntryFromBucketWithType[structs.PodStats](self.redis.GetContext(), self.redis.GetClient(), 1, DB_STATS_POD_STATS_BUCKET_NAME, namespace)
	if err != nil {
		self.logger.Error(err.Error())
	}
	return values
}

func (self *redisStatsDbModule) GetTrafficStatsEntriesForNamespace(namespace string) *[]structs.InterfaceStats {
	values, err := redisstore.GetObjectsByPrefix[structs.InterfaceStats](self.redis.GetContext(), self.redis.GetClient(), redisstore.ORDER_DESC, DB_STATS_TRAFFIC_BUCKET_NAME, namespace)
	if err != nil {
		self.logger.Error(err.Error())
	}

	return &values
}

func (self *redisStatsDbModule) GetTrafficStatsEntriesSumForNamespace(namespace string) []structs.InterfaceStats {
	result := []structs.InterfaceStats{}

	// all keys in this namespace
	keys, err := self.redis.Keys(DB_STATS_TRAFFIC_BUCKET_NAME + ":" + namespace + ":*")
	if err != nil {
		self.logger.Error("GetTrafficStatsEntriesSumForNamespace", "error", err)
		return result
	}

	for _, entry := range keys {
		controllerName := entry[len(DB_STATS_TRAFFIC_BUCKET_NAME)+1+len(namespace)+1:]
		controller := NewK8sController("", string(controllerName), namespace)
		entry := self.GetTrafficStatsEntrySumForController(controller, false)
		if entry != nil {
			result = append(result, *entry)
		}

	}
	return result
}

func (self *redisStatsDbModule) AddPodStatsToDb(stats structs.PodStats) {
	controller := ControllerForPod(stats.Namespace, stats.PodName)
	if controller == nil {
		return
	}

	stats.CreatedAt = time.Now().Format(time.RFC3339)
	err := self.redis.AddToBucket(DefaultMaxSize, stats, DB_STATS_POD_STATS_BUCKET_NAME, stats.Namespace, controller.Name)
	if err != nil {
		self.logger.Error("Error adding pod stats", "namespace", stats.Namespace, "podName", stats.PodName, "error", err)
	}
}

func (self *redisStatsDbModule) AddNodeStatsToDb(stats structs.NodeStats) {
	stats.CreatedAt = time.Now().Format(time.RFC3339)
	err := self.redis.AddToBucket(DefaultMaxSize, stats, DB_STATS_NODE_STATS_BUCKET_NAME, stats.Name)
	if err != nil {
		self.logger.Error("Error adding node stats", "node", stats.Name, "error", err)
	}
}

type GenericChartEntry struct {
	Time  string  `json:"time"`
	Value float64 `json:"value"`
}
