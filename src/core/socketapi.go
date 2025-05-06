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
	"mogenius-k8s-manager/src/networkmonitor"
	"mogenius-k8s-manager/src/schema"
	"mogenius-k8s-manager/src/secrets"
	"mogenius-k8s-manager/src/services"
	"mogenius-k8s-manager/src/shell"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeyclient"
	"mogenius-k8s-manager/src/version"
	"mogenius-k8s-manager/src/websocket"
	"mogenius-k8s-manager/src/xterm"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	gorillawebsocket "github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
	"helm.sh/helm/v3/pkg/release"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

type SocketApi interface {
	Link(
		httpService HttpService,
		xtermService XtermService,
		dbstatsModule ValkeyStatsDb,
		apiService Api,
		moKubernetes MoKubernetes,
	)
	Run()
	ExecuteCommandRequest(datagram structs.Datagram) interface{}
	ParseDatagram(data []byte) (structs.Datagram, error)
	RegisterPatternHandler(
		pattern string,
		config PatternConfig,
		callback func(datagram structs.Datagram) (any, error),
	)
	RegisterPatternHandlerRaw(
		pattern string,
		config PatternConfig,
		callback func(datagram structs.Datagram) any,
	)
	PatternConfigs() map[string]PatternConfig
	NormalizePatternName(pattern string) string
	AssertPatternsUnique()
}

type socketApi struct {
	logger *slog.Logger

	jobClient    websocket.WebsocketClient
	eventsClient websocket.WebsocketClient

	config       config.ConfigModule
	valkeyClient valkeyclient.ValkeyClient
	dbstats      ValkeyStatsDb

	// the patternHandler should only be edited on startup
	patternHandlerLock sync.RWMutex
	patternHandler     map[string]PatternHandler
	httpService        HttpService
	xtermService       XtermService
	apiService         Api
	moKubernetes       MoKubernetes
}

type PatternHandler struct {
	Config   PatternConfig
	Callback func(datagram structs.Datagram) any
}

type PatternConfig struct {
	RequestSchema     *schema.Schema `json:"requestSchema,omitempty"`
	ResponseSchema    *schema.Schema `json:"responseSchema,omitempty"`
	Deprecated        bool           `json:"deprecated,omitempty"`
	DeprecatedMessage string         `json:"deprecatedMessage,omitempty"`
	NeedsUser         bool           `json:"needsUser,omitempty"`
}

func NewSocketApi(
	logger *slog.Logger,
	configModule config.ConfigModule,
	jobClient websocket.WebsocketClient,
	eventsClient websocket.WebsocketClient,
	valkeyClient valkeyclient.ValkeyClient,
) SocketApi {
	self := &socketApi{}
	self.config = configModule
	self.jobClient = jobClient
	self.eventsClient = eventsClient
	self.logger = logger
	self.patternHandler = map[string]PatternHandler{}
	self.valkeyClient = valkeyClient

	self.registerPatterns()

	return self
}

func (self *socketApi) Link(
	httpService HttpService,
	xtermService XtermService,
	dbstatsModule ValkeyStatsDb,
	apiService Api,
	moKubernetes MoKubernetes,
) {
	assert.Assert(apiService != nil)
	assert.Assert(httpService != nil)
	assert.Assert(xtermService != nil)
	assert.Assert(dbstatsModule != nil)
	assert.Assert(moKubernetes != nil)

	self.apiService = apiService
	self.httpService = httpService
	self.xtermService = xtermService
	self.dbstats = dbstatsModule
	self.moKubernetes = moKubernetes
}

func (self *socketApi) Run() {
	assert.Assert(self.apiService != nil)
	assert.Assert(self.httpService != nil)
	assert.Assert(self.xtermService != nil)

	self.AssertPatternsUnique()
	self.startK8sManager()

	go structs.ConnectToEventQueue(self.eventsClient)
	go structs.ConnectToJobQueue(self.jobClient)
}

func (self *socketApi) AssertPatternsUnique() {
	patternConfigs := self.PatternConfigs()
	var patterns []string

	for pattern := range patternConfigs {
		normalizedPattern := self.NormalizePatternName(pattern)
		assert.Assert(!slices.Contains(patterns, normalizedPattern), "duplicate normalized pattern", normalizedPattern)
		patterns = append(patterns, normalizedPattern)
	}
}

func (self *socketApi) RegisterPatternHandler(
	pattern string,
	config PatternConfig,
	callback func(datagram structs.Datagram) (any, error),
) {
	self.patternHandlerLock.Lock()
	defer self.patternHandlerLock.Unlock()

	_, exists := self.patternHandler[pattern]
	assert.Assert(!exists, "patterns may only be registered once", pattern)

	self.patternHandler[pattern] = PatternHandler{
		Config: config,
		Callback: func(datagram structs.Datagram) any {
			result, err := callback(datagram)
			return NewMessageResponse(result, err)
		},
	}
}

func (self *socketApi) RegisterPatternHandlerRaw(
	pattern string,
	config PatternConfig,
	callback func(datagram structs.Datagram) any,
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

func (self *socketApi) PatternConfigs() map[string]PatternConfig {
	patterns := map[string]PatternConfig{}

	for pattern, handler := range self.patternHandler {
		patterns[pattern] = handler.Config
	}

	return patterns
}

func (self *socketApi) registerPatterns() {
	{
		type Response struct {
			BuildInfo struct {
				BuildType string          `json:"buildType"`
				Version   version.Version `json:"version,omitempty"`
			} `json:"buildInfo,omitempty"`
			Features struct{}                 `json:"features,omitempty"`
			Patterns map[string]PatternConfig `json:"patterns,omitempty"`
		}

		self.RegisterPatternHandler(
			"describe",
			PatternConfig{
				ResponseSchema: schema.Generate(Response{}),
			},
			func(datagram structs.Datagram) (any, error) {
				resp := Response{}
				resp.BuildInfo.BuildType = "prod"
				if utils.IsDevBuild() {
					resp.BuildInfo.BuildType = "dev"
				}
				resp.BuildInfo.Version = *version.NewVersion()
				resp.Patterns = self.PatternConfigs()
				return resp, nil
			},
		)
	}

	self.RegisterPatternHandlerRaw(
		"K8sNotification",
		PatternConfig{},
		func(datagram structs.Datagram) any {
			self.logger.Info("Received pattern", "pattern", datagram.Pattern)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		"ClusterStatus",
		PatternConfig{
			ResponseSchema: schema.Generate(dtos.ClusterStatusDto{}),
		},
		func(datagram structs.Datagram) any {
			return kubernetes.ClusterStatus()
		},
	)

	{
		type Response struct {
			LoadBalancerExternalIps []string              `json:"loadBalancerExternalIps"`
			NodeStats               []dtos.NodeStat       `json:"nodeStats"`
			Country                 *utils.CountryDetails `json:"country"`
			Provider                string                `json:"provider"`
			CniConfig               []structs.CniData     `json:"cniConfig"`
		}

		self.RegisterPatternHandlerRaw(
			"ClusterResourceInfo",
			PatternConfig{
				ResponseSchema: schema.Generate(Response{}),
			},
			func(datagram structs.Datagram) any {
				nodeStats := self.moKubernetes.GetNodeStats()
				loadBalancerExternalIps := kubernetes.GetClusterExternalIps()
				country, _ := utils.GuessClusterCountry()
				cniConfig, _ := self.dbstats.GetCniData()
				return Response{
					NodeStats:               nodeStats,
					LoadBalancerExternalIps: loadBalancerExternalIps,
					Country:                 country,
					Provider:                string(utils.ClusterProviderCached),
					CniConfig:               cniConfig,
				}
			},
		)
	}

	{
		type Request struct {
			Command string `json:"command" validate:"required"` // complete helm command from platform ui
		}
		self.RegisterPatternHandlerRaw(
			"UpgradeK8sManager",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.Generate(&structs.Job{}),
			},
			func(datagram structs.Datagram) any {
				data := Request{}
				_ = self.loadRequest(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return err
				}
				return self.upgradeK8sManager(data.Command)
			},
		)
	}

	self.RegisterPatternHandlerRaw(
		"ClusterForceReconnect",
		PatternConfig{
			ResponseSchema: schema.Boolean(),
		},
		func(datagram structs.Datagram) any {
			time.Sleep(1 * time.Second)
			return kubernetes.ClusterForceReconnect()
		},
	)

	self.RegisterPatternHandlerRaw(
		"ClusterForceDisconnect",
		PatternConfig{
			ResponseSchema: schema.Boolean(),
		},
		func(datagram structs.Datagram) any {
			time.Sleep(1 * time.Second)
			return kubernetes.ClusterForceDisconnect()
		},
	)

	self.RegisterPatternHandlerRaw(
		"SYSTEM_CHECK",
		PatternConfig{
			ResponseSchema: schema.Generate(services.SystemCheckResponse{}),
		},
		func(datagram structs.Datagram) any {
			return services.SystemCheck()
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/restart",
		PatternConfig{},
		func(datagram structs.Datagram) any {
			go func() {
				shutdownDelay := 1 * time.Second
				self.logger.Info("😵😵😵 Received RESTART COMMAND. Initialized restart.", "delay", shutdownDelay)
				time.Sleep(shutdownDelay)
				shutdown.SendShutdownSignal(false)
			}()
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		"print-current-config",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) any {
			return self.config.AsEnvs()
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/energy-consumption",
		PatternConfig{
			ResponseSchema: schema.Generate([]structs.EnergyConsumptionResponse{}),
		},
		func(datagram structs.Datagram) any {
			return services.EnergyConsumption()
		},
	)

	self.RegisterPatternHandler(
		"install-metrics-server",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			return services.InstallMetricsServer()
		},
	)

	self.RegisterPatternHandler(
		"install-ingress-controller-traefik",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			return services.InstallIngressControllerTreafik()
		},
	)

	self.RegisterPatternHandler(
		"install-cert-manager",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			return services.InstallCertManager()
		},
	)

	{
		type Request struct {
			Email string `json:"email" validate:"required,email"`
		}

		self.RegisterPatternHandlerRaw(
			"install-cluster-issuer",
			PatternConfig{
				RequestSchema: schema.Generate(Request{}),
			},
			func(datagram structs.Datagram) any {
				data := Request{}
				_ = self.loadRequest(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return err
				}
				secrets.AddSecret(data.Email)
				result, err := services.InstallClusterIssuer(data.Email, 0)
				return NewMessageResponse(result, err)
			},
		)
	}

	self.RegisterPatternHandler(
		"install-metallb",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			return services.InstallMetalLb()
		},
	)

	self.RegisterPatternHandler(
		"install-kepler",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			return services.InstallKepler()
		},
	)

	self.RegisterPatternHandler(
		"uninstall-metrics-server",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			return services.UninstallMetricsServer()
		},
	)

	self.RegisterPatternHandler(
		"uninstall-ingress-controller-traefik",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			return services.UninstallIngressControllerTreafik()
		},
	)

	self.RegisterPatternHandler(
		"uninstall-cert-manager",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			return services.UninstallCertManager()
		},
	)

	self.RegisterPatternHandler(
		"uninstall-cluster-issuer",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			return services.UninstallClusterIssuer()
		},
	)

	self.RegisterPatternHandler(
		"uninstall-metallb",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			return services.UninstallMetalLb()
		},
	)

	self.RegisterPatternHandler(
		"uninstall-kepler",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			return services.UninstallKepler()
		},
	)

	self.RegisterPatternHandler(
		"upgrade-metrics-server",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			return services.UpgradeMetricsServer()
		},
	)

	self.RegisterPatternHandler(
		"upgrade-ingress-controller-traefik",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			return services.UpgradeIngressControllerTreafik()
		},
	)

	self.RegisterPatternHandler(
		"upgrade-cert-manager",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			return services.UpgradeCertManager()
		},
	)

	self.RegisterPatternHandler(
		"upgrade-metallb",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			return services.UpgradeMetalLb()
		},
	)

	self.RegisterPatternHandler(
		"upgrade-kepler",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			return services.UpgradeKepler()
		},
	)

	{
		type Request struct {
			Kind              string `json:"kind"`
			Name              string `json:"name"`
			Namespace         string `json:"namespace"`
			TimeOffsetMinutes int    `json:"timeOffsetMinutes"`
		}

		self.RegisterPatternHandlerRaw(
			"stats/podstat/all-for-controller",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.Generate(&[]structs.PodStats{}),
			},
			func(datagram structs.Datagram) any {
				data := Request{}
				_ = self.loadRequest(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return err
				}
				if data.TimeOffsetMinutes <= 0 {
					data.TimeOffsetMinutes = 60 * 24 // 1 day
				}
				return self.dbstats.GetPodStatsEntriesForController(data.Kind, data.Name, data.Namespace, int64(data.TimeOffsetMinutes))
			},
		)

		self.RegisterPatternHandlerRaw(
			"stats/traffic/all-for-controller",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.Generate(&[]networkmonitor.PodNetworkStats{}),
			},
			func(datagram structs.Datagram) any {
				data := Request{}
				_ = self.loadRequest(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return err
				}
				if data.TimeOffsetMinutes <= 0 {
					data.TimeOffsetMinutes = 60 * 24 // 1 day
				}
				return self.dbstats.GetTrafficStatsEntriesForController(data.Kind, data.Name, data.Namespace, int64(data.TimeOffsetMinutes))
			},
		)
	}

	self.RegisterPatternHandlerRaw(
		"stats/podstat/last-for-controller",
		PatternConfig{
			RequestSchema:  schema.Generate(kubernetes.K8sController{}),
			ResponseSchema: schema.Generate(&structs.PodStats{}),
		},
		func(datagram structs.Datagram) any {
			data := kubernetes.K8sController{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetLastPodStatsEntryForController(data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"stats/traffic/sum-for-controller",
		PatternConfig{
			RequestSchema:  schema.Generate(kubernetes.K8sController{}),
			ResponseSchema: schema.Generate(&networkmonitor.PodNetworkStats{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(kubernetes.K8sController{}),
			ResponseSchema: schema.Generate(&structs.SocketConnections{}),
		},
		func(datagram structs.Datagram) any {
			data := kubernetes.K8sController{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return self.dbstats.GetSocketConnectionsForController(data)
		},
	)

	{
		type Request struct {
			Namespace string `json:"namespace" validate:"required"`
		}

		self.RegisterPatternHandlerRaw(
			"stats/traffic/sum-for-namespace",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.Generate([]networkmonitor.PodNetworkStats{}),
			},
			func(datagram structs.Datagram) any {
				data := Request{}
				_ = self.loadRequest(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return err
				}
				return self.dbstats.GetTrafficStatsEntriesSumForNamespace(data.Namespace)
			},
		)
	}

	{

		type Request struct {
			WorkspaceName     string `json:"workspaceName"`
			TimeOffsetMinutes int    `json:"timeOffsetMinutes"`
		}

		self.RegisterPatternHandler(
			"stats/workspace-cpu-utilization",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.Generate([]GenericChartEntry{}),
			},
			func(datagram structs.Datagram) (interface{}, error) {
				data := Request{}
				structs.MarshalUnmarshal(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return nil, err
				}
				resources, err := self.apiService.GetWorkspaceControllers(data.WorkspaceName, nil, nil, []string{})
				if err != nil {
					return nil, err
				}
				return self.dbstats.GetWorkspaceStatsCpuUtilization(data.TimeOffsetMinutes, resources)
			},
		)

		self.RegisterPatternHandler(
			"stats/workspace-memory-utilization",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.Generate([]GenericChartEntry{}),
			},
			func(datagram structs.Datagram) (interface{}, error) {
				data := Request{}
				structs.MarshalUnmarshal(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return nil, err
				}
				resources, err := self.apiService.GetWorkspaceControllers(data.WorkspaceName, nil, nil, []string{})
				if err != nil {
					return nil, err
				}
				return self.dbstats.GetWorkspaceStatsMemoryUtilization(data.TimeOffsetMinutes, resources)
			},
		)

		self.RegisterPatternHandler(
			"stats/workspace-traffic-utilization",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.Generate([]GenericChartEntry{}),
			},
			func(datagram structs.Datagram) (interface{}, error) {
				data := Request{}
				structs.MarshalUnmarshal(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return nil, err
				}
				resources, err := self.apiService.GetWorkspaceControllers(data.WorkspaceName, nil, nil, []string{})
				if err != nil {
					return nil, err
				}
				return self.dbstats.GetWorkspaceStatsTrafficUtilization(data.TimeOffsetMinutes, resources)
			},
		)
	}

	{
		self.RegisterPatternHandlerRaw(
			"metrics/deployment/average-utilization",
			PatternConfig{
				RequestSchema:  schema.Generate(kubernetes.K8sController{}),
				ResponseSchema: schema.Generate(&kubernetes.Metrics{}),
			},
			func(datagram structs.Datagram) any {
				data := kubernetes.K8sController{}
				data.Kind = "Deployment"
				_ = self.loadRequest(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return err
				}
				return kubernetes.GetAverageUtilizationForDeployment(data)
			},
		)
	}

	{
		type Request struct {
			Folder dtos.PersistentFileRequestDto `json:"folder" validate:"required"`
		}

		self.RegisterPatternHandlerRaw(
			"files/list",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.Generate([]dtos.PersistentFileDto{}),
			},
			func(datagram structs.Datagram) any {
				data := Request{}
				_ = self.loadRequest(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return err
				}
				return services.List(data.Folder)
			},
		)
	}

	{
		type Request struct {
			Folder dtos.PersistentFileRequestDto `json:"folder" validate:"required"`
		}

		self.RegisterPatternHandlerRaw(
			"files/create-folder",
			PatternConfig{
				RequestSchema: schema.Generate(Request{}),
			},
			func(datagram structs.Datagram) any {
				data := Request{}
				_ = self.loadRequest(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return err
				}
				return services.CreateFolder(data.Folder)
			},
		)
	}

	{
		type Request struct {
			File    dtos.PersistentFileRequestDto `json:"file" validate:"required"`
			NewName string                        `json:"newName" validate:"required"`
		}

		self.RegisterPatternHandlerRaw(
			"files/rename",
			PatternConfig{
				RequestSchema: schema.Generate(Request{}),
			},
			func(datagram structs.Datagram) any {
				data := Request{}
				_ = self.loadRequest(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return err
				}
				return services.Rename(data.File, data.NewName)
			},
		)
	}

	{
		type Request struct {
			File dtos.PersistentFileRequestDto `json:"file" validate:"required"`
			Uid  string                        `json:"uid" validate:"required"`
			Gid  string                        `json:"gid" validate:"required"`
		}

		self.RegisterPatternHandlerRaw(
			"files/chown",
			PatternConfig{
				RequestSchema: schema.Generate(Request{}),
			},
			func(datagram structs.Datagram) any {
				data := Request{}
				_ = self.loadRequest(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return err
				}
				return services.Chown(data.File, data.Uid, data.Gid)
			},
		)
	}

	{
		type Request struct {
			File dtos.PersistentFileRequestDto `json:"file" validate:"required"`
			Mode string                        `json:"mode" validate:"required"`
		}

		self.RegisterPatternHandlerRaw(
			"files/chmod",
			PatternConfig{
				RequestSchema: schema.Generate(Request{}),
			},
			func(datagram structs.Datagram) any {
				data := Request{}
				_ = self.loadRequest(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return err
				}
				return services.Chmod(data.File, data.Mode)
			},
		)
	}

	{
		type Request struct {
			File dtos.PersistentFileRequestDto `json:"file" validate:"required"`
		}

		self.RegisterPatternHandlerRaw(
			"files/delete",
			PatternConfig{
				RequestSchema: schema.Generate(Request{}),
			},
			func(datagram structs.Datagram) any {
				data := Request{}
				_ = self.loadRequest(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return err
				}
				return services.Delete(data.File)
			},
		)
	}

	{
		type Request struct {
			File   dtos.PersistentFileRequestDto `json:"file" validate:"required"`
			PostTo string                        `json:"postTo" validate:"required"`
		}

		self.RegisterPatternHandlerRaw(
			"files/download",
			PatternConfig{
				RequestSchema: schema.Generate(Request{}),
			},
			func(datagram structs.Datagram) any {
				data := Request{}
				_ = self.loadRequest(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return err
				}
				return services.Download(data.File, data.PostTo)
			},
		)
	}

	self.RegisterPatternHandlerRaw(
		"files/info",
		PatternConfig{
			RequestSchema:  schema.Generate(dtos.PersistentFileRequestDto{}),
			ResponseSchema: schema.Generate(dtos.PersistentFileRequestDto{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(services.ClusterHelmRequest{}),
			ResponseSchema: schema.Generate(&structs.Job{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ClusterHelmRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.InstallHelmChart(self.eventsClient, data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/uninstall-helm-chart",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ClusterHelmUninstallRequest{}),
			ResponseSchema: schema.Generate(&structs.Job{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ClusterHelmUninstallRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.DeleteHelmChart(self.eventsClient, data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/tcp-udp-configuration",
		PatternConfig{
			ResponseSchema: schema.Generate(dtos.TcpUdpClusterConfigurationDto{}),
		},
		func(datagram structs.Datagram) any {
			return services.TcpUdpClusterConfiguration()
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/backup",
		PatternConfig{
			ResponseSchema: schema.Generate(kubernetes.NamespaceBackupResponse{}),
		},
		func(datagram structs.Datagram) any {
			result, err := kubernetes.BackupNamespace("")
			if err != nil {
				return err.Error()
			}
			return result
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/read-configmap",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ClusterGetConfigMap{}),
			ResponseSchema: schema.Generate(kubernetes.K8sWorkloadResult{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema: schema.Generate(services.ClusterWriteConfigMap{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(services.ClusterListWorkloads{}),
			ResponseSchema: schema.Generate(kubernetes.K8sWorkloadResult{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(services.ClusterGetDeployment{}),
			ResponseSchema: schema.Generate(kubernetes.K8sWorkloadResult{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ClusterGetDeployment{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.GetDeploymentResult(data.Namespace, data.Name)
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/list-deployments",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ClusterListWorkloads{}),
			ResponseSchema: schema.Generate(kubernetes.K8sWorkloadResult{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(services.ClusterGetPersistentVolume{}),
			ResponseSchema: schema.Generate(&v1.PersistentVolumeClaim{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ClusterGetPersistentVolume{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(kubernetes.GetPersistentVolumeClaim(data.Namespace, data.Name))
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/list-persistent-volume-claims",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ClusterListWorkloads{}),
			ResponseSchema: schema.Generate(kubernetes.K8sWorkloadResult{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema: schema.Generate(services.ClusterUpdateLocalTlsSecret{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(services.NamespaceCreateRequest{}),
			ResponseSchema: schema.Generate(&structs.Job{}),
		},
		func(datagram structs.Datagram) any {
			data := services.NamespaceCreateRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Project.AddSecretsToRedaction()
			return services.CreateNamespace(self.eventsClient, data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"namespace/delete",
		PatternConfig{
			RequestSchema:  schema.Generate(services.NamespaceDeleteRequest{}),
			ResponseSchema: schema.Generate(&structs.Job{}),
		},
		func(datagram structs.Datagram) any {
			data := services.NamespaceDeleteRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.DeleteNamespace(self.eventsClient, data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"namespace/shutdown",
		PatternConfig{
			RequestSchema:  schema.Generate(services.NamespaceShutdownRequest{}),
			ResponseSchema: schema.Generate(&structs.Job{}),
		},
		func(datagram structs.Datagram) any {
			data := services.NamespaceShutdownRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			return services.ShutdownNamespace(self.eventsClient, data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"namespace/pod-ids",
		PatternConfig{
			RequestSchema:  schema.Generate(services.NamespacePodIdsRequest{}),
			ResponseSchema: schema.Generate([]string{}),
		},
		func(datagram structs.Datagram) any {
			data := services.NamespacePodIdsRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.PodIdsFor(data.Namespace, nil)
		},
	)

	self.RegisterPatternHandlerRaw(
		"namespace/validate-cluster-pods",
		PatternConfig{
			RequestSchema:  schema.Generate(services.NamespaceValidateClusterPodsRequest{}),
			ResponseSchema: schema.Generate(dtos.ValidateClusterPodsDto{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema: schema.Generate(services.NamespaceValidatePortsRequest{}),
		},
		func(datagram structs.Datagram) any {
			data := services.NamespaceValidatePortsRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			services.ValidateClusterPorts(data)
			return nil
		},
	)

	self.RegisterPatternHandlerRaw(
		"namespace/list-all",
		PatternConfig{
			ResponseSchema: schema.Generate([]string{}),
		},
		func(datagram structs.Datagram) any {
			return services.ListAllNamespaces()
		},
	)

	self.RegisterPatternHandlerRaw(
		"namespace/gather-all-resources",
		PatternConfig{
			RequestSchema:  schema.Generate(services.NamespaceGatherAllResourcesRequest{}),
			ResponseSchema: schema.Generate(dtos.NamespaceResourcesDto{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(services.NamespaceBackupRequest{}),
			ResponseSchema: schema.Generate(kubernetes.NamespaceBackupResponse{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(services.NamespaceRestoreRequest{}),
			ResponseSchema: schema.Generate(kubernetes.NamespaceRestoreResponse{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(services.NamespaceResourceYamlRequest{}),
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) any {
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

	{
		type Request struct {
			Nodes []string `json:"nodes" validate:"required"`
		}
		self.RegisterPatternHandlerRaw(
			"cluster/machine-stats",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.Generate([]structs.MachineStats{}),
			},
			func(datagram structs.Datagram) any {
				data := Request{}
				_ = self.loadRequest(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return err
				}
				return NewMessageResponse(self.dbstats.GetMachineStatsForNodes(data.Nodes), nil)
			},
		)
	}
	self.RegisterPatternHandlerRaw(
		"cluster/helm-repo-add",
		PatternConfig{
			RequestSchema:  schema.Generate(helm.HelmRepoAddRequest{}),
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(helm.HelmRepoPatchRequest{}),
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			ResponseSchema: schema.Generate([]helm.HelmEntryStatus{}),
		},
		func(datagram structs.Datagram) (any, error) {
			return helm.HelmRepoUpdate()
		},
	)

	self.RegisterPatternHandler(
		"cluster/helm-repo-list",
		PatternConfig{
			ResponseSchema: schema.Generate([]*helm.HelmEntryWithoutPassword{}),
		},
		func(datagram structs.Datagram) (any, error) {
			return helm.HelmRepoList()
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/helm-chart-remove",
		PatternConfig{
			RequestSchema:  schema.Generate(helm.HelmRepoRemoveRequest{}),
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(helm.HelmChartSearchRequest{}),
			ResponseSchema: schema.Generate([]helm.HelmChartInfo{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(helm.HelmChartInstallUpgradeRequest{}),
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(helm.HelmChartShowRequest{}),
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(helm.HelmChartVersionRequest{}),
			ResponseSchema: schema.Generate([]helm.HelmChartInfo{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(helm.HelmChartInstallUpgradeRequest{}),
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(helm.HelmReleaseUninstallRequest{}),
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(helm.HelmReleaseListRequest{}),
			ResponseSchema: schema.Generate([]*release.Release{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(helm.HelmReleaseStatusRequest{}),
			ResponseSchema: schema.Generate(&helm.HelmReleaseStatusInfo{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(helm.HelmReleaseHistoryRequest{}),
			ResponseSchema: schema.Generate([]*release.Release{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(helm.HelmReleaseRollbackRequest{}),
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(helm.HelmReleaseGetRequest{}),
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(helm.HelmReleaseGetWorkloadsRequest{}),
			ResponseSchema: schema.Generate([]unstructured.Unstructured{}),
		},
		func(datagram structs.Datagram) any {
			data := helm.HelmReleaseGetWorkloadsRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(helm.HelmReleaseGetWorkloads(self.valkeyClient, data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/create",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ServiceUpdateRequest{}),
			ResponseSchema: schema.Generate(&structs.Job{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ServiceUpdateRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			data.Project.AddSecretsToRedaction()
			return services.UpdateService(self.eventsClient, data, self.config)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/delete",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ServiceDeleteRequest{}),
			ResponseSchema: schema.Generate(&structs.Job{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ServiceDeleteRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			data.Project.AddSecretsToRedaction()
			return services.DeleteService(self.eventsClient, data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/pod-ids",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ServiceGetPodIdsRequest{}),
			ResponseSchema: schema.Generate([]string{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ServiceGetPodIdsRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.PodIdsFor(data.Namespace, &data.ServiceId)
		},
	)

	self.RegisterPatternHandlerRaw(
		"SERVICE_POD_EXISTS",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ServicePodExistsRequest{}),
			ResponseSchema: schema.Generate(kubernetes.ServicePodExistsResult{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ServicePodExistsRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.PodExists(data.K8sNamespace, data.K8sPod)
		},
	)

	self.RegisterPatternHandlerRaw(
		"SERVICE_PODS",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ServicePodsRequest{}),
			ResponseSchema: schema.Generate([]v1.Pod{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ServicePodsRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.ServicePodStatus(self.eventsClient, data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/log",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ServiceGetLogRequest{}),
			ResponseSchema: schema.Generate(kubernetes.ServiceGetLogResult{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ServiceGetLogRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.GetLog(data.Namespace, data.PodId, data.Timestamp)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/log-error",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ServiceGetLogRequest{}),
			ResponseSchema: schema.Generate(kubernetes.ServiceGetLogErrorResult{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ServiceGetLogRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.GetLogError(data.Namespace, data.PodId)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/resource-status",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ServiceResourceStatusRequest{}),
			ResponseSchema: schema.Generate(&v1.Pod{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ServiceResourceStatusRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return kubernetes.PodStatus(data.Namespace, data.Name, data.StatusOnly)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/restart",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ServiceRestartRequest{}),
			ResponseSchema: schema.Generate(&structs.Job{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ServiceRestartRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			return services.Restart(self.eventsClient, data, self.config)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/stop",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ServiceStopRequest{}),
			ResponseSchema: schema.Generate(&structs.Job{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ServiceStopRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			return services.StopService(self.eventsClient, data, self.config)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/start",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ServiceStartRequest{}),
			ResponseSchema: schema.Generate(&structs.Job{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ServiceStartRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			return services.StartService(self.eventsClient, data, self.config)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/update-service",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ServiceUpdateRequest{}),
			ResponseSchema: schema.Generate(&structs.Job{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ServiceUpdateRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			data.Project.AddSecretsToRedaction()
			data.Service.AddSecretsToRedaction()
			return services.UpdateService(self.eventsClient, data, self.config)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/trigger-job",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ServiceTriggerJobRequest{}),
			ResponseSchema: schema.Generate(&structs.Job{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ServiceTriggerJobRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.TriggerJobService(self.eventsClient, data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/status",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ServiceStatusRequest{}),
			ResponseSchema: schema.Generate(services.ServiceStatusResponse{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ServiceStatusRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.StatusServiceDebounced(self.valkeyClient, data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/log-stream",
		PatternConfig{
			RequestSchema:  schema.Generate(services.ServiceLogStreamRequest{}),
			ResponseSchema: schema.Generate(services.ServiceLogStreamResult{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema: schema.Generate(xterm.PodCmdConnectionRequest{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema: schema.Generate(xterm.PodCmdConnectionRequest{}),
		},
		func(datagram structs.Datagram) any {
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
		"cluster/component-log-stream-connection-request",
		PatternConfig{
			RequestSchema: schema.Generate(xterm.ComponentLogConnectionRequest{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema: schema.Generate(xterm.PodEventConnectionRequest{}),
		},
		func(datagram structs.Datagram) any {
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
		"service/cluster-tool-stream-connection-request",
		PatternConfig{
			RequestSchema: schema.Generate(xterm.ClusterToolConnectionRequest{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			ResponseSchema: schema.Generate([]utils.SyncResourceEntry{}),
		},
		func(datagram structs.Datagram) (any, error) {
			return kubernetes.GetAvailableResources()
		},
	)

	self.RegisterPatternHandler(
		"get/workload-list",
		PatternConfig{
			RequestSchema:  schema.Generate(utils.SyncResourceEntry{}),
			ResponseSchema: schema.Generate(unstructured.UnstructuredList{}),
		},
		func(datagram structs.Datagram) (any, error) {
			data := utils.SyncResourceEntry{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.GetUnstructuredResourceListFromStore(data.Group, data.Kind, data.Version, data.Name, data.Namespace)
		},
	)

	self.RegisterPatternHandler(
		"get/namespace-workload-list",
		PatternConfig{
			RequestSchema:  schema.Generate(kubernetes.GetUnstructuredNamespaceResourceListRequest{}),
			ResponseSchema: schema.Generate(&[]unstructured.Unstructured{}),
		},
		func(datagram structs.Datagram) (any, error) {
			data := kubernetes.GetUnstructuredNamespaceResourceListRequest{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.GetUnstructuredNamespaceResourceList(data.Namespace, data.Whitelist, data.Blacklist)
		},
	)

	self.RegisterPatternHandler(
		"get/labeled-workload-list",
		PatternConfig{
			RequestSchema:  schema.Generate(kubernetes.GetUnstructuredLabeledResourceListRequest{}),
			ResponseSchema: schema.Generate(&unstructured.UnstructuredList{}),
		},
		func(datagram structs.Datagram) (any, error) {
			data := kubernetes.GetUnstructuredLabeledResourceListRequest{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.GetUnstructuredLabeledResourceList(data.Label, data.Whitelist, data.Blacklist)
		},
	)

	self.RegisterPatternHandler(
		"describe/workload",
		PatternConfig{
			RequestSchema:  schema.Generate(utils.SyncResourceItem{}),
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			data := utils.SyncResourceItem{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.DescribeUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.ResourceName)
		},
	)

	self.RegisterPatternHandler(
		"create/new-workload",
		PatternConfig{
			RequestSchema:  schema.Generate(utils.SyncResourceData{}),
			ResponseSchema: schema.Generate(&unstructured.Unstructured{}),
		},
		func(datagram structs.Datagram) (any, error) {
			data := utils.SyncResourceData{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.CreateUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.YamlData)
		},
	)

	self.RegisterPatternHandler(
		"get/workload",
		PatternConfig{
			RequestSchema:  schema.Generate(utils.SyncResourceItem{}),
			ResponseSchema: schema.Generate(&unstructured.Unstructured{}),
		},
		func(datagram structs.Datagram) (any, error) {
			data := utils.SyncResourceItem{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.GetUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.ResourceName)
		},
	)

	self.RegisterPatternHandler(
		"get/workload-status",
		PatternConfig{
			RequestSchema:  schema.Generate(kubernetes.GetWorkloadStatusRequest{}),
			ResponseSchema: schema.Generate([]kubernetes.WorkloadStatusDto{}),
		},
		func(datagram structs.Datagram) (any, error) {
			data := kubernetes.GetWorkloadStatusRequest{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.GetWorkloadStatus(data)
		},
	)

	self.RegisterPatternHandler(
		"get/workload-example",
		PatternConfig{
			RequestSchema:  schema.Generate(utils.SyncResourceItem{}),
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) (any, error) {
			data := utils.SyncResourceItem{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.GetResourceTemplateYaml(data.Group, data.Version, data.Name, data.Kind, data.Namespace, data.ResourceName), nil
		},
	)

	self.RegisterPatternHandler(
		"update/workload",
		PatternConfig{
			RequestSchema:  schema.Generate(utils.SyncResourceData{}),
			ResponseSchema: schema.Generate(&unstructured.Unstructured{}),
		},
		func(datagram structs.Datagram) (any, error) {
			data := utils.SyncResourceData{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.UpdateUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.YamlData)
		},
	)

	self.RegisterPatternHandler(
		"delete/workload",
		PatternConfig{
			RequestSchema: schema.Generate(utils.SyncResourceItem{}),
		},
		func(datagram structs.Datagram) (any, error) {
			data := utils.SyncResourceItem{}
			_ = self.loadRequest(&datagram, &data)
			return nil, kubernetes.DeleteUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.ResourceName)
		},
	)

	self.RegisterPatternHandler(
		"trigger/workload",
		PatternConfig{
			RequestSchema: schema.Generate(utils.SyncResourceItem{}),
		},
		func(datagram structs.Datagram) (any, error) {
			data := utils.SyncResourceItem{}
			_ = self.loadRequest(&datagram, &data)
			return kubernetes.TriggerUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.ResourceName)
		},
	)

	self.RegisterPatternHandler(
		"get/workspaces",
		PatternConfig{
			ResponseSchema: schema.Generate([]GetWorkspaceResult{}),
		},
		func(datagram structs.Datagram) (any, error) {
			return self.apiService.GetAllWorkspaces()
		},
	)

	{
		type Request struct {
			Name        string                                 `json:"name" validate:"required"`
			DisplayName string                                 `json:"displayName" validate:"required"`
			Resources   []v1alpha1.WorkspaceResourceIdentifier `json:"resources" validate:"required"`
		}

		self.RegisterPatternHandler(
			"create/workspace",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.String(),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
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
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		self.RegisterPatternHandler(
			"get/workspace",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.Generate(&GetWorkspaceResult{}),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
				err := self.loadRequest(&datagram, &data)
				if err != nil {
					return nil, err
				}
				return self.apiService.GetWorkspace(data.Name)
			},
		)
	}

	{
		type Request struct {
			Name        string                                 `json:"name" validate:"required"`
			DisplayName string                                 `json:"displayName" validate:"required"`
			Resources   []v1alpha1.WorkspaceResourceIdentifier `json:"resources" validate:"required"`
		}

		self.RegisterPatternHandler(
			"update/workspace",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.String(),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
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
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		self.RegisterPatternHandler(
			"delete/workspace",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.String(),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
				err := self.loadRequest(&datagram, &data)
				if err != nil {
					return nil, err
				}
				return self.apiService.DeleteWorkspace(data.Name)
			},
		)
	}

	{
		type Request struct {
			Email *string `json:"email"`
		}

		self.RegisterPatternHandler(
			"get/users",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.Generate([]v1alpha1.User{}),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
				err := self.loadRequest(&datagram, &data)
				if err != nil {
					return nil, err
				}
				return self.apiService.GetAllUsers(data.Email)
			},
		)
	}

	{
		type Request struct {
			Name      string `json:"name"`
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
			Email     string `json:"email"`
		}

		self.RegisterPatternHandler(
			"create/user",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.String(),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
				err := self.loadRequest(&datagram, &data)
				if err != nil {
					return nil, err
				}
				return self.apiService.CreateUser(data.Name, v1alpha1.NewUserSpec(
					data.FirstName,
					data.LastName,
					data.Email,
				))
			},
		)
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		self.RegisterPatternHandler(
			"get/user",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.Generate(&v1alpha1.User{}),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
				err := self.loadRequest(&datagram, &data)
				if err != nil {
					return nil, err
				}
				return self.apiService.GetUser(data.Name)
			},
		)
	}

	{
		type Request struct {
			Name      string `json:"name" validate:"required"`
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
			Email     string `json:"email"`
		}

		self.RegisterPatternHandler(
			"update/user",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.String(),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
				err := self.loadRequest(&datagram, &data)
				if err != nil {
					return nil, err
				}
				return self.apiService.UpdateUser(data.Name, v1alpha1.NewUserSpec(
					data.FirstName,
					data.LastName,
					data.Email,
				))
			},
		)
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		self.RegisterPatternHandler(
			"delete/user",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.String(),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
				err := self.loadRequest(&datagram, &data)
				if err != nil {
					return nil, err
				}
				return self.apiService.DeleteUser(data.Name)
			},
		)
	}

	self.RegisterPatternHandler(
		"get/teams",
		PatternConfig{
			ResponseSchema: schema.Generate([]v1alpha1.Team{}),
		},
		func(datagram structs.Datagram) (any, error) {
			return self.apiService.GetAllTeams()
		},
	)

	{
		type Request struct {
			Name        string   `json:"name" validate:"required"`
			DisplayName string   `json:"displayName" validate:"required"`
			Users       []string `json:"users" validate:"required"`
		}

		self.RegisterPatternHandler(
			"create/team",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.String(),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
				err := self.loadRequest(&datagram, &data)
				if err != nil {
					return nil, err
				}
				return self.apiService.CreateTeam(data.Name, v1alpha1.NewTeamSpec(data.DisplayName, data.Users))
			},
		)
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		self.RegisterPatternHandler(
			"get/team",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.Generate(&v1alpha1.Team{}),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
				err := self.loadRequest(&datagram, &data)
				if err != nil {
					return nil, err
				}
				return self.apiService.GetTeam(data.Name)
			},
		)
	}

	{
		type Request struct {
			Name        string   `json:"name" validate:"required"`
			DisplayName string   `json:"displayName" validate:"required"`
			Users       []string `json:"users" validate:"required"`
		}

		self.RegisterPatternHandler(
			"update/team",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.String(),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
				err := self.loadRequest(&datagram, &data)
				if err != nil {
					return nil, err
				}
				return self.apiService.UpdateTeam(data.Name, v1alpha1.NewTeamSpec(data.DisplayName, data.Users))
			},
		)
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		self.RegisterPatternHandler(
			"delete/team",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.String(),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
				err := self.loadRequest(&datagram, &data)
				if err != nil {
					return nil, err
				}
				return self.apiService.DeleteTeam(data.Name)
			},
		)
	}

	{

		type Request struct {
			TargetType *string `json:"targetType"`
			TargetName *string `json:"targetName"`
		}
		self.RegisterPatternHandler(
			"get/grants",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.Generate([]v1alpha1.Grant{}),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
				err := self.loadRequest(&datagram, &data)
				if err != nil {
					return nil, err
				}
				return self.apiService.GetAllGrants(data.TargetType, data.TargetName)
			},
		)
	}

	{
		type Request struct {
			Name       string `json:"name" validate:"required"`
			Grantee    string `json:"grantee" validate:"required"`
			TargetType string `json:"targetType" validate:"required"`
			TargetName string `json:"targetName" validate:"required"`
			Role       string `json:"role" validate:"required"`
		}

		self.RegisterPatternHandler(
			"create/grant",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.String(),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
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
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		self.RegisterPatternHandler(
			"get/grant",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.Generate(&v1alpha1.Grant{}),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
				err := self.loadRequest(&datagram, &data)
				if err != nil {
					return nil, err
				}
				return self.apiService.GetGrant(data.Name)
			},
		)
	}

	{
		type Request struct {
			Name       string `json:"name" validate:"required"`
			Grantee    string `json:"grantee" validate:"required"`
			TargetType string `json:"targetType" validate:"required"`
			TargetName string `json:"targetName" validate:"required"`
			Role       string `json:"role" validate:"required"`
		}

		self.RegisterPatternHandler(
			"update/grant",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.String(),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
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
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		self.RegisterPatternHandler(
			"delete/grant",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.String(),
			},
			func(datagram structs.Datagram) (any, error) {
				data := Request{}
				err := self.loadRequest(&datagram, &data)
				if err != nil {
					return nil, err
				}
				return self.apiService.DeleteGrant(data.Name)
			},
		)
	}

	{
		type Request struct {
			WorkspaceName      string                     `json:"workspaceName"`
			Whitelist          []*utils.SyncResourceEntry `json:"whitelist"`
			Blacklist          []*utils.SyncResourceEntry `json:"blacklist"`
			NamespaceWhitelist []string                   `json:"namespaceWhitelist"`
		}

		self.RegisterPatternHandler(
			"get/workspace-workloads",
			PatternConfig{
				RequestSchema:  schema.Generate(Request{}),
				ResponseSchema: schema.Generate([]unstructured.Unstructured{}),
			},
			func(datagram structs.Datagram) (interface{}, error) {
				data := Request{}
				structs.MarshalUnmarshal(&datagram, &data)
				return self.apiService.GetWorkspaceResources(data.WorkspaceName, data.Whitelist, data.Blacklist, data.NamespaceWhitelist)
			},
		)
	}

	self.RegisterPatternHandlerRaw(
		"storage/create-volume",
		PatternConfig{
			RequestSchema:  schema.Generate(services.NfsVolumeRequest{}),
			ResponseSchema: schema.Generate(structs.DefaultResponse{}),
		},
		func(datagram structs.Datagram) any {
			data := services.NfsVolumeRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.CreateMogeniusNfsVolume(self.eventsClient, data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"storage/delete-volume",
		PatternConfig{
			RequestSchema:  schema.Generate(services.NfsVolumeRequest{}),
			ResponseSchema: schema.Generate(structs.DefaultResponse{}),
		},
		func(datagram structs.Datagram) any {
			data := services.NfsVolumeRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.DeleteMogeniusNfsVolume(self.eventsClient, data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"storage/stats",
		PatternConfig{
			RequestSchema:  schema.Generate(services.NfsVolumeStatsRequest{}),
			ResponseSchema: schema.Generate(services.NfsVolumeStatsResponse{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(services.NfsNamespaceStatsRequest{}),
			ResponseSchema: schema.Generate([]services.NfsVolumeStatsResponse{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(services.NfsStatusRequest{}),
			ResponseSchema: schema.Generate(services.NfsStatusResponse{}),
		},
		func(datagram structs.Datagram) any {
			data := services.NfsStatusRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return services.StatusMogeniusNfs(data)
		},
	)

	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// External Secrets
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -

	self.RegisterPatternHandlerRaw(
		"external-secret-store/create",
		PatternConfig{
			RequestSchema:  schema.Generate(controllers.CreateSecretsStoreRequest{}),
			ResponseSchema: schema.Generate(controllers.CreateSecretsStoreResponse{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(controllers.ListSecretStoresRequest{}),
			ResponseSchema: schema.Generate([]kubernetes.SecretStore{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(controllers.ListSecretsRequest{}),
			ResponseSchema: schema.Generate([]string{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(controllers.DeleteSecretsStoreRequest{}),
			ResponseSchema: schema.Generate(controllers.DeleteSecretsStoreResponse{}),
		},
		func(datagram structs.Datagram) any {
			data := controllers.DeleteSecretsStoreRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return controllers.DeleteExternalSecretsStore(data)
		},
	)

	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// Labeled Network Policies
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	self.RegisterPatternHandlerRaw(
		"attach/labeled_network_policy",
		PatternConfig{
			RequestSchema:  schema.Generate(controllers.AttachLabeledNetworkPolicyRequest{}),
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(controllers.DetachLabeledNetworkPolicyRequest{}),
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			ResponseSchema: schema.Generate([]dtos.K8sLabeledNetworkPolicyDto{}),
		},
		func(datagram structs.Datagram) (any, error) {
			return controllers.ListLabeledNetworkPolicyPorts()
		},
	)

	self.RegisterPatternHandlerRaw(
		"list/conflicting_network_policies",
		PatternConfig{
			RequestSchema:  schema.Generate(controllers.ListConflictingNetworkPoliciesRequest{}),
			ResponseSchema: schema.Generate([]controllers.K8sConflictingNetworkPolicyDto{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(controllers.RemoveConflictingNetworkPoliciesRequest{}),
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(controllers.ListControllerLabeledNetworkPoliciesRequest{}),
			ResponseSchema: schema.Generate(controllers.ListControllerLabeledNetworkPoliciesResponse{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema: schema.Generate([]kubernetes.NetworkPolicy{}),
		},
		func(datagram structs.Datagram) (any, error) {
			data := []kubernetes.NetworkPolicy{}
			_ = self.loadRequest(&datagram, &data)
			return nil, controllers.UpdateNetworkPolicyTemplate(data)
		},
	)

	self.RegisterPatternHandler(
		"list/all_network_policies",
		PatternConfig{
			ResponseSchema: schema.Generate([]controllers.ListNetworkPolicyNamespace{}),
		},
		func(datagram structs.Datagram) (any, error) {
			return controllers.ListAllNetworkPolicies(self.valkeyClient)
		},
	)

	self.RegisterPatternHandlerRaw(
		"list/namespace_network_policies",
		PatternConfig{
			RequestSchema:  schema.Generate(controllers.ListNamespaceLabeledNetworkPoliciesRequest{}),
			ResponseSchema: schema.Generate([]controllers.ListNetworkPolicyNamespace{}),
		},
		func(datagram structs.Datagram) any {
			data := controllers.ListNamespaceLabeledNetworkPoliciesRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(controllers.ListNamespaceNetworkPolicies(self.valkeyClient, data))
		},
	)

	self.RegisterPatternHandlerRaw(
		"enforce/network_policy_manager",
		PatternConfig{
			RequestSchema: schema.Generate(controllers.EnforceNetworkPolicyManagerRequest{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema: schema.Generate(controllers.DisableNetworkPolicyManagerRequest{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema: schema.Generate(controllers.RemoveUnmanagedNetworkPoliciesRequest{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema:  schema.Generate(controllers.ListNamespaceLabeledNetworkPoliciesRequest{}),
			ResponseSchema: schema.Generate([]controllers.ListManagedAndUnmanagedNetworkPolicyNamespace{}),
		},
		func(datagram structs.Datagram) any {
			data := controllers.ListNamespaceLabeledNetworkPoliciesRequest{}
			_ = self.loadRequest(&datagram, &data)
			if err := utils.ValidateJSON(data); err != nil {
				return err
			}
			return NewMessageResponse(controllers.ListManagedAndUnmanagedNamespaceNetworkPolicies(self.valkeyClient, data))
		},
	)

	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// Cronjobs
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	{
		type Request struct {
			ProjectId      string `json:"projectId" validate:"required"`
			NamespaceName  string `json:"namespaceName" validate:"required"`
			ControllerName string `json:"controllerName" validate:"required"`
		}

		self.RegisterPatternHandlerRaw(
			"list/cronjob-jobs",
			PatternConfig{
				RequestSchema: schema.Generate(Request{}),
			},
			func(datagram structs.Datagram) any {
				data := Request{}
				_ = self.loadRequest(&datagram, &data)
				if err := utils.ValidateJSON(data); err != nil {
					return err
				}
				return kubernetes.ListCronjobJobs(data.ControllerName, data.NamespaceName, data.ProjectId)
			},
		)
	}

	self.RegisterPatternHandlerRaw(
		"live-stream/nodes-traffic",
		PatternConfig{
			RequestSchema: schema.Generate(xterm.WsConnectionRequest{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema: schema.Generate(xterm.WsConnectionRequest{}),
		},
		func(datagram structs.Datagram) any {
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
		PatternConfig{
			RequestSchema: schema.Generate(xterm.WsConnectionRequest{}),
		},
		func(datagram structs.Datagram) any {
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
	self.startMessageHandler()
}

func (self *socketApi) startMessageHandler() {
	var preparedFileName *string
	var preparedFileRequest *services.FilesUploadRequest
	var openFile *os.File

	maxGoroutines := 100
	semaphoreChan := make(chan struct{}, maxGoroutines)
	var wg sync.WaitGroup

	go func() {
		for !self.jobClient.IsTerminated() {
			_, message, err := self.jobClient.ReadMessage()
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
				preparedFileName = utils.Pointer(fmt.Sprintf("/tmp/%s.zip", utils.NanoId()))
				openFile, err = os.OpenFile(*preparedFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					self.logger.Error("Cannot open uploadfile", "filename", *preparedFileName, "error", err)
				}
				continue
			}
			if strings.HasPrefix(rawDataStr, "######END_UPLOAD######;") {
				openFile.Close()
				if preparedFileName != nil && preparedFileRequest != nil {
					err = services.Uploaded(*preparedFileName, *preparedFileRequest)
					if err != nil {
						self.logger.Error("Error uploading file", "error", err)
					}
				}
				os.Remove(*preparedFileName)

				var ack = structs.CreateDatagramAck("ack:files/upload:end", preparedFileRequest.Id)
				self.JobServerSendData(self.jobClient, ack)

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

			datagram.DisplayReceiveSummary(self.logger)

			// TODO: refactor! @bene
			if datagram.Pattern == "files/upload" {
				preparedFileRequest = self.executeBinaryRequestUpload(datagram)

				var ack = structs.CreateDatagramAck("ack:files/upload:datagram", datagram.Id)
				self.JobServerSendData(self.jobClient, ack)
				continue
			}

			if self.patternHandlerExists(datagram.Pattern) {
				semaphoreChan <- struct{}{}

				wg.Add(1)
				go func() {
					defer func() {
						<-semaphoreChan
						wg.Done()
					}()

					if datagram.Zlib {
						decompressedData, err := utils.TryZlibDecompress(datagram.Payload)
						if err != nil {
							self.logger.Error("failed to decompress payload", "error", err)
							return
						}
						datagram.Payload = decompressedData
					}

					responsePayload := self.ExecuteCommandRequest(datagram)

					compressedData, err := utils.TryZlibCompress(responsePayload)
					if err != nil {
						self.logger.Error("failed to compress response payload", "error", err)
					} else {
						responsePayload = compressedData
					}

					result := structs.Datagram{
						Id:        datagram.Id,
						Pattern:   datagram.Pattern,
						Payload:   responsePayload,
						CreatedAt: datagram.CreatedAt,
						Zlib:      err == nil,
					}
					self.JobServerSendData(self.jobClient, result)
				}()
			}
		}
		self.logger.Debug("api messagehandler finished as the websocket client was terminated")
	}()
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
			element.DisplaySentSummary(self.logger, i+1, len(jobDataQueue))
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
	mogeniusPlatform, doesExist := helmData.Entries["mogenius-operator"]
	if !doesExist {
		self.logger.Error("HelmIndex does not contain the field 'mogenius-operator'. Check the HelmIndex for errors.", "HelmIndex", utils.HELM_INDEX)
		return
	}
	if len(mogeniusPlatform) <= 0 {
		self.logger.Error("Field 'mogenius-operator' does not contain a proper version. Check the HelmIndex for errors.", "HelmIndex", utils.HELM_INDEX)
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
		self.logger.Error("The umbrella chart 'mogenius-operator' does not contain a dependency for 'mogenius-k8s-manager'. Check the HelmIndex for errors.", "HelmIndex", utils.HELM_INDEX)
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

func (self *socketApi) upgradeK8sManager(command string) *structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob(self.eventsClient, "Upgrade mogenius platform", "UPGRADE", "", "")
	job.Start(self.eventsClient)
	kubernetes.UpgradeMyself(self.eventsClient, job, command, &wg)
	wg.Wait()
	job.Finish(self.eventsClient)
	return job
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
		tmpPreviousResReq, err := kubernetes.StreamPreviousLog(data.Namespace, data.PodId)
		if err != nil {
			self.logger.Error("failed to get previous pod log stream", "error", err)
		} else {
			previousResReq = tmpPreviousResReq
		}
	}

	restReq, err := kubernetes.StreamLog(data.Namespace, data.PodId, int64(data.SinceSeconds))
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
		self.sendDataWs(toServerUrl, stream)
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

	self.sendDataWs(toServerUrl, io.NopCloser(mergedStream))
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

func componentLogStreamConnection(componentLogConnectionRequest xterm.ComponentLogConnectionRequest) {
	xterm.ComponentStreamConnection(
		componentLogConnectionRequest.WsConnection,
		componentLogConnectionRequest.Component,
		componentLogConnectionRequest.Namespace,
		componentLogConnectionRequest.Controller,
		componentLogConnectionRequest.Release,
	)
}

func podEventStreamConnection(podLogConnectionRequest xterm.PodEventConnectionRequest) {
	xterm.PodEventStreamConnection(
		podLogConnectionRequest.WsConnection,
		podLogConnectionRequest.Namespace,
		podLogConnectionRequest.Controller,
	)
}

func (self *socketApi) executeBinaryRequestUpload(datagram structs.Datagram) *services.FilesUploadRequest {
	data := services.FilesUploadRequest{}
	structs.MarshalUnmarshal(&datagram, &data)
	return &data
}

func (self *socketApi) NormalizePatternName(pattern string) string {
	pattern = strings.ToUpper(pattern)
	pattern = strings.ReplaceAll(pattern, "/", "_")
	pattern = strings.ReplaceAll(pattern, "-", "_")
	return pattern
}

func (self *socketApi) sendDataWs(sendToServer string, reader io.ReadCloser) {
	defer func() {
		if reader != nil {
			err := reader.Close()
			if err != nil {
				self.logger.Debug("failed to close reader", "error", err)
			}
		}
	}()

	header := utils.HttpHeader("-logs")
	var dialer *gorillawebsocket.Dialer = gorillawebsocket.DefaultDialer
	if self.config.Get("MO_HTTP_PROXY") != "" {
		dialer.Proxy = http.ProxyURL(&url.URL{
			Scheme: "http",
			Host:   self.config.Get("MO_HTTP_PROXY"),
		})
	}
	conn, _, err := dialer.Dial(sendToServer, header)
	if err != nil {
		self.logger.Error("Connection to stream endpoint failed", "sendToServer", sendToServer, "error", err)
		return
	}

	defer func() {
		err := conn.Close()
		if err != nil {
			self.logger.Debug("failed to close connection", "error", err)
		}
	}()

	// API send ack when it is ready to receive messages.
	err = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err != nil {
		self.logger.Error("Error setting read deadline.", "error", err)
		return
	}
	_, ack, err := conn.ReadMessage()
	if err != nil {
		self.logger.Error("Error reading ack message.", "error", err)
		return
	}

	self.logger.Info("Ready ack from stream endpoint.", "ack", string(ack))

	buf := make([]byte, 1024)
	for {
		if reader != nil {
			n, err := reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					self.logger.Error("Unexpected stop of stream.", "sendToServer", sendToServer)
				}
				return
			}
			// debugging
			// str := string(buf[:n])
			// StructsLogger.Info("Send data ws.", "data", str)

			err = conn.WriteMessage(gorillawebsocket.BinaryMessage, buf[:n])
			if err != nil {
				self.logger.Error("Error sending data", "sendToServer", sendToServer, "error", err)
				return
			}

			// if conn, ok := conn.UnderlyingConn().(*net.TCPConn); ok {
			// 	err := conn.SetWriteBuffer(0)
			// 	if err != nil {
			// 		StructsLogger.Error("Error flushing connection", "error", err)
			// 	}
			// }
		} else {
			return
		}
	}
}
