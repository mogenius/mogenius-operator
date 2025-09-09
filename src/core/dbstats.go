package core

import (
	"fmt"
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/networkmonitor"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeyclient"
	"sort"
	"strings"
	"sync"
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
	DB_STATS_PROCESSES_NAME            = "proc"
)

var DefaultMaxSizeSocketConnections int64 = 60

var lastNetworkPodStats = make(map[string]networkmonitor.PodNetworkStats)

type ValkeyStatsDb interface {
	Run()
	AddInterfaceStatsToDb(stats []networkmonitor.PodNetworkStats)
	AddNodeStatsToDb(stats []structs.NodeStats) error
	AddMachineStatsToDb(nodeName string, stats structs.MachineStats) error
	AddPodStatsToDb(stats []structs.PodStats) error
	AddNodeRamMetricsToDb(nodeName string, data interface{}) error
	AddNodeRamProcessMetricsToDb(nodeName string, data interface{}) error
	AddNodeCpuMetricsToDb(nodeName string, data interface{}) error
	AddNodeCpuProcessMetricsToDb(nodeName string, data interface{}) error
	AddNodeTrafficMetricsToDb(nodeName string, data interface{}) error
	AddSnoopyStatusToDb(nodeName string, data networkmonitor.SnoopyStatus) error
	GetCniData() ([]structs.CniData, error)
	// GetLastPodStatsEntriesForNamespace(namespace string) []structs.PodStats
	// GetLastPodStatsEntryForController(controller dtos.K8sController) *structs.PodStats
	GetMachineStatsForNode(nodeName string) (*structs.MachineStats, error)
	GetMachineStatsForNodes(nodeNames []string) []structs.MachineStats
	GetPodStatsEntriesForController(kind string, name string, namespace string, timeOffsetMinutes int64) *[]structs.PodStats
	GetPodStatsEntriesForNamespace(namespace string) *[]structs.PodStats
	//GetSocketConnectionsForController(controller dtos.K8sController) *structs.SocketConnections
	GetTrafficStatsEntriesForController(kind string, name string, namespace string, timeOffsetMinutes int64) *[]networkmonitor.PodNetworkStats
	GetTrafficStatsEntriesForNamespace(namespace string) *[]networkmonitor.PodNetworkStats
	GetTrafficStatsEntriesSumForNamespace(namespace string) []networkmonitor.PodNetworkStats
	GetTrafficStatsEntrySumForController(controller dtos.K8sController, includeSocketConnections bool) *networkmonitor.PodNetworkStats
	GetWorkspaceStatsCpuUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error)
	GetWorkspaceStatsMemoryUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error)
	GetWorkspaceStatsTrafficUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error)
	ReplaceCniData(data []structs.CniData)
	Publish(data interface{}, keys ...string)
}

type valkeyStatsDb struct {
	config            cfg.ConfigModule
	logger            *slog.Logger
	valkey            valkeyclient.ValkeyClient
	ownerCacheService OwnerCacheService
}

func NewValkeyStatsModule(logger *slog.Logger, config cfg.ConfigModule, valkey valkeyclient.ValkeyClient, ownerCacheService OwnerCacheService) ValkeyStatsDb {
	dbStatsModule := valkeyStatsDb{
		config:            config,
		logger:            logger,
		valkey:            valkey,
		ownerCacheService: ownerCacheService,
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
		controller := self.ownerCacheService.ControllerForPod(stat.Namespace, stat.Pod)
		if controller == nil {
			// in case we cannot determine a controller
			controller = &dtos.K8sController{
				Kind:      "Pod",
				Name:      stat.Pod,
				Namespace: stat.Namespace,
			}
		}

		// Compute deltas if we have previous stats
		if entry, exist := lastNetworkPodStats[stat.Pod]; exist {
			// calculate delta
			stat.TransmitBytes = stat.TransmitBytes - entry.TransmitBytes
			stat.ReceivedBytes = stat.ReceivedBytes - entry.ReceivedBytes
			stat.ReceivedPackets = stat.ReceivedPackets - entry.ReceivedPackets
			stat.TransmitPackets = stat.TransmitPackets - entry.TransmitPackets
		} else {
			// start from zero
			stat.TransmitBytes = 0
			stat.ReceivedBytes = 0
			stat.ReceivedPackets = 0
			stat.TransmitPackets = 0
		}

		lastNetworkPodStats[stat.Pod] = stat

		// err := self.valkey.AddToBucket(DefaultMaxSize, stat, DB_STATS_TRAFFIC_BUCKET_NAME, stat.Namespace, controller.Name)
		err := self.valkey.StoreSortedListEntry(stat, time.Now().Truncate(time.Minute).Unix(), DB_STATS_TRAFFIC_BUCKET_NAME, stat.Namespace, controller.Name)
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
	result, err := valkeyclient.GetObjectsFromSortedListWithDuration[structs.PodStats](self.valkey, timeOffsetMinutes, DB_STATS_POD_STATS_BUCKET_NAME, namespace, name)
	if err != nil {
		self.logger.Error("GetPodStatsEntriesForController", "error", err)
	}
	return &result
}

func (self *valkeyStatsDb) GetTrafficStatsEntriesForController(kind string, name string, namespace string, timeOffsetMinutes int64) *[]networkmonitor.PodNetworkStats {
	result, err := valkeyclient.GetObjectsFromSortedListWithDuration[networkmonitor.PodNetworkStats](self.valkey, timeOffsetMinutes, DB_STATS_TRAFFIC_BUCKET_NAME, namespace, name)
	if err != nil {
		self.logger.Error(err.Error())
	}
	return &result
}

func (self *valkeyStatsDb) GetTrafficStatsEntrySumForController(controller dtos.K8sController, includeSocketConnections bool) *networkmonitor.PodNetworkStats {
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

func (self *valkeyStatsDb) GetWorkspaceStatsCpuUtilization(
	timeOffsetInMinutes int,
	resources []unstructured.Unstructured,
) ([]GenericChartEntry, error) {

	// Clamp to valid range
	if timeOffsetInMinutes < 5 {
		timeOffsetInMinutes = 5
	}
	if timeOffsetInMinutes > 60*24*7 {
		timeOffsetInMinutes = 60 * 24 * 7 // 7 days
	}

	result := make(map[time.Time]GenericChartEntry)
	var mu sync.Mutex

	promise := utils.NewPromise[structs.PodStats]()

	for _, controller := range resources {
		ns, name := controller.GetNamespace(), controller.GetName()

		promise.RunArray(func() *[]structs.PodStats {
			values, err := valkeyclient.GetObjectsFromSortedListWithDuration[structs.PodStats](self.valkey, int64(timeOffsetInMinutes), DB_STATS_POD_STATS_BUCKET_NAME, ns, name)
			if err != nil {
				self.logger.Error(err.Error())
				return nil
			}

			// Aggregate locally for this resource
			localAgg := make(map[time.Time]GenericChartEntry)

			for i, entry := range values {
				if i >= timeOffsetInMinutes {
					break
				}

				if entry.CreatedAt.IsZero() {
					continue
				}
				minute := entry.CreatedAt.Truncate(time.Minute)

				e := localAgg[minute]
				if e.Pods == nil {
					e = GenericChartEntry{
						Time: minute,
						Pods: make(map[string]float64),
					}
				}

				cpu := float64(entry.Cpu)
				e.Value += cpu
				e.Pods = updateTop5Pods(e.Pods, cpu, entry.PodName)

				localAgg[minute] = e
			}

			// Merge local aggregation into global map
			mu.Lock()
			for t, e := range localAgg {
				g := result[t]
				if g.Pods == nil {
					g = GenericChartEntry{
						Time: e.Time,
						Pods: make(map[string]float64),
					}
				}
				g.Value += e.Value
				for pod, v := range e.Pods {
					g.Pods = updateTop5Pods(g.Pods, v, pod)
				}
				result[t] = g
			}
			mu.Unlock()

			return &values
		})
	}

	_ = promise.Wait()

	// Collect timestamps
	times := make([]time.Time, 0, len(result))
	for t := range result {
		times = append(times, t)
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })

	// Build ordered result
	sortedEntries := make([]GenericChartEntry, 0, len(times))
	for _, t := range times {
		sortedEntries = append(sortedEntries, result[t])
	}

	return sortedEntries, nil
}

func updateTop5Pods(pods map[string]float64, compareValue float64, podName string) map[string]float64 {
	if smallestKey, smallestValue, found := findSmallest(pods); found {
		if len(pods) >= 5 {
			pods[smallestKey] = compareValue
		} else {
			if smallestValue < compareValue {
				pods[podName] = compareValue
			}
		}
	} else {
		pods[podName] = compareValue
	}
	return pods
}

func (self *valkeyStatsDb) GetWorkspaceStatsMemoryUtilization(
	timeOffsetInMinutes int,
	resources []unstructured.Unstructured,
) ([]GenericChartEntry, error) {

	// Clamp ranges
	if timeOffsetInMinutes < 5 {
		timeOffsetInMinutes = 5
	}
	if timeOffsetInMinutes > 60*24*7 {
		timeOffsetInMinutes = 60 * 24 * 7 // 7 days
	}

	result := make(map[time.Time]GenericChartEntry)
	var mu sync.Mutex

	promise := utils.NewPromise[structs.PodStats]()

	for _, controller := range resources {
		ns := controller.GetNamespace()
		name := controller.GetName()

		promise.RunArray(func() *[]structs.PodStats {
			values, err := valkeyclient.GetObjectsFromSortedListWithDuration[structs.PodStats](self.valkey, int64(timeOffsetInMinutes), DB_STATS_POD_STATS_BUCKET_NAME, ns, name)
			if err != nil {
				self.logger.Error(err.Error())
				return nil
			}

			// Local aggregation per goroutine
			localAgg := make(map[time.Time]GenericChartEntry)

			for i, entry := range values {
				if i >= timeOffsetInMinutes {
					break
				}

				if entry.CreatedAt.IsZero() {
					continue
				}
				minute := entry.CreatedAt.Truncate(time.Minute)

				e := localAgg[minute]
				if e.Pods == nil {
					e = GenericChartEntry{
						Time:  minute,
						Value: 0.0,
						Pods:  map[string]float64{},
					}
				}

				mem := float64(entry.Memory)
				e.Value += mem
				e.Pods = updateTop5Pods(e.Pods, mem, entry.PodName)
				localAgg[minute] = e
			}

			// Merge aggregated values into global map under one lock
			mu.Lock()
			for t, e := range localAgg {
				g := result[t]
				if g.Pods == nil {
					g = GenericChartEntry{
						Time:  e.Time,
						Value: 0.0,
						Pods:  map[string]float64{},
					}
				}
				g.Value += e.Value
				for pod, v := range e.Pods {
					g.Pods = updateTop5Pods(g.Pods, v, pod)
				}
				result[t] = g
			}
			mu.Unlock()

			return &values
		})
	}

	_ = promise.Wait()

	// Collect keys and sort them chronologically
	times := make([]time.Time, 0, len(result))
	for t := range result {
		times = append(times, t)
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })

	// Build sorted slice of results
	sortedEntries := make([]GenericChartEntry, 0, len(times))
	for _, t := range times {
		sortedEntries = append(sortedEntries, result[t])
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
	trafficByMinute := make(map[time.Time]GenericChartEntry)

	promise := utils.NewPromise[networkmonitor.PodNetworkStats]()
	var resultMutex sync.Mutex
	for _, controller := range resources {
		promise.RunArray(func() *[]networkmonitor.PodNetworkStats {
			values, err := valkeyclient.GetObjectsFromSortedListWithDuration[networkmonitor.PodNetworkStats](self.valkey, int64(timeOffsetInMinutes), DB_STATS_TRAFFIC_BUCKET_NAME, controller.GetNamespace(), controller.GetName())
			if err != nil {
				self.logger.Error(err.Error())
				return nil
			}

			// Add traffic for each minute
			for _, entry := range values {
				minute := entry.CreatedAt.Round(time.Minute)
				resultMutex.Lock()
				if existingEntry, exists := trafficByMinute[minute]; !exists {
					trafficByMinute[minute] = GenericChartEntry{
						Time:  minute,
						Value: float64(entry.TransmitBytes + entry.ReceivedBytes),
						Pods: map[string]float64{
							entry.Pod: float64(entry.TransmitBytes + entry.ReceivedBytes),
						},
					}
				} else {
					trafficByMinute[minute] = GenericChartEntry{
						Time:  minute,
						Value: existingEntry.Value + float64(entry.TransmitBytes+entry.ReceivedBytes),
						Pods:  updateTop5Pods(existingEntry.Pods, float64(entry.TransmitBytes+entry.ReceivedBytes), entry.Pod),
					}
				}
				resultMutex.Unlock()
			}
			return &values
		})
	}
	_ = promise.Wait()

	// Collect keys and sort them chronologically
	times := make([]time.Time, 0, len(trafficByMinute))
	for t := range trafficByMinute {
		times = append(times, t)
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })

	// Build sorted slice of results
	sortedEntries := make([]GenericChartEntry, 0, len(times))
	for i := 1; i < len(times); i++ {
		currentEntry := trafficByMinute[times[i]]
		prevEntry := trafficByMinute[times[i-1]]

		// Calculate delta, handle resets/negative values
		delta := currentEntry.Value - prevEntry.Value
		if delta < 0 {
			// Counter reset or restart - use current value as delta
			delta = currentEntry.Value
			// Or skip this entry entirely:
			// continue
		}

		currentEntry.Value = delta
		sortedEntries = append(sortedEntries, currentEntry)
	}

	return sortedEntries, nil
}

func (self *valkeyStatsDb) GetPodStatsEntriesForNamespace(namespace string) *[]structs.PodStats {
	values, err := valkeyclient.GetObjectsByPrefix[structs.PodStats](self.valkey, valkeyclient.ORDER_NONE, DB_STATS_POD_STATS_BUCKET_NAME, namespace)
	if err != nil {
		self.logger.Error(err.Error())
	}

	return &values
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
		controller := dtos.NewK8sController("", string(controllerName), namespace)
		entry := self.GetTrafficStatsEntrySumForController(controller, false)
		if entry != nil {
			result = append(result, *entry)
		}

	}
	return result
}

func (self *valkeyStatsDb) AddPodStatsToDb(stats []structs.PodStats) error {
	for _, stat := range stats {
		controller := self.ownerCacheService.ControllerForPod(stat.Namespace, stat.PodName)
		if controller == nil {
			self.logger.Debug("No controller found for pod", "podName", stat.PodName, "namespace", stat.Namespace)
			continue
		}

		stat.CreatedAt = time.Now()
		err := self.valkey.StoreSortedListEntry(stat, time.Now().Truncate(time.Minute).Unix(), DB_STATS_POD_STATS_BUCKET_NAME, stat.Namespace, controller.Name)
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

func (self *valkeyStatsDb) AddNodeRamProcessMetricsToDb(nodeName string, data interface{}) error {
	self.Publish(data, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_MEMORY_NAME, DB_STATS_PROCESSES_NAME, nodeName)
	return self.valkey.SetObject(data, 0, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_MEMORY_NAME, DB_STATS_PROCESSES_NAME, nodeName)
}

func (self *valkeyStatsDb) AddNodeCpuMetricsToDb(nodeName string, data interface{}) error {
	self.Publish(data, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_CPU_NAME, nodeName)
	return self.valkey.SetObject(data, 0, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_CPU_NAME, nodeName)
}

func (self *valkeyStatsDb) AddNodeCpuProcessMetricsToDb(nodeName string, data interface{}) error {
	self.Publish(data, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_CPU_NAME, DB_STATS_PROCESSES_NAME, nodeName)
	return self.valkey.SetObject(data, 0, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_CPU_NAME, DB_STATS_PROCESSES_NAME, nodeName)
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
		// err := self.valkey.AddToBucket(DefaultMaxSize, stat, DB_STATS_NODE_STATS_BUCKET_NAME, stat.Name)
		err := self.valkey.StoreSortedListEntry(stat, time.Now().Truncate(time.Minute).Unix(), DB_STATS_NODE_STATS_BUCKET_NAME, stat.Name)
		if err != nil {
			return fmt.Errorf("error adding node stats: %s", err)
		}
	}
	return nil
}

func (self *valkeyStatsDb) Publish(data interface{}, keys ...string) {
	key := strings.Join(keys, ":")
	client := self.valkey.GetValkeyClient()
	err := client.Do(self.valkey.GetContext(), client.B().Publish().Channel(key).Message(utils.PrintJson(data)).Build()).Error()

	if err != nil {
		self.logger.Error("Error publishing to Redis", "error", err, "key", key)
	}
}

type GenericChartEntry struct {
	Time  time.Time          `json:"time"`
	Value float64            `json:"value"`          // this is the total value of all counted pods per time entry
	Pods  map[string]float64 `json:"pods,omitempty"` // this list is limited to 5 entries
}

func findSmallest(m map[string]float64) (string, float64, bool) {
	if len(m) == 0 {
		return "", 0, false
	}

	var smallestKey string
	var smallestValue float64
	first := true

	for key, value := range m {
		if first || value < smallestValue {
			smallestKey = key
			smallestValue = value
			first = false
		}
	}

	return smallestKey, smallestValue, true
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
