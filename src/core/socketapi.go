package core

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/controllers"
	"mogenius-k8s-manager/src/crds/v1alpha1"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/helm"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/services"
	"mogenius-k8s-manager/src/shell"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"
	"mogenius-k8s-manager/src/websocket"
	"mogenius-k8s-manager/src/xterm"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	jsoniter "github.com/json-iterator/go"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

type SocketApi interface {
	Link(httpService HttpService, xtermService XtermService, apiService Api)
	Run()
	ExecuteCommandRequest(datagram structs.Datagram, httpApi HttpService) interface{}
	ParseDatagram(data []byte) (structs.Datagram, error)
	RegisterPatternHandler(
		pattern string,
		config PatternConfig,
		callback func(datagram structs.Datagram) (interface{}, error),
	)
	RegisterPatternHandlerRaw(
		pattern string,
		config PatternConfig,
		callback func(datagram structs.Datagram) interface{},
	)
}

type socketApi struct {
	logger *slog.Logger

	client  websocket.WebsocketClient
	config  config.ConfigModule
	dbstats kubernetes.BoltDbStats

	// the patternHandler should only be edited on startup
	patternHandlerLock sync.RWMutex
	patternHandler     map[string]PatternHandler
	httpService        HttpService
	xtermService       XtermService
	apiService         Api
}

type PatternHandler struct {
	Config   PatternConfig
	Callback func(datagram structs.Datagram) interface{}
}

type PatternConfig struct {
	NeedsUser bool
}

func NewSocketApi(
	logger *slog.Logger,
	configModule config.ConfigModule,
	client websocket.WebsocketClient,
	dbstatsModule kubernetes.BoltDbStats,
) SocketApi {
	self := &socketApi{}
	self.config = configModule
	self.client = client
	self.logger = logger
	self.dbstats = dbstatsModule
	self.patternHandler = map[string]PatternHandler{}

	self.registerPatterns()

	return self
}

func (self *socketApi) Link(httpService HttpService, xtermService XtermService, apiService Api) {
	assert.Assert(apiService != nil)
	assert.Assert(httpService != nil)
	assert.Assert(xtermService != nil)

	self.apiService = apiService
	self.httpService = httpService
	self.xtermService = xtermService
}

func (self *socketApi) Run() {
	assert.Assert(self.apiService != nil)
	assert.Assert(self.httpService != nil)
	assert.Assert(self.xtermService != nil)

	self.startK8sManager()
}

func (self *socketApi) RegisterPatternHandler(
	pattern string,
	config PatternConfig,
	callback func(datagram structs.Datagram) (interface{}, error),
) {
	self.patternHandlerLock.Lock()
	defer self.patternHandlerLock.Unlock()

	_, exists := self.patternHandler[pattern]
	assert.Assert(exists == false, "patterns may only be registered once", pattern)

	self.patternHandler[pattern] = PatternHandler{
		Config: config,
		Callback: func(datagram structs.Datagram) interface{} {
			result, err := callback(datagram)
			return NewMessageResponse(result, err)
		},
	}
}

func (self *socketApi) RegisterPatternHandlerRaw(
	pattern string,
	config PatternConfig,
	callback func(datagram structs.Datagram) interface{},
) {
	self.patternHandlerLock.Lock()
	defer self.patternHandlerLock.Unlock()

	_, exists := self.patternHandler[pattern]
	assert.Assert(exists == false, "patterns may only be registered once", pattern)

	self.patternHandler[pattern] = PatternHandler{
		Config:   config,
		Callback: callback,
	}
}

func (self *socketApi) registerPatterns() {
	self.RegisterPatternHandlerRaw(
		"K8sNotification",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			self.logger.Info("Received pattern", "pattern", datagram.Pattern)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		"ClusterStatus",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			return kubernetes.ClusterStatus()
		},
	)

	self.RegisterPatternHandlerRaw(
		"ClusterResourceInfo",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			nodeStats := kubernetes.GetNodeStats()
			loadBalancerExternalIps := kubernetes.GetClusterExternalIps()
			country, _ := utils.GuessClusterCountry()
			cniConfig, _ := self.dbstats.GetCniData()
			result := ClusterResourceInfoDto{
				NodeStats:               nodeStats,
				LoadBalancerExternalIps: loadBalancerExternalIps,
				Country:                 country,
				Provider:                string(utils.ClusterProviderCached),
				CniConfig:               cniConfig,
			}
			return result
		},
	)

	self.RegisterPatternHandlerRaw(
		"UpgradeK8sManager",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := K8sManagerUpgradeRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return UpgradeK8sManager(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"ClusterForceReconnect",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			time.Sleep(1 * time.Second)
			return kubernetes.ClusterForceReconnect()
		},
	)

	self.RegisterPatternHandlerRaw(
		"ClusterForceDisconnect",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			time.Sleep(1 * time.Second)
			return kubernetes.ClusterForceDisconnect()
		},
	)

	self.RegisterPatternHandlerRaw(
		"SYSTEM_CHECK",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			return services.SystemCheck()
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/restart",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			self.logger.Info("😵😵😵 Received RESTART COMMAND. Restarting now ...")
			time.Sleep(1 * time.Second)
			shutdown.SendShutdownSignal(false)
			select {}
		},
	)

	self.RegisterPatternHandlerRaw(
		"print-current-config",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			return self.config.AsEnvs()
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/energy-consumption",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			return services.EnergyConsumption()
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/sync-info",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			result, err := kubernetes.GetSyncRepoData()
			if err != nil {
				return err
			}
			return result
		},
	)

	self.RegisterPatternHandler(
		"install-traffic-collector",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.InstallTrafficCollector()
		},
	)

	self.RegisterPatternHandler(
		"install-pod-stats-collector",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.InstallPodStatsCollector()
		},
	)

	self.RegisterPatternHandler(
		"install-metrics-server",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.InstallMetricsServer()
		},
	)

	self.RegisterPatternHandler(
		"install-ingress-controller-traefik",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.InstallIngressControllerTreafik()
		},
	)

	self.RegisterPatternHandler(
		"install-cert-manager",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.InstallCertManager()
		},
	)

	self.RegisterPatternHandlerRaw(
		"install-cluster-issuer",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterIssuerInstallRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.AddSecretsToRedaction()
			result, err := services.InstallClusterIssuer(data.Email, 0)
			return NewMessageResponse(result, err)
		},
	)

	self.RegisterPatternHandler(
		"install-container-registry",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.InstallContainerRegistry()
		},
	)

	self.RegisterPatternHandler(
		"install-external-secrets",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.InstallExternalSecrets()
		},
	)

	self.RegisterPatternHandler(
		"install-metallb",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.InstallMetalLb()
		},
	)

	self.RegisterPatternHandler(
		"install-kepler",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.InstallKepler()
		},
	)

	self.RegisterPatternHandler(
		"uninstall-traffic-collector",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UninstallTrafficCollector()
		},
	)

	self.RegisterPatternHandler(
		"uninstall-pod-stats-collector",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UninstallPodStatsCollector()
		},
	)

	self.RegisterPatternHandler(
		"uninstall-metrics-server",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UninstallMetricsServer()
		},
	)

	self.RegisterPatternHandler(
		"uninstall-ingress-controller-traefik",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UninstallIngressControllerTreafik()
		},
	)

	self.RegisterPatternHandler(
		"uninstall-cert-manager",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UninstallCertManager()
		},
	)

	self.RegisterPatternHandler(
		"uninstall-cluster-issuer",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UninstallClusterIssuer()
		},
	)

	self.RegisterPatternHandler(
		"uninstall-container-registry",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UninstallContainerRegistry()
		},
	)

	self.RegisterPatternHandler(
		"uninstall-external-secrets",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UninstallExternalSecrets()
		},
	)

	self.RegisterPatternHandler(
		"uninstall-metallb",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UninstallMetalLb()
		},
	)

	self.RegisterPatternHandler(
		"uninstall-kepler",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UninstallKepler()
		},
	)

	self.RegisterPatternHandler(
		"upgrade-traffic-collector",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UpgradeTrafficCollector()
		},
	)

	self.RegisterPatternHandler(
		"upgrade-pod-stats-collector",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UpgradePodStatsCollector()
		},
	)

	self.RegisterPatternHandler(
		"upgrade-metrics-server",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UpgradeMetricsServer()
		},
	)

	self.RegisterPatternHandler(
		"upgrade-ingress-controller-traefik",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UpgradeIngressControllerTreafik()
		},
	)

	self.RegisterPatternHandler(
		"upgrade-cert-manager",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UpgradeCertManager()
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_UPGRADE_CONTAINER_REGISTRY,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UpgradeContainerRegistry()
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_UPGRADE_METALLB,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UpgradeMetalLb()
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_UPGRADE_KEPLER,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UpgradeKepler()
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STATS_PODSTAT_FOR_POD_ALL,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.StatsDataRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			ctrl := kubernetes.ControllerForPod(data.Namespace, data.PodName)
			if ctrl == nil {
				return fmt.Errorf("could not find controller for pod %s in namespace %s", data.PodName, data.Namespace)
			}
			return self.dbstats.GetPodStatsEntriesForController(*ctrl)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STATS_PODSTAT_FOR_POD_LAST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.StatsDataRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			ctrl := kubernetes.ControllerForPod(data.Namespace, data.PodName)
			if ctrl == nil {
				return fmt.Errorf("could not find controller for pod %s in namespace %s", data.PodName, data.Namespace)
			}
			return self.dbstats.GetLastPodStatsEntryForController(*ctrl)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STATS_PODSTAT_FOR_CONTROLLER_ALL,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := kubernetes.K8sController{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetPodStatsEntriesForController(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STATS_PODSTAT_FOR_CONTROLLER_LAST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := kubernetes.K8sController{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetLastPodStatsEntryForController(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STATS_TRAFFIC_FOR_CONTROLLER_ALL,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := kubernetes.K8sController{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetTrafficStatsEntriesForController(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STATS_TRAFFIC_FOR_CONTROLLER_SUM,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := kubernetes.K8sController{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetTrafficStatsEntrySumForController(data, false)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STATS_TRAFFIC_FOR_CONTROLLER_LAST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := kubernetes.K8sController{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetTrafficStatsEntrySumForController(data, false)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STATS_TRAFFIC_FOR_CONTROLLER_SOCKET_CONNECTIONS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := kubernetes.K8sController{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetSocketConnectionsForController(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STATS_TRAFFIC_FOR_POD_ALL,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.StatsDataRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			ctrl := kubernetes.ControllerForPod(data.Namespace, data.PodName)
			if ctrl == nil {
				return fmt.Errorf("could not find controller for pod %s in namespace %s", data.PodName, data.Namespace)
			}
			return self.dbstats.GetTrafficStatsEntriesForController(*ctrl)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STATS_TRAFFIC_FOR_POD_SUM,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.StatsDataRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			ctrl := kubernetes.ControllerForPod(data.Namespace, data.PodName)
			if ctrl == nil {
				return fmt.Errorf("could not find controller for pod %s in namespace %s", data.PodName, data.Namespace)
			}
			return self.dbstats.GetTrafficStatsEntrySumForController(*ctrl, false)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STATS_TRAFFIC_FOR_POD_LAST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.StatsDataRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			ctrl := kubernetes.ControllerForPod(data.Namespace, data.PodName)
			if ctrl == nil {
				return fmt.Errorf("could not find controller for pod %s in namespace %s", data.PodName, data.Namespace)
			}
			return self.dbstats.GetTrafficStatsEntrySumForController(*ctrl, false)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STATS_PODSTAT_FOR_NAMESPACE_ALL,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NsStatsDataRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetPodStatsEntriesForNamespace(data.Namespace)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STATS_PODSTAT_FOR_NAMESPACE_LAST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NsStatsDataRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetLastPodStatsEntriesForNamespace(data.Namespace)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STATS_TRAFFIC_FOR_NAMESPACE_ALL,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NsStatsDataRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetTrafficStatsEntriesForNamespace(data.Namespace)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STATS_TRAFFIC_FOR_NAMESPACE_SUM,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NsStatsDataRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetTrafficStatsEntriesSumForNamespace(data.Namespace)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STATS_TRAFFIC_FOR_NAMESPACE_LAST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NsStatsDataRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetTrafficStatsEntriesSumForNamespace(data.Namespace)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_STATS_WORKLOAD_CPU_UTILIZATION,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WorkspaceStatsRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return nil, err
			}
			return self.dbstats.GetWorkspaceStatsCpuUtilization(data.WorkspaceName)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_STATS_WORKLOAD_MEMORY_UTILIZATION,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WorkspaceStatsRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return nil, err
			}
			return self.dbstats.GetWorkspaceStatsMemoryUtilization(data.WorkspaceName)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_STATS_WORKLOAD_TRAFFIC_UTILIZATION,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WorkspaceStatsRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return nil, err
			}
			return self.dbstats.GetWorkspaceStatsTrafficUtilization(data.WorkspaceName)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_METRICS_DEPLOYMENT_AVG_UTILIZATION,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := kubernetes.K8sController{}
			data.Kind = "Deployment"
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.GetAverageUtilizationForDeployment(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_FILES_LIST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.FilesListRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.List(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_FILES_CREATE_FOLDER,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.FilesCreateFolderRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.CreateFolder(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_FILES_RENAME,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.FilesRenameRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.Rename(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_FILES_CHOWN,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.FilesChownRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.Chown(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_FILES_CHMOD,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.FilesChmodRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.Chmod(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_FILES_DELETE,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.FilesDeleteRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.Delete(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_FILES_DOWNLOAD,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.FilesDownloadRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.Download(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_FILES_INFO,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := dtos.PersistentFileRequestDto{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.Info(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_EXECUTE_HELM_CHART_TASK,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterHelmRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.InstallHelmChart(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_UNINSTALL_HELM_CHART,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterHelmUninstallRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.DeleteHelmChart(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_TCP_UDP_CONFIGURATION,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			return services.TcpUdpClusterConfiguration()
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_BACKUP,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			result, err := kubernetes.BackupNamespace("")
			if err != nil {
				return err.Error()
			}
			return result
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_READ_CONFIGMAP,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterGetConfigMap{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.GetConfigMapWR(data.Namespace, data.Name)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_WRITE_CONFIGMAP,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterWriteConfigMap{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.WriteConfigMap(data.Namespace, data.Name, data.Data, data.Labels)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_LIST_CONFIGMAPS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterListWorkloads{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.ListConfigMapWithFieldSelector(data.Namespace, data.LabelSelector, data.Prefix)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_READ_DEPLOYMENT,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterGetDeployment{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.GetDeploymentResult(data.Namespace, data.Name)
		},
	)

	// // TODO
	// // case structs.PAT_CLUSTER_WRITE_DEPLOYMENT:
	// // 	data := ClusterWriteDeployment{}
	// // 	structs.MarshalUnmarshal(&datagram, &data)
	// // 	if err := utils.ValidateJSON(data); err != nil {
	// // 		return err
	// // 	}
	// // 	return kubernetes.WriteConfigMap(data.Namespace, data.Name, data.Data, data.Labels)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_LIST_DEPLOYMENTS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterListWorkloads{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.ListDeploymentsWithFieldSelector(data.Namespace, data.LabelSelector, data.Prefix)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_READ_PERSISTENT_VOLUME_CLAIM,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterGetPersistentVolume{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(kubernetes.GetPersistentVolumeClaim(data.Namespace, data.Name))
		},
	)

	// // TODO
	// // case structs.PAT_CLUSTER_WRITE_PERSISTENT_VOLUME_CLAIM:
	// // 	data := ClusterWritePersistentVolume{}
	// // 	structs.MarshalUnmarshal(&datagram, &data)
	// // 	if err := utils.ValidateJSON(data); err != nil {
	// // 		return err
	// // 	}
	// // 	return kubernetes.WritePersistentVolume(data.Namespace, data.Name, data.Data, data.Labels)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterListWorkloads{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			// AllPersistentVolumes
			return kubernetes.ListPersistentVolumeClaimsWithFieldSelector(data.Namespace, data.LabelSelector, data.Prefix)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_UPDATE_LOCAL_TLS_SECRET,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterUpdateLocalTlsSecret{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.CreateMogeniusContainerRegistryTlsSecret(data.LocalTlsCrt, data.LocalTlsKey)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_NAMESPACE_CREATE,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceCreateRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Project.AddSecretsToRedaction()
			return services.CreateNamespace(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_NAMESPACE_DELETE,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceDeleteRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.DeleteNamespace(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_NAMESPACE_SHUTDOWN,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceShutdownRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			return services.ShutdownNamespace(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_NAMESPACE_POD_IDS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespacePodIdsRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.PodIds(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_NAMESPACE_VALIDATE_CLUSTER_PODS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceValidateClusterPodsRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.ValidateClusterPods(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_NAMESPACE_VALIDATE_PORTS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceValidatePortsRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.ValidateClusterPorts(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_NAMESPACE_LIST_ALL,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			return services.ListAllNamespaces()
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_NAMESPACE_GATHER_ALL_RESOURCES,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceGatherAllResourcesRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.ListAllResourcesForNamespace(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_NAMESPACE_BACKUP,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceBackupRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			result, err := kubernetes.BackupNamespace(data.NamespaceName)
			if err != nil {
				return err.Error()
			}
			return result
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_NAMESPACE_RESTORE,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceRestoreRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			result, err := kubernetes.RestoreNamespace(data.YamlData, data.NamespaceName)
			if err != nil {
				return err.Error()
			}
			return result
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_NAMESPACE_RESOURCE_YAML,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceResourceYamlRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			result, err := kubernetes.AllResourcesFromToCombinedYaml(data.NamespaceName, data.Resources)
			if err != nil {
				return err.Error()
			}
			return result
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_HELM_REPO_ADD,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmRepoAddRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmRepoAdd(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_HELM_REPO_PATCH,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmRepoPatchRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmRepoPatch(data))
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_CLUSTER_HELM_REPO_UPDATE,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return helm.HelmRepoUpdate()
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_CLUSTER_HELM_REPO_LIST,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return helm.HelmRepoList()
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_HELM_REPO_REMOVE,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmRepoRemoveRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmRepoRemove(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_HELM_CHART_SEARCH,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmChartSearchRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmChartSearch(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_HELM_CHART_INSTALL,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmChartInstallUpgradeRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmChartInstall(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_HELM_CHART_SHOW,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmChartShowRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmChartShow(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_HELM_CHART_VERSIONS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmChartVersionRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmChartVersion(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_HELM_RELEASE_UPGRADE,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmChartInstallUpgradeRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmReleaseUpgrade(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_HELM_RELEASE_UNINSTALL,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmReleaseUninstallRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmReleaseUninstall(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_HELM_RELEASE_LIST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmReleaseListRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmReleaseList(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_HELM_RELEASE_STATUS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmReleaseStatusRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmReleaseStatus(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_HELM_RELEASE_HISTORY,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmReleaseHistoryRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmReleaseHistory(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_HELM_RELEASE_ROLLBACK,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmReleaseRollbackRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmReleaseRollback(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_HELM_RELEASE_GET,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmReleaseGetRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmReleaseGet(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_HELM_RELEASE_GET_WORKLOADS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmReleaseGetWorkloadsRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmReleaseGetWorkloads(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_CREATE,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceUpdateRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			data.Project.AddSecretsToRedaction()
			return services.UpdateService(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_DELETE,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceDeleteRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			data.Project.AddSecretsToRedaction()
			return services.DeleteService(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_POD_IDS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceGetPodIdsRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.ServicePodIds(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_POD_EXISTS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServicePodExistsRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.ServicePodExists(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_PODS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServicePodsRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.ServicePodStatus(data)
		},
	)

	// // case structs.PAT_SERVICE_SET_IMAGE:
	// // 	data := ServiceSetImageRequest{}
	// // 	structs.MarshalUnmarshal(&datagram, &data)
	// // 	if err := utils.ValidateJSON(data); err != nil {
	// // 		return err
	// // 	}
	// // 	return SetImage(data)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_LOG,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceGetLogRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.PodLog(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_LOG_ERROR,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceGetLogRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.PodLogError(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_RESOURCE_STATUS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceResourceStatusRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.PodStatus(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_RESTART,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceRestartRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			return services.Restart(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_STOP,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceStopRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			return services.StopService(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_START,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceStartRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			return services.StartService(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_UPDATE_SERVICE,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceUpdateRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Project.AddSecretsToRedaction()
			data.Service.AddSecretsToRedaction()
			return services.UpdateService(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_TRIGGER_JOB,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceTriggerJobRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.TriggerJobService(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_STATUS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceStatusRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.StatusServiceDebounced(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_LOG_STREAM,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceLogStreamRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.logStream(data, datagram)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_EXEC_SH_CONNECTION_REQUEST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.PodCmdConnectionRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go self.execShConnection(data)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_LOG_STREAM_CONNECTION_REQUEST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.PodCmdConnectionRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go self.logStreamConnection(data)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_BUILD_LOG_STREAM_CONNECTION_REQUEST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.BuildLogConnectionRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go buildLogStreamConnection(data)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.ComponentLogConnectionRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go componentLogStreamConnection(data)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.PodEventConnectionRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go podEventStreamConnection(data)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_SCAN_IMAGE_LOG_STREAM_CONNECTION_REQUEST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.ScanImageLogConnectionRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.AddSecretsToRedaction()
			go scanImageLogStreamConnection(data)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_SERVICE_CLUSTER_TOOL_STREAM_CONNECTION_REQUEST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.ClusterToolConnectionRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go services.XTermClusterToolStreamConnection(data)
			return nil
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_LIST_ALL_WORKLOADS,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return kubernetes.GetAvailableResources()
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_GET_WORKLOAD_LIST,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.SyncResourceEntry{}
			structs.MarshalUnmarshal(&datagram, &data)
			return kubernetes.GetUnstructuredResourceListFromStore(data.Group, data.Kind, data.Version, data.Name, data.Namespace)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_GET_NAMESPACE_WORKLOAD_LIST,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := kubernetes.GetUnstructuredNamespaceResourceListRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			return kubernetes.GetUnstructuredNamespaceResourceList(data.Namespace, data.Whitelist, data.Blacklist)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_GET_LABELED_WORKLOAD_LIST,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := kubernetes.GetUnstructuredLabeledResourceListRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			return kubernetes.GetUnstructuredLabeledResourceList(data.Label, data.Whitelist, data.Blacklist)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_DESCRIBE_WORKLOAD,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.SyncResourceItem{}
			structs.MarshalUnmarshal(&datagram, &data)
			return kubernetes.DescribeUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.ResourceName)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_CREATE_NEW_WORKLOAD,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.SyncResourceData{}
			structs.MarshalUnmarshal(&datagram, &data)
			return kubernetes.CreateUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.YamlData)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_GET_WORKLOAD,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.SyncResourceItem{}
			structs.MarshalUnmarshal(&datagram, &data)
			return kubernetes.GetUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.ResourceName)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_GET_WORKLOAD_STATUS,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := kubernetes.GetWorkloadStatusRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			return kubernetes.GetWorkloadStatus(data)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_GET_WORKLOAD_EXAMPLE,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.SyncResourceItem{}
			structs.MarshalUnmarshal(&datagram, &data)
			return kubernetes.GetResourceTemplateYaml(data.Group, data.Version, data.Name, data.Kind, data.Namespace, data.ResourceName), nil
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_UPDATE_WORKLOAD,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.SyncResourceData{}
			structs.MarshalUnmarshal(&datagram, &data)
			return kubernetes.UpdateUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.YamlData)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_DELETE_WORKLOAD,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.SyncResourceItem{}
			structs.MarshalUnmarshal(&datagram, &data)
			return nil, kubernetes.DeleteUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.ResourceName)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_GET_WORKSPACES,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return self.apiService.GetAllWorkspaces()
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_CREATE_WORKSPACE,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestCreateWorkspace{}
			structs.MarshalUnmarshal(&datagram, &data)
			return self.apiService.CreateWorkspace(data.Name, v1alpha1.NewWorkspaceSpec(
				data.DisplayName,
				data.Resources,
			))
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_GET_WORKSPACE,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestGetWorkspace{}
			structs.MarshalUnmarshal(&datagram, &data)
			return self.apiService.GetWorkspace(data.Name)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_UPDATE_WORKSPACE,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestUpdateWorkspace{}
			structs.MarshalUnmarshal(&datagram, &data)
			// TODO: use this method everywhere in place of `structs.MarshalUnmarshal`
			// err := self.loadRequest(&datagram, &data)
			// if err != nil {
			// 	return nil, err
			// }
			return self.apiService.UpdateWorkspace(data.Name, v1alpha1.NewWorkspaceSpec(
				data.DisplayName,
				data.Resources,
			))
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_DELETE_WORKSPACE,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestDeleteWorkspace{}
			structs.MarshalUnmarshal(&datagram, &data)
			return self.apiService.DeleteWorkspace(data.Name)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_GET_USERS,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return self.apiService.GetAllUsers()
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_CREATE_USER,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestCreateUser{}
			structs.MarshalUnmarshal(&datagram, &data)
			return self.apiService.CreateUser(data.Name, v1alpha1.NewUserSpec(data.MogeniusId))
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_GET_USER,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestGetUser{}
			structs.MarshalUnmarshal(&datagram, &data)
			return self.apiService.GetUser(data.Name)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_UPDATE_USER,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestUpdateUser{}
			structs.MarshalUnmarshal(&datagram, &data)
			return self.apiService.UpdateUser(data.Name, v1alpha1.NewUserSpec(data.MogeniusId))
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_DELETE_USER,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestDeleteUser{}
			structs.MarshalUnmarshal(&datagram, &data)
			return self.apiService.DeleteUser(data.Name)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_GET_TEAMS,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return self.apiService.GetAllTeams()
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_CREATE_TEAM,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestCreateTeam{}
			structs.MarshalUnmarshal(&datagram, &data)
			return self.apiService.CreateTeam(data.Name, v1alpha1.NewTeamSpec(data.DisplayName, data.Users))
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_GET_TEAM,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestGetTeam{}
			structs.MarshalUnmarshal(&datagram, &data)
			return self.apiService.GetTeam(data.Name)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_UPDATE_TEAM,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestUpdateTeam{}
			structs.MarshalUnmarshal(&datagram, &data)
			return self.apiService.UpdateTeam(data.Name, v1alpha1.NewTeamSpec(data.DisplayName, data.Users))
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_DELETE_TEAM,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestDeleteTeam{}
			structs.MarshalUnmarshal(&datagram, &data)
			return self.apiService.DeleteTeam(data.Name)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_GET_GRANTS,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return self.apiService.GetAllGrants()
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_CREATE_GRANT,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestCreateGrant{}
			structs.MarshalUnmarshal(&datagram, &data)
			return self.apiService.CreateGrant(data.Name, v1alpha1.NewGrantSpec(
				data.Grantee,
				data.TargetType,
				data.TargetName,
				data.Role,
			))
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_GET_GRANT,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestGetGrant{}
			structs.MarshalUnmarshal(&datagram, &data)
			return self.apiService.GetGrant(data.Name)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_UPDATE_GRANT,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestUpdateGrant{}
			structs.MarshalUnmarshal(&datagram, &data)
			return self.apiService.UpdateGrant(data.Name, v1alpha1.NewGrantSpec(
				data.Grantee,
				data.TargetType,
				data.TargetName,
				data.Role,
			))
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_DELETE_GRANT,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestDeleteGrant{}
			structs.MarshalUnmarshal(&datagram, &data)
			return self.apiService.DeleteGrant(data.Name)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_GET_WORKSPACE_WORKLOADS,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WorkspaceWorkloadRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			return self.apiService.GetWorkspaceResources(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_BUILDER_STATUS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			return kubernetes.GetDb().GetBuilderStatus()
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_BUILD_INFOS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.BuildJobStatusRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.GetDb().GetBuildJobInfosFromDb(data.BuildId)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_BUILD_LAST_INFOS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.BuildTaskRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.GetDb().GetLastBuildJobInfosFromDb(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_BUILD_LIST_ALL,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			return services.ListAll()
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_BUILD_LIST_BY_PROJECT,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.ListBuildByProjectIdRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.ListByProjectId(data.ProjectId)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_BUILD_ADD,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.BuildJob{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Project.AddSecretsToRedaction()
			data.Service.AddSecretsToRedaction()
			return services.AddBuildJob(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_BUILD_CANCEL,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.BuildJob{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Project.AddSecretsToRedaction()
			data.Service.AddSecretsToRedaction()
			return services.Cancel(data.BuildId)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_BUILD_DELETE,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.BuildJobStatusRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.DeleteBuild(data.BuildId)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_BUILD_LAST_JOB_OF_SERVICES,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.BuildTaskListOfServicesRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.LastBuildInfosOfServices(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_BUILD_JOB_LIST_OF_SERVICE,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.BuildTaskRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.GetDb().GetBuildJobInfosListFromDb(data.Namespace, data.Controller, data.Container)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_BUILD_DELETE_ALL_OF_SERVICE,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.BuildTaskRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			kubernetes.GetDb().DeleteAllBuildData(data.Namespace, data.Controller, data.Container)
			return nil
		},
	)

	// //case structs.PAT_BUILD_LAST_JOB_INFO_OF_SERVICE:
	// //	data := structs.BuildServiceRequest{}
	// //	structs.MarshalUnmarshal(&datagram, &data)
	// //	if err := utils.ValidateJSON(data); err != nil {
	// //		return err
	// //	}
	// //	return LastBuildForService(data.ServiceId)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STORAGE_CREATE_VOLUME,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NfsVolumeRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.CreateMogeniusNfsVolume(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STORAGE_DELETE_VOLUME,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NfsVolumeRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.DeleteMogeniusNfsVolume(data)
		},
	)

	// // case structs.PAT_STORAGE_BACKUP_VOLUME:
	// // 	data := NfsVolumeBackupRequest{}
	// // 	structs.MarshalUnmarshal(&datagram, &data)
	// // 	if err := utils.ValidateJSON(data); err != nil {
	// // 		return err
	// // 	}
	// // 	data.AddSecretsToRedaction()
	// // 	return BackupMogeniusNfsVolume(data)
	// // case structs.PAT_STORAGE_RESTORE_VOLUME:
	// // 	data := NfsVolumeRestoreRequest{}
	// // 	structs.MarshalUnmarshal(&datagram, &data)
	// // 	if err := utils.ValidateJSON(data); err != nil {
	// // 		return err
	// // 	}
	// // 	data.AddSecretsToRedaction()
	// // 	return RestoreMogeniusNfsVolume(data)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STORAGE_STATS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NfsVolumeStatsRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.StatsMogeniusNfsVolume(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STORAGE_NAMESPACE_STATS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NfsNamespaceStatsRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.StatsMogeniusNfsNamespace(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_STORAGE_STATUS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NfsStatusRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.StatusMogeniusNfs(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_LOG_LIST_ALL,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			return kubernetes.GetDb().ListLogFromDb()
		},
	)

	// // - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// // External Secrets
	// // - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -

	self.RegisterPatternHandlerRaw(
		structs.PAT_EXTERNAL_SECRET_STORE_CREATE,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.CreateSecretsStoreRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return controllers.CreateExternalSecretStore(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_EXTERNAL_SECRET_STORE_LIST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.ListSecretStoresRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return controllers.ListExternalSecretsStores(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_EXTERNAL_SECRET_LIST_AVAILABLE_SECRETS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.ListSecretsRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return controllers.ListAvailableExternalSecrets(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_EXTERNAL_SECRET_STORE_DELETE,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.DeleteSecretsStoreRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return controllers.DeleteExternalSecretsStore(data)
		},
	)

	// // - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// // Labeled Network Policies
	// // - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	self.RegisterPatternHandlerRaw(
		structs.PAT_ATTACH_LABELED_NETWORK_POLICY,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.AttachLabeledNetworkPolicyRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(controllers.AttachLabeledNetworkPolicy(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_DETACH_LABELED_NETWORK_POLICY,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.DetachLabeledNetworkPolicyRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(controllers.DetachLabeledNetworkPolicy(data))
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_LIST_LABELED_NETWORK_POLICY_PORTS,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return controllers.ListLabeledNetworkPolicyPorts()
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_LIST_CONFLICTING_NETWORK_POLICIES,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.ListConflictingNetworkPoliciesRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(controllers.ListAllConflictingNetworkPolicies(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_REMOVE_CONFLICTING_NETWORK_POLICIES,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.RemoveConflictingNetworkPoliciesRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(controllers.RemoveConflictingNetworkPolicies(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_LIST_CONTROLLER_NETWORK_POLICIES,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.ListControllerLabeledNetworkPoliciesRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(controllers.ListControllerLabeledNetwork(data))
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_UPDATE_NETWORK_POLICIES_TEMPLATE,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := []kubernetes.NetworkPolicy{}
			structs.MarshalUnmarshal(&datagram, &data)
			return nil, controllers.UpdateNetworkPolicyTemplate(data)
		},
	)

	self.RegisterPatternHandler(
		structs.PAT_LIST_ALL_NETWORK_POLICIES,
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return controllers.ListAllNetworkPolicies()
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_LIST_NAMESPACE_NETWORK_POLICIES,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.ListNamespaceLabeledNetworkPoliciesRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(controllers.ListNamespaceNetworkPolicies(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_ENFORCE_NETWORK_POLICY_MANAGER,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.EnforceNetworkPolicyManagerRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(nil, controllers.EnforceNetworkPolicyManager(data.NamespaceName))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_DISABLE_NETWORK_POLICY_MANAGER,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.DisableNetworkPolicyManagerRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(nil, controllers.DisableNetworkPolicyManager(data.NamespaceName))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_REMOVE_UNMANAGED_NETWORK_POLICIES,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.RemoveUnmanagedNetworkPoliciesRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(nil, controllers.RemoveUnmanagedNetworkPolicies(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_LIST_ONLY_NAMESPACE_NETWORK_POLICIES,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.ListNamespaceLabeledNetworkPoliciesRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(controllers.ListManagedAndUnmanagedNamespaceNetworkPolicies(data))
		},
	)

	// // - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// // Cronjobs
	// // - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	self.RegisterPatternHandlerRaw(
		structs.PAT_LIST_CRONJOB_JOBS,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := ListCronjobJobsRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.ListCronjobJobs(data.ControllerName, data.NamespaceName, data.ProjectId)
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_LIVE_STREAM_NODES_TRAFFIC_REQUEST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.WsConnectionRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go self.xtermService.LiveStreamConnection(data, datagram, self.httpService)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_LIVE_STREAM_NODES_MEMORY_REQUEST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.WsConnectionRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go self.xtermService.LiveStreamConnection(data, datagram, self.httpService)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		structs.PAT_LIVE_STREAM_NODES_CPU_REQUEST,
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.WsConnectionRequest{}
			structs.MarshalUnmarshal(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go self.xtermService.LiveStreamConnection(data, datagram, self.httpService)
			return nil
		},
	)

	// return NewMessageResponse(nil, fmt.Errorf("Pattern not found"))
}

func (self *socketApi) loadRequest(datagram *structs.Datagram, data interface{}) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary

	bytes, err := json.Marshal(datagram.Payload)
	if err != nil {
		return err
	}

	err = json.Unmarshal(bytes, data)
	if err != nil {
		return err
	}

	return nil
}

func (self *socketApi) startK8sManager() {
	self.updateCheck()
	self.versionTicker()

	go func() {
		for status := range structs.EventConnectionStatus {
			if status {
				// CONNECTED
				for {
					_, _, err := structs.EventQueueConnection.ReadMessage()
					if err != nil {
						self.logger.Error("failed to read message for event queue", "eventConnectionUrl", structs.EventConnectionUrl, "error", err)
						break
					}
				}
				structs.EventQueueConnection.Close()
			}
		}
	}()

	self.startMessageHandler()
}

func (self *socketApi) startMessageHandler() {
	var preparedFileName *string
	var preparedFileRequest *services.FilesUploadRequest
	var openFile *os.File

	maxGoroutines := 100
	semaphoreChan := make(chan struct{}, maxGoroutines)
	var wg sync.WaitGroup

	for !self.client.IsTerminated() {
		_, message, err := self.client.ReadMessage()
		if err != nil {
			self.logger.Error("failed to read message from websocket connection", "error", err)
			time.Sleep(time.Second) // wait before next attempt to read
			continue
		}
		rawDataStr := string(message)
		if rawDataStr == "" {
			continue
		}
		if strings.HasPrefix(rawDataStr, "######START_UPLOAD######;") {
			preparedFileName = utils.Pointer(fmt.Sprintf("%s.zip", utils.NanoId()))
			openFile, err = os.OpenFile(*preparedFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				self.logger.Error("Cannot open uploadfile", "filename", *preparedFileName, "error", err)
			}
			continue
		}
		if strings.HasPrefix(rawDataStr, "######END_UPLOAD######;") {
			openFile.Close()
			if preparedFileName != nil && preparedFileRequest != nil {
				services.Uploaded(*preparedFileName, *preparedFileRequest)
			}
			os.Remove(*preparedFileName)

			var ack = structs.CreateDatagramAck("ack:files/upload:end", preparedFileRequest.Id)
			self.JobServerSendData(self.client, ack)

			preparedFileName = nil
			preparedFileRequest = nil
			continue
		}

		if preparedFileName != nil {
			_, err := openFile.Write([]byte(rawDataStr))
			if err != nil {
				self.logger.Error("Error writing to file", "error", err)
			}
			continue
		}

		datagram, err := self.ParseDatagram([]byte(rawDataStr))
		if err != nil {
			self.logger.Error("failed to parse datagram", "error", err)
			continue
		}

		datagram.DisplayReceiveSummary()

		if isSuppressed := utils.Contains(structs.SUPPRESSED_OUTPUT_PATTERN, datagram.Pattern); !isSuppressed {
			moDebug, err := strconv.ParseBool(self.config.Get("MO_DEBUG"))
			assert.Assert(err == nil, err)
			if moDebug {
				self.logger.Info("received datagram", "datagram", datagram)
			}
		}

		if slices.Contains(structs.COMMAND_REQUESTS, datagram.Pattern) {
			// ####### COMMAND
			semaphoreChan <- struct{}{}

			wg.Add(1)
			go func() {
				defer wg.Done()
				responsePayload := self.ExecuteCommandRequest(datagram, self.httpService)
				result := structs.Datagram{
					Id:        datagram.Id,
					Pattern:   datagram.Pattern,
					Payload:   responsePayload,
					CreatedAt: datagram.CreatedAt,
				}
				self.JobServerSendData(self.client, result)
				<-semaphoreChan
			}()
		} else if slices.Contains(structs.BINARY_REQUEST_UPLOAD, datagram.Pattern) {
			preparedFileRequest = ExecuteBinaryRequestUpload(datagram)

			var ack = structs.CreateDatagramAck("ack:files/upload:datagram", datagram.Id)
			self.JobServerSendData(self.client, ack)
		} else {
			self.logger.Error("Pattern not found", "pattern", datagram.Pattern)
		}
	}
	self.logger.Debug("api messagehandler finished as the websocket client was terminated")
}

func (self *socketApi) ParseDatagram(data []byte) (structs.Datagram, error) {
	datagram := structs.CreateEmptyDatagram()

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, &datagram)
	if err != nil {
		self.logger.Error("failed to unmarshal", "error", err)
		return datagram, err
	}

	validationErr := utils.ValidateJSON(datagram)
	if validationErr != nil {
		self.logger.Error("validaten failed for datagram", "pattern", datagram.Pattern, "validationErr", validationErr)
		return datagram, fmt.Errorf("validaten failed for datagram: %s", strings.Join(validationErr.Errors, " | "))
	}

	return datagram, nil
}

var jobDataQueue []structs.Datagram = []structs.Datagram{}
var jobSendMutex sync.Mutex

func (self *socketApi) JobServerSendData(jobClient websocket.WebsocketClient, datagram structs.Datagram) {
	jobDataQueue = append(jobDataQueue, datagram)
	self.processJobNow(jobClient)
}

func (self *socketApi) processJobNow(jobClient websocket.WebsocketClient) {
	jobSendMutex.Lock()
	defer jobSendMutex.Unlock()
	for i := 0; i < len(jobDataQueue); i++ {
		element := jobDataQueue[i]
		err := jobClient.WriteJSON(element)
		if err == nil {
			element.DisplaySentSummary(i+1, len(jobDataQueue))
			if isSuppressed := utils.Contains(structs.SUPPRESSED_OUTPUT_PATTERN, element.Pattern); !isSuppressed {
				self.logger.Debug("sent summary", "payload", element.Payload)
			}
			jobDataQueue = self.removeJobIndex(jobDataQueue, i)
		} else {
			self.logger.Error("Error writing json in job queue", "error", err)
			return
		}
	}
}

func (self *socketApi) removeJobIndex(s []structs.Datagram, index int) []structs.Datagram {
	if len(s) > index {
		return append(s[:index], s[index+1:]...)
	}
	return s
}

func (self *socketApi) versionTicker() {
	interval, err := strconv.Atoi(self.config.Get("MO_UPDATE_INTERVAL"))
	assert.Assert(err == nil, err)
	updateTicker := time.NewTicker(time.Second * time.Duration(interval))
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-updateTicker.C:
				self.updateCheck()
			}
		}
	}()
}

func (self *socketApi) updateCheck() {
	if !utils.IsProduction() {
		self.logger.Info("Skipping updates ... [not production]")
		return
	}

	self.logger.Info("Checking for updates ...")

	helmData, err := utils.GetVersionData(utils.HELM_INDEX)
	if err != nil {
		self.logger.Error("GetVersionData", "error", err.Error())
		return
	}
	// VALIDATE RESPONSE
	if len(helmData.Entries) < 1 {
		self.logger.Error("HelmIndex Entries length <= 0. Check the HelmIndex for errors.", "HelmIndex", utils.HELM_INDEX)
		return
	}
	mogeniusPlatform, doesExist := helmData.Entries["mogenius-platform"]
	if !doesExist {
		self.logger.Error("HelmIndex does not contain the field 'mogenius-platform'. Check the HelmIndex for errors.", "HelmIndex", utils.HELM_INDEX)
		return
	}
	if len(mogeniusPlatform) <= 0 {
		self.logger.Error("Field 'mogenius-platform' does not contain a proper version. Check the HelmIndex for errors.", "HelmIndex", utils.HELM_INDEX)
		return
	}
	var mok8smanager *utils.HelmDependency = nil
	for _, dep := range mogeniusPlatform[0].Dependencies {
		if dep.Name == "mogenius-k8s-manager" {
			mok8smanager = &dep
			break
		}
	}
	if mok8smanager == nil {
		self.logger.Error("The umbrella chart 'mogenius-platform' does not contain a dependency for 'mogenius-k8s-manager'. Check the HelmIndex for errors.", "HelmIndex", utils.HELM_INDEX)
		return
	}

	if version.Ver != mok8smanager.Version {
		fmt.Printf("\n####################################################################\n"+
			"####################################################################\n"+
			"######                  %s                ######\n"+
			"######               %s              ######\n"+
			"######                                                        ######\n"+
			"######                    Available: %s                    ######\n"+
			"######                    In-Use:    %s                    ######\n"+
			"######                                                        ######\n"+
			"######   %s   ######\n", shell.Colorize("Not updating might result in service interruption.", shell.Red)+
			"####################################################################\n"+
			"####################################################################\n",
			shell.Colorize("NEW VERSION AVAILABLE!", shell.Blue),
			shell.Colorize(" UPDATE AS FAST AS POSSIBLE", shell.Yellow),
			shell.Colorize(mok8smanager.Version, shell.Green),
			shell.Colorize(version.Ver, shell.Red),
		)
		self.notUpToDateAction(helmData)
	} else {
		self.logger.Debug(" Up-To-Date: 👍", "version", version.Ver)
	}
}

func (self *socketApi) notUpToDateAction(helmData *utils.HelmData) {
	localVer, err := semver.NewVersion(version.Ver)
	if err != nil {
		self.logger.Error("Error parsing local version", "error", err)
		return
	}

	remoteVer, err := semver.NewVersion(helmData.Entries["mogenius-k8s-manager"][0].Version)
	if err != nil {
		self.logger.Error("Error parsing remote version", "error", err)
		return
	}

	constraint, err := semver.NewConstraint(">= " + version.Ver)
	if err != nil {
		self.logger.Error("Error parsing constraint version", "error", err)
		return
	}

	_, errors := constraint.Validate(remoteVer)
	for _, m := range errors {
		self.logger.Error("failed to validate semver constraint", "remoteVer", remoteVer, "error", m)
	}
	// Local version > Remote version (likely development version)
	if remoteVer.LessThan(localVer) {
		self.logger.Warn("Your local version is greater than the remote version. AI thinks: You are likely a developer.",
			"localVer", localVer.String(),
			"remoteVer", remoteVer.String(),
		)
		return
	}

	// MAYOR CHANGES: MUST UPGRADE TO CONTINUE
	if remoteVer.GreaterThan(localVer) && remoteVer.Major() > localVer.Major() {
		self.logger.Error("Your version is too low to continue. Please upgrade to and try again.\n",
			"localVer", localVer.String(),
			"remoteVer", remoteVer.String(),
		)
		shutdown.SendShutdownSignal(true)
		select {}
	}

	// MINOR&PATCH CHANGES: SHOULD UPGRADE
	if remoteVer.GreaterThan(localVer) {
		self.logger.Warn("Your version is out-dated. Please upgrade to avoid service interruption.",
			"localVer", localVer.String(),
			"remoteVer", remoteVer.String(),
		)
		return
	}
}

func (self *socketApi) ExecuteCommandRequest(datagram structs.Datagram, httpApi HttpService) interface{} {
	if patternHandler, ok := self.patternHandler[datagram.Pattern]; ok {
		return patternHandler.Callback(datagram)
	}

	return NewMessageResponse(nil, fmt.Errorf("Pattern not found"))
}

type MessageResponseStatus string

const (
	StatusSuccess MessageResponseStatus = "success"
	StatusError   MessageResponseStatus = "error"
)

type MessageResponse struct {
	Status  MessageResponseStatus `json:"status"` // success, error
	Message string                `json:"message,omitempty"`
	Data    interface{}           `json:"data,omitempty"`
}

func NewMessageResponse(result interface{}, err error) MessageResponse {
	if err != nil {
		return MessageResponse{
			Status:  StatusError,
			Message: err.Error(),
		}
	}
	if str, ok := result.(string); ok {
		return MessageResponse{
			Status:  StatusSuccess,
			Message: str,
		}
	}
	return MessageResponse{
		Status: StatusSuccess,
		Data:   result,
	}
}

type ClusterResourceInfoDto struct {
	LoadBalancerExternalIps []string              `json:"loadBalancerExternalIps"`
	NodeStats               []dtos.NodeStat       `json:"nodeStats"`
	Country                 *utils.CountryDetails `json:"country"`
	Provider                string                `json:"provider"`
	CniConfig               []structs.CniData     `json:"cniConfig"`
}

type K8sManagerUpgradeRequest struct {
	Command string `json:"command" validate:"required"` // complete helm command from platform ui
}

func UpgradeK8sManager(r K8sManagerUpgradeRequest) *structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Upgrade mogenius platform", "UPGRADE", "", "")
	job.Start()
	kubernetes.UpgradeMyself(job, r.Command, &wg)
	wg.Wait()
	job.Finish()
	return job
}

type ListCronjobJobsRequest struct {
	ProjectId      string `json:"projectId" validate:"required"`
	NamespaceName  string `json:"namespaceName" validate:"required"`
	ControllerName string `json:"controllerName" validate:"required"`
}

func (self *socketApi) logStream(data services.ServiceLogStreamRequest, datagram structs.Datagram) services.ServiceLogStreamResult {
	_ = datagram
	result := services.ServiceLogStreamResult{}

	url, err := url.Parse(data.PostTo)
	if err != nil {
		result.Error = err.Error()
		result.Success = false
		self.logger.Error(result.Error)
		return result
	}

	pod := kubernetes.PodStatus(data.Namespace, data.PodId, false)
	terminatedState := kubernetes.LastTerminatedStateIfAny(pod)

	var previousResReq *rest.Request
	if terminatedState != nil {
		tmpPreviousResReq, err := services.PreviousPodLogStream(data.Namespace, data.PodId)
		if err != nil {
			self.logger.Error("failed to get previous pod log stream", "error", err)
		} else {
			previousResReq = tmpPreviousResReq
		}
	}

	restReq, err := services.PodLogStream(data)
	if err != nil {
		result.Error = err.Error()
		result.Success = false
		self.logger.Error(result.Error)
		return result
	}

	if terminatedState != nil {
		self.logger.Info("Logger try multiStreamData")
		go self.multiStreamData(previousResReq, restReq, terminatedState, url.String())
	} else {
		self.logger.Info("Logger try streamData")
		go self.streamData(restReq, url.String())
	}

	result.Success = true

	return result
}

func (self *socketApi) streamData(restReq *rest.Request, toServerUrl string) {
	ctx := context.Background()
	cancelCtx, endGofunc := context.WithCancel(ctx)
	stream, err := restReq.Stream(cancelCtx)
	if err != nil {
		self.logger.Error(err.Error())
	} else {
		structs.SendDataWs(toServerUrl, stream)
	}
	endGofunc()
}

func (self *socketApi) multiStreamData(previousRestReq *rest.Request, restReq *rest.Request, terminatedState *v1.ContainerStateTerminated, toServerUrl string) {
	ctx := context.Background()
	ctx, endGofunc := context.WithCancel(ctx)
	defer endGofunc()

	lastState := kubernetes.LastTerminatedStateToString(terminatedState)

	var previousStream io.ReadCloser
	if previousRestReq != nil {
		tmpPreviousStream, err := previousRestReq.Stream(ctx)
		if err != nil {
			self.logger.Error(err.Error())
			previousStream = io.NopCloser(strings.NewReader(fmt.Sprintln(err.Error())))
		} else {
			previousStream = tmpPreviousStream
		}
	}

	stream, err := restReq.Stream(ctx)
	if err != nil {
		self.logger.Error(err.Error())
		stream = io.NopCloser(strings.NewReader(fmt.Sprintln(err.Error())))
	}

	nl := strings.NewReader("\n")
	previousState := strings.NewReader(lastState)
	headlineLastLog := strings.NewReader("Last Log:\n")
	headlineCurrentLog := strings.NewReader("Current Log:\n")

	mergedStream := io.MultiReader(previousState, nl, headlineLastLog, nl, previousStream, nl, headlineCurrentLog, nl, stream)

	structs.SendDataWs(toServerUrl, io.NopCloser(mergedStream))
}

func (self *socketApi) execShConnection(podCmdConnectionRequest xterm.PodCmdConnectionRequest) {
	// allows to execute itself without being in $PATH (e.g. while developing locally)
	bin, err := os.Executable()
	if err != nil {
		self.logger.Error("failed to get current executable path", "error", err)
		return
	}

	cmd := exec.Command(
		bin,
		"exec",
		"--namespace",
		podCmdConnectionRequest.Namespace,
		"--pod",
		podCmdConnectionRequest.Pod,
		"--container",
		podCmdConnectionRequest.Container,
		"--",
		"sh",
	)

	xterm.XTermCommandStreamConnection(
		"exec-sh",
		podCmdConnectionRequest.WsConnection,
		podCmdConnectionRequest.Namespace,
		podCmdConnectionRequest.Controller,
		podCmdConnectionRequest.Pod,
		podCmdConnectionRequest.Container,
		cmd,
		nil,
	)
}

func (self *socketApi) logStreamConnection(podCmdConnectionRequest xterm.PodCmdConnectionRequest) {
	bin, err := os.Executable()
	if err != nil {
		self.logger.Error("failed to get current executable path", "error", err)
		return
	}

	cmd := exec.Command(
		bin,
		"logs",
		"--namespace",
		podCmdConnectionRequest.Namespace,
		"--pod",
		podCmdConnectionRequest.Pod,
		"--container",
		podCmdConnectionRequest.Container,
		"--tail-lines",
		podCmdConnectionRequest.LogTail,
	)

	xterm.XTermCommandStreamConnection(
		"log",
		podCmdConnectionRequest.WsConnection,
		podCmdConnectionRequest.Namespace,
		podCmdConnectionRequest.Controller,
		podCmdConnectionRequest.Pod,
		podCmdConnectionRequest.Container,
		cmd,
		services.GetPreviousLogContent(podCmdConnectionRequest),
	)
}

func buildLogStreamConnection(buildLogConnectionRequest xterm.BuildLogConnectionRequest) {
	xterm.XTermBuildLogStreamConnection(
		buildLogConnectionRequest.WsConnection,
		buildLogConnectionRequest.Namespace,
		buildLogConnectionRequest.Controller,
		buildLogConnectionRequest.Container,
		buildLogConnectionRequest.BuildTask,
		buildLogConnectionRequest.BuildId,
	)
}

func componentLogStreamConnection(componentLogConnectionRequest xterm.ComponentLogConnectionRequest) {
	xterm.XTermComponentStreamConnection(
		componentLogConnectionRequest.WsConnection,
		componentLogConnectionRequest.Component,
		componentLogConnectionRequest.Namespace,
		componentLogConnectionRequest.Controller,
		componentLogConnectionRequest.Release,
	)
}

func podEventStreamConnection(buildLogConnectionRequest xterm.PodEventConnectionRequest) {
	xterm.XTermPodEventStreamConnection(
		buildLogConnectionRequest.WsConnection,
		buildLogConnectionRequest.Namespace,
		buildLogConnectionRequest.Controller,
	)
}

func scanImageLogStreamConnection(buildLogConnectionRequest xterm.ScanImageLogConnectionRequest) {
	xterm.XTermScanImageLogStreamConnection(
		buildLogConnectionRequest.WsConnection,
		buildLogConnectionRequest.Namespace,
		buildLogConnectionRequest.Controller,
		buildLogConnectionRequest.Container,
		buildLogConnectionRequest.CmdType,
		buildLogConnectionRequest.ScanImageType,
		buildLogConnectionRequest.ContainerRegistryUrl,
		&buildLogConnectionRequest.ContainerRegistryUser,
		&buildLogConnectionRequest.ContainerRegistryPat,
	)
}

func ExecuteBinaryRequestUpload(datagram structs.Datagram) *services.FilesUploadRequest {
	data := services.FilesUploadRequest{}
	structs.MarshalUnmarshal(&datagram, &data)
	return &data
}
