package core

import (
	"fmt"
	"log/slog"
	"maps"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/networkmonitor"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	DB_STATS_TRAFFIC_BUCKET_NAME            = "traffic-stats"
	DB_STATS_POD_STATS_BUCKET_NAME          = "pod-stats"
	DB_STATS_NODE_STATS_BUCKET_NAME         = "node-stats"
	DB_STATS_NODE_STATS_LATEST_BUCKET_NAME  = "node-stats-latest"
	DB_STATS_MACHINE_STATS_BUCKET_NAME      = "machine-stats"
	DB_STATS_SOCKET_STATS_BUCKET            = "socket-stats"
	DB_STATS_CNI_BUCKET_NAME                = "cluster-cni-configuration"
	DB_STATS_LIVE_BUCKET_NAME               = "live-stats"
	DB_STATS_TRAFFIC_NAME                   = "traffic"
	DB_STATS_CPU_NAME                       = "cpu"
	DB_STATS_MEMORY_NAME                    = "memory"
	DB_STATS_PROCESSES_NAME                 = "proc"
)

var DefaultMaxSizeSocketConnections int64 = 60
var liveStatsTTL = 10 * time.Minute // TTL for live-stats snapshot keys (refreshed on every write)

type ValkeyStatsDb interface {
	Run()
	AddInterfaceStatsToDb(stats []networkmonitor.PodNetworkStats)
	AddNodeStatsToDb(stats []structs.NodeStats) error
	AddMachineStatsToDb(nodeName string, stats structs.MachineStats) error
	AddPodStatsToDb(stats []structs.PodStats) error
	AddNodeRamMetricsToDb(nodeName string, data any) error
	AddNodeRamProcessMetricsToDb(nodeName string, data any) error
	AddNodeCpuMetricsToDb(nodeName string, data any) error
	AddNodeCpuProcessMetricsToDb(nodeName string, data any) error
	AddNodeTrafficMetricsToDb(nodeName string, data any) error
	AddSnoopyStatusToDb(nodeName string, data networkmonitor.SnoopyStatus) error
	GetCniData() ([]structs.CniData, error)
	GetLatestNodeStatsForNode(nodeName string) (*structs.NodeStats, error)
	GetMachineStatsForNode(nodeName string) (*structs.MachineStats, error)
	GetMachineStatsForNodes(nodeNames []string) []structs.MachineStats
	GetPodStatsEntriesForController(kind string, name string, namespace string, timeOffsetMinutes int64) *[]structs.PodStats
	GetTrafficStatsEntriesForController(kind string, name string, namespace string, timeOffsetMinutes int64) *[]networkmonitor.PodNetworkStats
	GetWorkspaceStatsCpuUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error)
	GetWorkspaceStatsMemoryUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error)
	GetWorkspaceStatsTrafficUtilization(timeOffsetInMinutes int, resources []unstructured.Unstructured) ([]GenericChartEntry, error)
	GetClusterDashboardStats(workspaceNames []string, getControllers func(string) ([]unstructured.Unstructured, error)) (ClusterDashboardStats, error)
	ReplaceCniData(data []structs.CniData)
	Publish(data any, keys ...string)
}

type valkeyStatsDb struct {
	config            cfg.ConfigModule
	logger            *slog.Logger
	valkey            valkeyclient.ValkeyClient
	ownerCacheService store.OwnerCacheService

	lastPodNetworkStats     []networkmonitor.PodNetworkStats
	lastPodNetworkStatsLock sync.RWMutex
}

func NewValkeyStatsModule(logger *slog.Logger, config cfg.ConfigModule, valkey valkeyclient.ValkeyClient, ownerCacheService store.OwnerCacheService) ValkeyStatsDb {
	dbStatsModule := valkeyStatsDb{
		config:              config,
		logger:              logger,
		valkey:              valkey,
		ownerCacheService:   ownerCacheService,
		lastPodNetworkStats: []networkmonitor.PodNetworkStats{},
	}

	return &dbStatsModule
}

func (self *valkeyStatsDb) Run() {
	// THIS CAN BE REMOVED IN THE NEXT PRODUCTION RELEASE
	// clean valkey on every startup
	err := store.DropAllResourcesFromValkey(self.valkey, self.logger)
	if err != nil {
		self.logger.Error("Error dropping all resources from valkey", "error", err)
	}
}

func (self *valkeyStatsDb) AddMachineStatsToDb(nodeName string, stats structs.MachineStats) error {
	return self.valkey.SetObject(stats, liveStatsTTL, DB_STATS_MACHINE_STATS_BUCKET_NAME, nodeName)
}

func (self *valkeyStatsDb) GetLatestNodeStatsForNode(nodeName string) (*structs.NodeStats, error) {
	return valkeyclient.GetObjectForKey[structs.NodeStats](
		self.valkey,
		DB_STATS_NODE_STATS_LATEST_BUCKET_NAME, nodeName,
	)
}

func (self *valkeyStatsDb) GetMachineStatsForNode(nodeName string) (*structs.MachineStats, error) {
	machineStats, err := valkeyclient.GetObjectForKey[structs.MachineStats](
		self.valkey,
		DB_STATS_MACHINE_STATS_BUCKET_NAME, nodeName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data as MachineStats: %s", err)
	}

	return machineStats, nil
}

func (self *valkeyStatsDb) AddInterfaceStatsToDb(currentStats []networkmonitor.PodNetworkStats) {
	self.lastPodNetworkStatsLock.Lock()
	lastStats := self.lastPodNetworkStats
	self.lastPodNetworkStats = currentStats
	self.lastPodNetworkStatsLock.Unlock()

	// Build a map for O(1) lookup instead of O(n) linear search
	lastStatsMap := make(map[string]networkmonitor.PodNetworkStats, len(lastStats))
	for _, e := range lastStats {
		lastStatsMap[e.Pod] = e
	}

	for _, currentStat := range currentStats {
		controller := self.ownerCacheService.ControllerForPod(currentStat.Namespace, currentStat.Pod)
		if controller == nil {
			// in case we cannot determine a controller
			controller = &utils.WorkloadSingleRequest{
				ResourceDescriptor: utils.PodResource,
				ResourceName:       currentStat.Pod,
				Namespace:          currentStat.Namespace,
			}
		}

		deltaStat := currentStat

		if lastEntry, ok := lastStatsMap[currentStat.Pod]; ok {
			// Guard against uint64 underflow: if the current counter is smaller than the
			// last known value, the snoopy process or pod was restarted and the BPF counter
			// reset to 0. In that case we skip this interval (delta = 0) rather than
			// producing a huge underflowed value that would appear as a traffic spike.
			if currentStat.ReceivedBytes >= lastEntry.ReceivedBytes {
				deltaStat.ReceivedBytes = currentStat.ReceivedBytes - lastEntry.ReceivedBytes
			} else {
				deltaStat.ReceivedBytes = 0
			}
			if currentStat.ReceivedPackets >= lastEntry.ReceivedPackets {
				deltaStat.ReceivedPackets = currentStat.ReceivedPackets - lastEntry.ReceivedPackets
			} else {
				deltaStat.ReceivedPackets = 0
			}
			if currentStat.TransmitBytes >= lastEntry.TransmitBytes {
				deltaStat.TransmitBytes = currentStat.TransmitBytes - lastEntry.TransmitBytes
			} else {
				deltaStat.TransmitBytes = 0
			}
			if currentStat.TransmitPackets >= lastEntry.TransmitPackets {
				deltaStat.TransmitPackets = currentStat.TransmitPackets - lastEntry.TransmitPackets
			} else {
				deltaStat.TransmitPackets = 0
			}
		}

		err := self.valkey.StoreSortedListEntry(
			deltaStat,
			time.Now().Truncate(time.Minute).Unix(),
			DB_STATS_TRAFFIC_BUCKET_NAME, currentStat.Namespace, controller.ResourceName,
		)
		if err != nil {
			self.logger.Error("Error adding interface stats", "namespace", currentStat.Namespace, "podName", currentStat.Pod, "error", err)
		}
	}
}

func (self *valkeyStatsDb) ReplaceCniData(data []structs.CniData) {
	for _, v := range data {
		err := self.valkey.SetObject(data, liveStatsTTL, DB_STATS_CNI_BUCKET_NAME, v.Node)
		if err != nil {
			self.logger.Error("Error adding cni data", "node", v.Node, "error", err)
		}
	}
}

func (self *valkeyStatsDb) GetCniData() ([]structs.CniData, error) {
	result, err := valkeyclient.GetObjectsByPrefix[structs.CniData](
		self.valkey,
		valkeyclient.ORDER_NONE,
		DB_STATS_CNI_BUCKET_NAME,
	)
	return result, err
}

func (self *valkeyStatsDb) GetPodStatsEntriesForController(kind string, name string, namespace string, timeOffsetMinutes int64) *[]structs.PodStats {
	result, err := valkeyclient.GetObjectsFromSortedListWithDuration[structs.PodStats](
		self.valkey,
		timeOffsetMinutes,
		DB_STATS_POD_STATS_BUCKET_NAME, namespace, name,
	)
	if err != nil {
		self.logger.Error("failed to GetPodStatsEntriesForController", "error", err)
	}
	return &result
}

func (self *valkeyStatsDb) GetTrafficStatsEntriesForController(kind string, name string, namespace string, timeOffsetMinutes int64) *[]networkmonitor.PodNetworkStats {
	result, err := valkeyclient.GetObjectsFromSortedListWithDuration[networkmonitor.PodNetworkStats](
		self.valkey,
		timeOffsetMinutes,
		DB_STATS_TRAFFIC_BUCKET_NAME, namespace, name,
	)
	if err != nil {
		self.logger.Error("failed to GetTrafficStatsEntriesForController", "error", err)
	}
	return &result
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
	var resultMutex sync.Mutex
	var wg sync.WaitGroup

	for _, controller := range resources {
		ns, name := controller.GetNamespace(), controller.GetName()

		wg.Go(func() {
			values, err := valkeyclient.GetObjectsFromSortedListWithDuration[structs.PodStats](
				self.valkey,
				int64(timeOffsetInMinutes),
				DB_STATS_POD_STATS_BUCKET_NAME, ns, name,
			)
			if err != nil {
				self.logger.Error(err.Error())
				return
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
			resultMutex.Lock()
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
			resultMutex.Unlock()
		})
	}
	wg.Wait()

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
	if _, exists := pods[podName]; exists {
		pods[podName] = compareValue
		return pods
	}

	if len(pods) < 5 {
		pods[podName] = compareValue
		return pods
	}

	smallestKey, smallestValue, _ := findSmallest(pods)
	if compareValue > smallestValue {
		delete(pods, smallestKey)
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
	var resultMutex sync.Mutex
	var wg sync.WaitGroup

	for _, controller := range resources {
		ns := controller.GetNamespace()
		name := controller.GetName()

		wg.Go(func() {
			values, err := valkeyclient.GetObjectsFromSortedListWithDuration[structs.PodStats](
				self.valkey,
				int64(timeOffsetInMinutes),
				DB_STATS_POD_STATS_BUCKET_NAME, ns, name,
			)
			if err != nil {
				self.logger.Error("failed to GetObjectsFromSortedListWithDuration", "error", err)
				return
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
			resultMutex.Lock()
			defer resultMutex.Unlock()
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
		})
	}
	wg.Wait()

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
	minOffset := 5
	maxOffset := 60 * 24 * 7
	if timeOffsetInMinutes < minOffset {
		timeOffsetInMinutes = minOffset
	}
	if timeOffsetInMinutes > maxOffset {
		timeOffsetInMinutes = maxOffset
	}

	// Aggregate all traffic by minute
	trafficByMinute := make(map[time.Time]GenericChartEntry)

	wg := sync.WaitGroup{}
	var resultMutex sync.Mutex
	for _, controller := range resources {
		wg.Go(func() {
			values, err := valkeyclient.GetObjectsFromSortedListWithDuration[networkmonitor.PodNetworkStats](
				self.valkey,
				int64(timeOffsetInMinutes),
				DB_STATS_TRAFFIC_BUCKET_NAME, controller.GetNamespace(), controller.GetName(),
			)
			if err != nil {
				self.logger.Error("failed to fetch objects from valkey", "error", err)
				return
			}

			// Add traffic for each minute
			for _, entry := range values {
				minute := entry.CreatedAt.Round(time.Minute)
				// Clamp any leftover underflowed values that were stored before the
			// delta-underflow fix in AddInterfaceStatsToDb. Underflowed uint64 values
			// are astronomically large; zeroing them is safer than the previous
			// "0xffffffffffffffff - value" approach which incorrectly restored the old
			// counter value as a traffic spike.
				if entry.ReceivedPackets > 184467440736991000 {
					entry.ReceivedPackets = 0
				}
				if entry.TransmitPackets > 184467440736991000 {
					entry.TransmitPackets = 0
				}
				if entry.ReceivedBytes > 184467440736991000 {
					entry.ReceivedBytes = 0
				}
				if entry.TransmitBytes > 184467440736991000 {
					entry.TransmitBytes = 0
				}
				if entry.ReceivedStartBytes > 184467440736991000 {
					entry.ReceivedStartBytes = 0
				}
				if entry.TransmitStartBytes > 184467440736991000 {
					entry.TransmitStartBytes = 0
				}

				resultMutex.Lock()
				{
					existingEntry, exists := trafficByMinute[minute]
					value := float64(entry.TransmitBytes + entry.ReceivedBytes)
					if !exists {
						trafficByMinute[minute] = GenericChartEntry{
							Time:  minute,
							Value: value,
							Pods: map[string]float64{
								entry.Pod: value,
							},
						}
					} else {
						trafficByMinute[minute] = GenericChartEntry{
							Time:  minute,
							Value: existingEntry.Value + value,
							Pods:  updateTop5Pods(existingEntry.Pods, value, entry.Pod),
						}
					}
				}
				resultMutex.Unlock()
			}
		})
	}
	wg.Wait()

	// sort and create array
	times := slices.Collect(maps.Keys(trafficByMinute))
	slices.SortFunc(times, time.Time.Compare)
	sortedEntries := make([]GenericChartEntry, 0, len(times))
	for i := 0; i < len(times); i++ {
		sortedEntries = append(sortedEntries, trafficByMinute[times[i]])
	}

	return sortedEntries, nil
}

func (self *valkeyStatsDb) AddPodStatsToDb(stats []structs.PodStats) error {
	for _, stat := range stats {
		controller := self.ownerCacheService.ControllerForPod(stat.Namespace, stat.PodName)
		if controller == nil {
			self.logger.Debug("No controller found for pod", "podName", stat.PodName, "namespace", stat.Namespace)
			continue
		}

		stat.CreatedAt = time.Now()
		err := self.valkey.StoreSortedListEntry(
			stat,
			time.Now().Truncate(time.Minute).Unix(),
			DB_STATS_POD_STATS_BUCKET_NAME, stat.Namespace, controller.ResourceName,
		)
		if err != nil {
			return fmt.Errorf("error adding pod stats: %s", err)
		}
	}
	return nil
}

func (self *valkeyStatsDb) AddNodeRamMetricsToDb(nodeName string, data any) error {
	self.Publish(data, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_MEMORY_NAME, nodeName)
	return self.valkey.SetObject(data, liveStatsTTL, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_MEMORY_NAME, nodeName)
}

func (self *valkeyStatsDb) AddNodeRamProcessMetricsToDb(nodeName string, data any) error {
	self.Publish(data, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_MEMORY_NAME, DB_STATS_PROCESSES_NAME, nodeName)
	return self.valkey.SetObject(data, liveStatsTTL, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_MEMORY_NAME, DB_STATS_PROCESSES_NAME, nodeName)
}

func (self *valkeyStatsDb) AddNodeCpuMetricsToDb(nodeName string, data any) error {
	self.Publish(data, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_CPU_NAME, nodeName)
	return self.valkey.SetObject(data, liveStatsTTL, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_CPU_NAME, nodeName)
}

func (self *valkeyStatsDb) AddNodeCpuProcessMetricsToDb(nodeName string, data any) error {
	self.Publish(data, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_CPU_NAME, DB_STATS_PROCESSES_NAME, nodeName)
	return self.valkey.SetObject(data, liveStatsTTL, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_CPU_NAME, DB_STATS_PROCESSES_NAME, nodeName)
}

func (self *valkeyStatsDb) AddNodeTrafficMetricsToDb(nodeName string, data any) error {
	self.Publish(data, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_TRAFFIC_NAME, nodeName)
	return self.valkey.SetObject(data, liveStatsTTL, DB_STATS_LIVE_BUCKET_NAME, DB_STATS_TRAFFIC_NAME, nodeName)
}

func (self *valkeyStatsDb) AddSnoopyStatusToDb(nodeName string, data networkmonitor.SnoopyStatus) error {
	return self.valkey.SetObject(data, liveStatsTTL, "status", nodeName, "snoopy")
}

func (self *valkeyStatsDb) AddNodeStatsToDb(stats []structs.NodeStats) error {
	for _, stat := range stats {
		stat.CreatedAt = time.Now().Format(time.RFC3339)
		// err := self.valkey.AddToBucket(DefaultMaxSize, stat, DB_STATS_NODE_STATS_BUCKET_NAME, stat.Name)
		err := self.valkey.StoreSortedListEntry(
			stat,
			time.Now().Truncate(time.Minute).Unix(),
			DB_STATS_NODE_STATS_BUCKET_NAME, stat.Name,
		)
		if err != nil {
			return fmt.Errorf("error adding node stats: %s", err)
		}
		// Also store as latest snapshot for O(1) current-value lookups (used by GetNodeStats)
		_ = self.valkey.SetObject(stat, liveStatsTTL, DB_STATS_NODE_STATS_LATEST_BUCKET_NAME, stat.Name)
	}
	return nil
}

func (self *valkeyStatsDb) Publish(data any, keys ...string) {
	key := strings.Join(keys, ":")
	client := self.valkey.GetValkeyClient()
	err := client.Do(
		self.valkey.GetContext(),
		client.B().Publish().Channel(key).Message(utils.PrintJson(data)).Build(),
	).Error()

	if err != nil {
		self.logger.Error("Error publishing to Redis", "error", err, "key", key)
	}
}

type GenericChartEntry struct {
	Time  time.Time          `json:"time"`
	Value float64            `json:"value"`          // this is the total value of all counted pods per time entry
	Pods  map[string]float64 `json:"pods,omitempty"` // this list is limited to 5 entries
}

type WorkspaceDashboardMetrics struct {
	Name          string  `json:"name"`
	CpuMillicores float64 `json:"cpuMillicores"`
	MemoryMb      float64 `json:"memoryMb"`
	PodCount      int     `json:"podCount"`
}

type ClusterDashboardStats struct {
	CpuHistory       []GenericChartEntry        `json:"cpuHistory"`
	WorkspaceMetrics []WorkspaceDashboardMetrics `json:"workspaceMetrics"`
}

const (
	dashboardStatsConcurrencyLimit = 20
	dashboardCpuHistoryPoints      = 12
	dashboardBytesPerMB            = 1 << 20
)

func (self *valkeyStatsDb) GetClusterDashboardStats(
	workspaceNames []string,
	getControllers func(string) ([]unstructured.Unstructured, error),
) (ClusterDashboardStats, error) {
	type wsResult struct {
		metrics    WorkspaceDashboardMetrics
		cpuEntries []GenericChartEntry
	}

	results := make([]wsResult, len(workspaceNames))
	var wg sync.WaitGroup
	sem := make(chan struct{}, dashboardStatsConcurrencyLimit)

	for i, name := range workspaceNames {
		wg.Add(1)
		sem <- struct{}{} // acquire before spawning to limit goroutine count
		go func(idx int, wsName string) {
			defer wg.Done()
			defer func() { <-sem }()

			controllers, err := getControllers(wsName)
			if err != nil {
				self.logger.Warn("dashboard-stats: failed to get controllers", "workspace", wsName, "error", err)
				results[idx] = wsResult{metrics: WorkspaceDashboardMetrics{Name: wsName}}
				return
			}

			type statsResult struct {
				entries []GenericChartEntry
				err     error
			}

			cpuCh := make(chan statsResult, 1)
			memCh := make(chan statsResult, 1)
			go func() {
				entries, err := self.GetWorkspaceStatsCpuUtilization(60, controllers)
				cpuCh <- statsResult{entries, err}
			}()
			go func() {
				entries, err := self.GetWorkspaceStatsMemoryUtilization(5, controllers)
				memCh <- statsResult{entries, err}
			}()
			cpu, mem := <-cpuCh, <-memCh

			if cpu.err != nil {
				self.logger.Warn("dashboard-stats: cpu stats failed", "workspace", wsName, "error", cpu.err)
			}
			if mem.err != nil {
				self.logger.Warn("dashboard-stats: memory stats failed", "workspace", wsName, "error", mem.err)
			}

			m := WorkspaceDashboardMetrics{Name: wsName}
			if len(cpu.entries) > 0 {
				latest := cpu.entries[len(cpu.entries)-1]
				m.CpuMillicores = latest.Value
				m.PodCount = len(latest.Pods)
			}
			if len(mem.entries) > 0 {
				m.MemoryMb = mem.entries[len(mem.entries)-1].Value / float64(dashboardBytesPerMB)
			}

			results[idx] = wsResult{metrics: m, cpuEntries: cpu.entries}
		}(i, name)
	}
	wg.Wait()

	// Merge all workspace CPU entries into cluster-wide history by timestamp
	clusterCpuMap := make(map[time.Time]float64)
	for _, r := range results {
		for _, entry := range r.cpuEntries {
			clusterCpuMap[entry.Time] += entry.Value
		}
	}

	// Sort timestamps ascending and downsample to target points
	timestamps := make([]time.Time, 0, len(clusterCpuMap))
	for t := range clusterCpuMap {
		timestamps = append(timestamps, t)
	}
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i].Before(timestamps[j])
	})

	cpuHistory := make([]GenericChartEntry, 0, dashboardCpuHistoryPoints)
	if len(timestamps) <= dashboardCpuHistoryPoints {
		for _, t := range timestamps {
			cpuHistory = append(cpuHistory, GenericChartEntry{Time: t, Value: clusterCpuMap[t]})
		}
	} else {
		// Pick dashboardCpuHistoryPoints-1 evenly spaced points, then always include the latest
		step := float64(len(timestamps)-1) / float64(dashboardCpuHistoryPoints-1)
		for i := 0; i < dashboardCpuHistoryPoints; i++ {
			idx := int(float64(i) * step)
			if idx >= len(timestamps) {
				idx = len(timestamps) - 1
			}
			t := timestamps[idx]
			cpuHistory = append(cpuHistory, GenericChartEntry{Time: t, Value: clusterCpuMap[t]})
		}
	}

	wsMetrics := make([]WorkspaceDashboardMetrics, len(results))
	for i, r := range results {
		wsMetrics[i] = r.metrics
	}

	return ClusterDashboardStats{
		CpuHistory:       cpuHistory,
		WorkspaceMetrics: wsMetrics,
	}, nil
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
	result := make([]structs.MachineStats, 0, len(nodes))

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
