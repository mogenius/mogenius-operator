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
	ExecuteCommandRequest(datagram structs.Datagram) interface{}
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
	Deprecated        bool
	DeprecatedMessage string
	NeedsUser         bool
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
	assert.Assert(!exists, "patterns may only be registered once", pattern)

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
	assert.Assert(!exists, "patterns may only be registered once", pattern)

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
			_ = self.loadRequest(&datagram, &data)
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
			_ = self.loadRequest(&datagram, &data)
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
		"upgrade-container-registry",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UpgradeContainerRegistry()
		},
	)

	self.RegisterPatternHandler(
		"upgrade-metallb",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UpgradeMetalLb()
		},
	)

	self.RegisterPatternHandler(
		"upgrade-kepler",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return services.UpgradeKepler()
		},
	)

	self.RegisterPatternHandlerRaw(
		"stats/podstat/all-for-pod",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.StatsDataRequest{}
			_ = self.loadRequest(&datagram, &data)
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
		"stats/podstat/last-for-pod",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.StatsDataRequest{}
			_ = self.loadRequest(&datagram, &data)
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
		"stats/podstat/all-for-controller",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := kubernetes.K8sController{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetPodStatsEntriesForController(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"stats/podstat/last-for-controller",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := kubernetes.K8sController{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetLastPodStatsEntryForController(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"stats/traffic/all-for-controller",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := kubernetes.K8sController{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetTrafficStatsEntriesForController(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"stats/traffic/sum-for-controller",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := kubernetes.K8sController{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetTrafficStatsEntrySumForController(data, false)
		},
	)

	self.RegisterPatternHandlerRaw(
		"stats/traffic/last-for-controller",
		PatternConfig{
			Deprecated:        true,
			DeprecatedMessage: `Use "stats/traffic/sum-for-controller" instead`,
		},
		func(datagram structs.Datagram) interface{} {
			data := kubernetes.K8sController{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetTrafficStatsEntrySumForController(data, false)
		},
	)

	self.RegisterPatternHandlerRaw(
		"stats/traffic/for-controller-socket-connections",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := kubernetes.K8sController{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetSocketConnectionsForController(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"stats/traffic/all-for-pod",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.StatsDataRequest{}
			_ = self.loadRequest(&datagram, &data)
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
		"stats/traffic/sum-for-pod",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.StatsDataRequest{}
			_ = self.loadRequest(&datagram, &data)
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
		"stats/traffic/last-for-pod",
		PatternConfig{
			Deprecated:        true,
			DeprecatedMessage: `Use "stats/traffic/sum-for-pod" instead`,
		},
		func(datagram structs.Datagram) interface{} {
			data := services.StatsDataRequest{}
			_ = self.loadRequest(&datagram, &data)
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
		"stats/podstat/all-for-namespace",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NsStatsDataRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetPodStatsEntriesForNamespace(data.Namespace)
		},
	)

	self.RegisterPatternHandlerRaw(
		"stats/podstat/last-for-namespace",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NsStatsDataRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetLastPodStatsEntriesForNamespace(data.Namespace)
		},
	)

	self.RegisterPatternHandlerRaw(
		"stats/traffic/all-for-namespace",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NsStatsDataRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetTrafficStatsEntriesForNamespace(data.Namespace)
		},
	)

	self.RegisterPatternHandlerRaw(
		"stats/traffic/sum-for-namespace",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NsStatsDataRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetTrafficStatsEntriesSumForNamespace(data.Namespace)
		},
	)

	self.RegisterPatternHandlerRaw(
		"stats/traffic/last-for-namespace",
		PatternConfig{
			Deprecated:        true,
			DeprecatedMessage: `Use "stats/traffic/sum-for-namespace" instead`,
		},
		func(datagram structs.Datagram) interface{} {
			data := services.NsStatsDataRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetTrafficStatsEntriesSumForNamespace(data.Namespace)
		},
	)

	self.RegisterPatternHandlerRaw(
		"metrics/deployment/average-utilization",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := kubernetes.K8sController{}
			data.Kind = "Deployment"
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.GetAverageUtilizationForDeployment(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"files/list",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.FilesListRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.List(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"files/create-folder",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.FilesCreateFolderRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.CreateFolder(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"files/rename",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.FilesRenameRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.Rename(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"files/chown",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.FilesChownRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.Chown(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"files/chmod",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.FilesChmodRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.Chmod(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"files/delete",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.FilesDeleteRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.Delete(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"files/download",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.FilesDownloadRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.Download(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"files/info",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := dtos.PersistentFileRequestDto{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.Info(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/execute-helm-chart-task",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterHelmRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.InstallHelmChart(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/uninstall-helm-chart",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterHelmUninstallRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.DeleteHelmChart(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/tcp-udp-configuration",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			return services.TcpUdpClusterConfiguration()
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/backup",
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
		"cluster/read-configmap",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterGetConfigMap{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.GetConfigMapWR(data.Namespace, data.Name)
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/write-configmap",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterWriteConfigMap{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.WriteConfigMap(data.Namespace, data.Name, data.Data, data.Labels)
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/list-configmaps",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterListWorkloads{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.ListConfigMapWithFieldSelector(data.Namespace, data.LabelSelector, data.Prefix)
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/read-deployment",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterGetDeployment{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.GetDeploymentResult(data.Namespace, data.Name)
		},
	)

	// // TODO
	// // case structs.PAT_CLUSTER_WRITE_DEPLOYMENT:
	// // 	data := ClusterWriteDeployment{}
	// // 	_ = self.loadRequest(&datagram, &data)
	// // 	if err := utils.ValidateJSON(data); err != nil {
	// // 		return err
	// // 	}
	// // 	return kubernetes.WriteConfigMap(data.Namespace, data.Name, data.Data, data.Labels)

	self.RegisterPatternHandlerRaw(
		"cluster/list-deployments",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterListWorkloads{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.ListDeploymentsWithFieldSelector(data.Namespace, data.LabelSelector, data.Prefix)
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/read-persistent-volume-claim",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterGetPersistentVolume{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(kubernetes.GetPersistentVolumeClaim(data.Namespace, data.Name))
		},
	)

	// // TODO
	// // case structs.PAT_CLUSTER_WRITE_PERSISTENT_VOLUME_CLAIM:
	// // 	data := ClusterWritePersistentVolume{}
	// // 	_ = self.loadRequest(&datagram, &data)
	// // 	if err := utils.ValidateJSON(data); err != nil {
	// // 		return err
	// // 	}
	// // 	return kubernetes.WritePersistentVolume(data.Namespace, data.Name, data.Data, data.Labels)

	self.RegisterPatternHandlerRaw(
		"cluster/list-persistent-volume-claims",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterListWorkloads{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			// AllPersistentVolumes
			return kubernetes.ListPersistentVolumeClaimsWithFieldSelector(data.Namespace, data.LabelSelector, data.Prefix)
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/update-local-tls-secret",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ClusterUpdateLocalTlsSecret{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.CreateMogeniusContainerRegistryTlsSecret(data.LocalTlsCrt, data.LocalTlsKey)
		},
	)

	self.RegisterPatternHandlerRaw(
		"namespace/create",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceCreateRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Project.AddSecretsToRedaction()
			return services.CreateNamespace(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"namespace/delete",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceDeleteRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.DeleteNamespace(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"namespace/shutdown",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceShutdownRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			return services.ShutdownNamespace(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"namespace/pod-ids",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespacePodIdsRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.PodIds(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"namespace/validate-cluster-pods",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceValidateClusterPodsRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.ValidateClusterPods(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"namespace/validate-ports",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceValidatePortsRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.ValidateClusterPorts(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"namespace/list-all",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			return services.ListAllNamespaces()
		},
	)

	self.RegisterPatternHandlerRaw(
		"namespace/gather-all-resources",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceGatherAllResourcesRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.ListAllResourcesForNamespace(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"namespace/backup",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceBackupRequest{}
			_ = self.loadRequest(&datagram, &data)
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
		"namespace/restore",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceRestoreRequest{}
			_ = self.loadRequest(&datagram, &data)
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
		"namespace/resource-yaml",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NamespaceResourceYamlRequest{}
			_ = self.loadRequest(&datagram, &data)
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
		"cluster/helm-repo-add",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmRepoAddRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmRepoAdd(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/helm-repo-patch",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmRepoPatchRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmRepoPatch(data))
		},
	)

	self.RegisterPatternHandler(
		"cluster/helm-repo-update",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return helm.HelmRepoUpdate()
		},
	)

	self.RegisterPatternHandler(
		"cluster/helm-repo-list",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return helm.HelmRepoList()
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/helm-chart-remove",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmRepoRemoveRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmRepoRemove(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/helm-chart-search",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmChartSearchRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmChartSearch(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/helm-chart-install",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmChartInstallUpgradeRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmChartInstall(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/helm-chart-show",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmChartShowRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmChartShow(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/helm-chart-versions",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmChartVersionRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmChartVersion(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/helm-release-upgrade",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmChartInstallUpgradeRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmReleaseUpgrade(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/helm-release-uninstall",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmReleaseUninstallRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmReleaseUninstall(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/helm-release-list",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmReleaseListRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmReleaseList(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/helm-release-status",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmReleaseStatusRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmReleaseStatus(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/helm-release-history",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmReleaseHistoryRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmReleaseHistory(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/helm-release-rollback",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmReleaseRollbackRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmReleaseRollback(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/helm-release-get",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmReleaseGetRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmReleaseGet(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/helm-release-get-workloads",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := helm.HelmReleaseGetWorkloadsRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmReleaseGetWorkloads(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/create",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceUpdateRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			data.Project.AddSecretsToRedaction()
			return services.UpdateService(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/delete",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceDeleteRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			data.Project.AddSecretsToRedaction()
			return services.DeleteService(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/pod-ids",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceGetPodIdsRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.ServicePodIds(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"SERVICE_POD_EXISTS",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServicePodExistsRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.ServicePodExists(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"SERVICE_PODS",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServicePodsRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.ServicePodStatus(data)
		},
	)

	// // case structs.PAT_SERVICE_SET_IMAGE:
	// // 	data := ServiceSetImageRequest{}
	// // 	_ = self.loadRequest(&datagram, &data)
	// // 	if err := utils.ValidateJSON(data); err != nil {
	// // 		return err
	// // 	}
	// // 	return SetImage(data)

	self.RegisterPatternHandlerRaw(
		"service/log",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceGetLogRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.PodLog(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/log-error",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceGetLogRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.PodLogError(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/resource-status",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceResourceStatusRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.PodStatus(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/restart",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceRestartRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			return services.Restart(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/stop",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceStopRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			return services.StopService(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/start",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceStartRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			return services.StartService(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/update-service",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceUpdateRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Project.AddSecretsToRedaction()
			data.Service.AddSecretsToRedaction()
			return services.UpdateService(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/trigger-job",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceTriggerJobRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.TriggerJobService(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/status",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceStatusRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.StatusServiceDebounced(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/log-stream",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.ServiceLogStreamRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.logStream(data, datagram)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/exec-sh-connection-request",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.PodCmdConnectionRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go self.execShConnection(data)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/log-stream-connection-request",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.PodCmdConnectionRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go self.logStreamConnection(data)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/build-log-stream-connection-request",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.BuildLogConnectionRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go buildLogStreamConnection(data)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/component-log-stream-connection-request",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.ComponentLogConnectionRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go componentLogStreamConnection(data)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/pod-event-stream-connection-request",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.PodEventConnectionRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go podEventStreamConnection(data)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/scan-image-log-stream-connection-request",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.ScanImageLogConnectionRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.AddSecretsToRedaction()
			go scanImageLogStreamConnection(data)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/cluster-tool-stream-connection-request",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.ClusterToolConnectionRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go services.XTermClusterToolStreamConnection(data)
			return nil
		},
	)

	self.RegisterPatternHandler(
		"list/all-workloads",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return kubernetes.GetAvailableResources()
		},
	)

	self.RegisterPatternHandler(
		"get/workload-list",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.SyncResourceEntry{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.GetUnstructuredResourceListFromStore(data.Group, data.Kind, data.Version, data.Name, data.Namespace)
		},
	)

	self.RegisterPatternHandler(
		"get/namespace-workload-list",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := kubernetes.GetUnstructuredNamespaceResourceListRequest{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.GetUnstructuredNamespaceResourceList(data.Namespace, data.Whitelist, data.Blacklist)
		},
	)

	self.RegisterPatternHandler(
		"get/labeled-workload-list",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := kubernetes.GetUnstructuredLabeledResourceListRequest{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.GetUnstructuredLabeledResourceList(data.Label, data.Whitelist, data.Blacklist)
		},
	)

	self.RegisterPatternHandler(
		"describe/workload",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.SyncResourceItem{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.DescribeUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.ResourceName)
		},
	)

	self.RegisterPatternHandler(
		"create/new-workload",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.SyncResourceData{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.CreateUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.YamlData)
		},
	)

	self.RegisterPatternHandler(
		"get/workload",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.SyncResourceItem{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.GetUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.ResourceName)
		},
	)

	self.RegisterPatternHandler(
		"get/workload-status",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := kubernetes.GetWorkloadStatusRequest{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.GetWorkloadStatus(data)
		},
	)

	self.RegisterPatternHandler(
		"get/workload-example",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.SyncResourceItem{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.GetResourceTemplateYaml(data.Group, data.Version, data.Name, data.Kind, data.Namespace, data.ResourceName), nil
		},
	)

	self.RegisterPatternHandler(
		"update/workload",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.SyncResourceData{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.UpdateUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.YamlData)
		},
	)

	self.RegisterPatternHandler(
		"delete/workload",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.SyncResourceItem{}
			_ = self.loadRequest(&datagram, &data)
			return nil, kubernetes.DeleteUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.ResourceName)
		},
	)

	self.RegisterPatternHandler(
		"get/workspaces",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return self.apiService.GetAllWorkspaces()
		},
	)

	self.RegisterPatternHandler(
		"create/workspace",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestCreateWorkspace{}
			err := self.loadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
			return self.apiService.CreateWorkspace(data.Name, v1alpha1.NewWorkspaceSpec(
				data.DisplayName,
				data.Resources,
			))
		},
	)

	self.RegisterPatternHandler(
		"get/workspace",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestGetWorkspace{}
			err := self.loadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
			return self.apiService.GetWorkspace(data.Name)
		},
	)

	self.RegisterPatternHandler(
		"update/workspace",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestUpdateWorkspace{}
			err := self.loadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
			return self.apiService.UpdateWorkspace(data.Name, v1alpha1.NewWorkspaceSpec(
				data.DisplayName,
				data.Resources,
			))
		},
	)

	self.RegisterPatternHandler(
		"delete/workspace",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestDeleteWorkspace{}
			err := self.loadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
			return self.apiService.DeleteWorkspace(data.Name)
		},
	)

	self.RegisterPatternHandler(
		"get/users",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return self.apiService.GetAllUsers()
		},
	)

	self.RegisterPatternHandler(
		"create/user",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestCreateUser{}
			err := self.loadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
			return self.apiService.CreateUser(data.Name, v1alpha1.NewUserSpec(data.MogeniusId))
		},
	)

	self.RegisterPatternHandler(
		"get/user",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestGetUser{}
			err := self.loadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
			return self.apiService.GetUser(data.Name)
		},
	)

	self.RegisterPatternHandler(
		"update/user",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestUpdateUser{}
			err := self.loadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
			return self.apiService.UpdateUser(data.Name, v1alpha1.NewUserSpec(data.MogeniusId))
		},
	)

	self.RegisterPatternHandler(
		"delete/user",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestDeleteUser{}
			err := self.loadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
			return self.apiService.DeleteUser(data.Name)
		},
	)

	self.RegisterPatternHandler(
		"get/teams",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return self.apiService.GetAllTeams()
		},
	)

	self.RegisterPatternHandler(
		"create/team",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestCreateTeam{}
			err := self.loadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
			return self.apiService.CreateTeam(data.Name, v1alpha1.NewTeamSpec(data.DisplayName, data.Users))
		},
	)

	self.RegisterPatternHandler(
		"get/team",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestGetTeam{}
			err := self.loadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
			return self.apiService.GetTeam(data.Name)
		},
	)

	self.RegisterPatternHandler(
		"update/team",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestUpdateTeam{}
			err := self.loadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
			return self.apiService.UpdateTeam(data.Name, v1alpha1.NewTeamSpec(data.DisplayName, data.Users))
		},
	)

	self.RegisterPatternHandler(
		"delete/team",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestDeleteTeam{}
			err := self.loadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
			return self.apiService.DeleteTeam(data.Name)
		},
	)

	self.RegisterPatternHandler(
		"get/grants",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return self.apiService.GetAllGrants()
		},
	)

	self.RegisterPatternHandler(
		"create/grant",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestCreateGrant{}
			err := self.loadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
			return self.apiService.CreateGrant(data.Name, v1alpha1.NewGrantSpec(
				data.Grantee,
				data.TargetType,
				data.TargetName,
				data.Role,
			))
		},
	)

	self.RegisterPatternHandler(
		"get/grant",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestGetGrant{}
			err := self.loadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
			return self.apiService.GetGrant(data.Name)
		},
	)

	self.RegisterPatternHandler(
		"update/grant",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestUpdateGrant{}
			err := self.loadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
			return self.apiService.UpdateGrant(data.Name, v1alpha1.NewGrantSpec(
				data.Grantee,
				data.TargetType,
				data.TargetName,
				data.Role,
			))
		},
	)

	self.RegisterPatternHandler(
		"delete/grant",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := utils.WebsocketRequestDeleteGrant{}
			err := self.loadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
			return self.apiService.DeleteGrant(data.Name)
		},
	)

	self.RegisterPatternHandlerRaw(
		"build/builder-status",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			return kubernetes.GetDb().GetBuilderStatus()
		},
	)

	self.RegisterPatternHandlerRaw(
		"build/info",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.BuildJobStatusRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.GetDb().GetBuildJobInfosFromDb(data.BuildId)
		},
	)

	self.RegisterPatternHandlerRaw(
		"build/last-infos",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.BuildTaskRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.GetDb().GetLastBuildJobInfosFromDb(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"build/list-all",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			return services.ListAll()
		},
	)

	self.RegisterPatternHandlerRaw(
		"build/list-by-project",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.ListBuildByProjectIdRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.ListByProjectId(data.ProjectId)
		},
	)

	self.RegisterPatternHandlerRaw(
		"build/add",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.BuildJob{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Project.AddSecretsToRedaction()
			data.Service.AddSecretsToRedaction()
			return services.AddBuildJob(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"build/cancel",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.BuildJob{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Project.AddSecretsToRedaction()
			data.Service.AddSecretsToRedaction()
			return services.Cancel(data.BuildId)
		},
	)

	self.RegisterPatternHandlerRaw(
		"build/delete",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.BuildJobStatusRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.DeleteBuild(data.BuildId)
		},
	)

	self.RegisterPatternHandlerRaw(
		"build/last-job-of-services",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.BuildTaskListOfServicesRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.LastBuildInfosOfServices(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"build/job-list-of-service",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.BuildTaskRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.GetDb().GetBuildJobInfosListFromDb(data.Namespace, data.Controller, data.Container)
		},
	)

	self.RegisterPatternHandlerRaw(
		"build/delete-of-service",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := structs.BuildTaskRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			kubernetes.GetDb().DeleteAllBuildData(data.Namespace, data.Controller, data.Container)
			return nil
		},
	)

	// //case structs.PAT_BUILD_LAST_JOB_INFO_OF_SERVICE:
	// //	data := structs.BuildServiceRequest{}
	// //	_ = self.loadRequest(&datagram, &data)
	// //	if err := utils.ValidateJSON(data); err != nil {
	// //		return err
	// //	}
	// //	return LastBuildForService(data.ServiceId)

	self.RegisterPatternHandlerRaw(
		"storage/create-volume",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NfsVolumeRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.CreateMogeniusNfsVolume(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"storage/delete-volume",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NfsVolumeRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.DeleteMogeniusNfsVolume(data)
		},
	)

	// // case structs.PAT_STORAGE_BACKUP_VOLUME:
	// // 	data := NfsVolumeBackupRequest{}
	// // 	_ = self.loadRequest(&datagram, &data)
	// // 	if err := utils.ValidateJSON(data); err != nil {
	// // 		return err
	// // 	}
	// // 	data.AddSecretsToRedaction()
	// // 	return BackupMogeniusNfsVolume(data)
	// // case structs.PAT_STORAGE_RESTORE_VOLUME:
	// // 	data := NfsVolumeRestoreRequest{}
	// // 	_ = self.loadRequest(&datagram, &data)
	// // 	if err := utils.ValidateJSON(data); err != nil {
	// // 		return err
	// // 	}
	// // 	data.AddSecretsToRedaction()
	// // 	return RestoreMogeniusNfsVolume(data)

	self.RegisterPatternHandlerRaw(
		"storage/stats",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NfsVolumeStatsRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.StatsMogeniusNfsVolume(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"storage/namespace/stats",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NfsNamespaceStatsRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.StatsMogeniusNfsNamespace(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"storage/status",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := services.NfsStatusRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.StatusMogeniusNfs(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"log/list-all",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			return kubernetes.GetDb().ListLogFromDb()
		},
	)

	// // - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// // External Secrets
	// // - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -

	self.RegisterPatternHandlerRaw(
		"external-secret-store/create",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.CreateSecretsStoreRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return controllers.CreateExternalSecretStore(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"external-secret-store/list",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.ListSecretStoresRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return controllers.ListExternalSecretsStores(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"external-secret/list-available-secrets",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.ListSecretsRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return controllers.ListAvailableExternalSecrets(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"external-secret-store/delete",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.DeleteSecretsStoreRequest{}
			_ = self.loadRequest(&datagram, &data)
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
		"attach/labeled_network_policy",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.AttachLabeledNetworkPolicyRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(controllers.AttachLabeledNetworkPolicy(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"detach/labeled_network_policy",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.DetachLabeledNetworkPolicyRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(controllers.DetachLabeledNetworkPolicy(data))
		},
	)

	self.RegisterPatternHandler(
		"list/labeled_network_policy_ports",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return controllers.ListLabeledNetworkPolicyPorts()
		},
	)

	self.RegisterPatternHandlerRaw(
		"list/conflicting_network_policies",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.ListConflictingNetworkPoliciesRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(controllers.ListAllConflictingNetworkPolicies(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"remove/conflicting_network_policies",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.RemoveConflictingNetworkPoliciesRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(controllers.RemoveConflictingNetworkPolicies(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"list/controller_network_policies",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.ListControllerLabeledNetworkPoliciesRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(controllers.ListControllerLabeledNetwork(data))
		},
	)

	self.RegisterPatternHandler(
		"update/network_policies_template",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			data := []kubernetes.NetworkPolicy{}
			_ = self.loadRequest(&datagram, &data)
			return nil, controllers.UpdateNetworkPolicyTemplate(data)
		},
	)

	self.RegisterPatternHandler(
		"list/all_network_policies",
		PatternConfig{},
		func(datagram structs.Datagram) (interface{}, error) {
			return controllers.ListAllNetworkPolicies()
		},
	)

	self.RegisterPatternHandlerRaw(
		"list/namespace_network_policies",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.ListNamespaceLabeledNetworkPoliciesRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(controllers.ListNamespaceNetworkPolicies(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"enforce/network_policy_manager",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.EnforceNetworkPolicyManagerRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(nil, controllers.EnforceNetworkPolicyManager(data.NamespaceName))
		},
	)

	self.RegisterPatternHandlerRaw(
		"disable/network_policy_manager",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.DisableNetworkPolicyManagerRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(nil, controllers.DisableNetworkPolicyManager(data.NamespaceName))
		},
	)

	self.RegisterPatternHandlerRaw(
		"remove/unmanaged_network_policies",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.RemoveUnmanagedNetworkPoliciesRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(nil, controllers.RemoveUnmanagedNetworkPolicies(data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"list/only_namespace_network_policies",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := controllers.ListNamespaceLabeledNetworkPoliciesRequest{}
			_ = self.loadRequest(&datagram, &data)
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
		"list/cronjob-jobs",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := ListCronjobJobsRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.ListCronjobJobs(data.ControllerName, data.NamespaceName, data.ProjectId)
		},
	)

	self.RegisterPatternHandlerRaw(
		"live-stream/nodes-traffic",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.WsConnectionRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go self.xtermService.LiveStreamConnection(data, datagram, self.httpService)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		"live-stream/nodes-memory",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.WsConnectionRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go self.xtermService.LiveStreamConnection(data, datagram, self.httpService)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		"live-stream/nodes-cpu",
		PatternConfig{},
		func(datagram structs.Datagram) interface{} {
			data := xterm.WsConnectionRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			go self.xtermService.LiveStreamConnection(data, datagram, self.httpService)
			return nil
		},
	)
}

func (self *socketApi) loadRequest(datagram *structs.Datagram, data interface{}) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary

	bytes, err := json.Marshal(datagram.Payload)
	if err != nil {
		datagram.Err = err.Error()
		return err
	}

	err = json.Unmarshal(bytes, data)
	if err != nil {
		datagram.Err = err.Error()
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

		if datagram.Pattern == "files/upload" {
			preparedFileRequest = self.executeBinaryRequestUpload(datagram)

			var ack = structs.CreateDatagramAck("ack:files/upload:datagram", datagram.Id)
			self.JobServerSendData(self.client, ack)
			continue
		}

		if self.patternHandlerExists(datagram.Pattern) {
			semaphoreChan <- struct{}{}

			wg.Add(1)
			go func() {
				defer wg.Done()
				responsePayload := self.ExecuteCommandRequest(datagram)
				result := structs.Datagram{
					Id:        datagram.Id,
					Pattern:   datagram.Pattern,
					Payload:   responsePayload,
					CreatedAt: datagram.CreatedAt,
				}
				self.JobServerSendData(self.client, result)
				<-semaphoreChan
			}()
		}
	}
	self.logger.Debug("api messagehandler finished as the websocket client was terminated")
}

func (self *socketApi) patternHandlerExists(pattern string) bool {
	_, ok := self.patternHandler[pattern]
	return ok
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
			self.logger.Debug("sent summary", "payload", element.Payload)
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

func (self *socketApi) ExecuteCommandRequest(datagram structs.Datagram) interface{} {
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

func (self *socketApi) executeBinaryRequestUpload(datagram structs.Datagram) *services.FilesUploadRequest {
	data := services.FilesUploadRequest{}
	structs.MarshalUnmarshal(&datagram, &data)
	return &data
}
