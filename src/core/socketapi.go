package core

import (
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
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	gorillawebsocket "github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
	"helm.sh/helm/v3/pkg/release"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	LoadRequest(datagram *structs.Datagram, data interface{}) error
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

type Void *struct{}

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

type PatternHandle struct {
	SocketApi SocketApi
	Pattern   string
}

func RegisterPatternHandler[RequestType any, ResponseType any](handle PatternHandle, config PatternConfig, callback func(request RequestType) (ResponseType, error)) {
	assert.Assert(handle.SocketApi != nil, "SocketApi has to be given")
	assert.Assert(handle.Pattern != "", "Pattern has to be defined")

	assert.Assert(config.RequestSchema == nil, "config.RequestSchema should be empty", "RegisterPatternHandler overrides this field.")
	var requestType RequestType
	config.RequestSchema = schema.Generate(requestType)

	assert.Assert(config.ResponseSchema == nil, "config.ResponseSchema should be empty", "RegisterPatternHandler overrides this field.")
	var responseType ResponseType
	config.ResponseSchema = schema.Generate(responseType)

	handle.SocketApi.RegisterPatternHandler(handle.Pattern, config, func(datagram structs.Datagram) (any, error) {
		var data RequestType
		kind := reflect.TypeOf(data).Kind()

		if kind != reflect.Pointer {
			err := handle.SocketApi.LoadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
		}

		if kind == reflect.Pointer && datagram.Payload != nil {
			err := handle.SocketApi.LoadRequest(&datagram, &data)
			if err != nil {
				return nil, err
			}
		}

		return callback(data)
	})
}

func (self *socketApi) RegisterPatternHandler(
	pattern string,
	config PatternConfig,
	callback func(datagram structs.Datagram) (any, error),
) {
	assert.Assert(config.RequestSchema != nil, "config.RequestSchema has to be set")
	assert.Assert(config.ResponseSchema != nil, "config.ResponseSchema has to be set")

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
		RegisterPatternHandler(
			PatternHandle{self, "describe"},
			PatternConfig{},
			func(request Void) (Response, error) {
				resp := Response{}
				resp.BuildInfo.BuildType = "prod"
				if utils.IsDevBuild() {
					resp.BuildInfo.BuildType = "dev"
				}
				resp.BuildInfo.Version = *version.NewVersion()
				resp.Patterns = self.PatternConfigs()
				return resp, nil
			})
	}

	self.RegisterPatternHandlerRaw(
		"K8sNotification",
		PatternConfig{},
		func(datagram structs.Datagram) any {
			self.logger.Info("Received pattern", "pattern", datagram.Pattern)
			return nil
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
				if err := self.LoadRequest(&datagram, &data); err != nil {
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

	{
		type Request struct {
			IncludeTraffic   bool `json:"includeTraffic" validate:"boolean"`
			IncludePodStats  bool `json:"includePodStats" validate:"boolean"`
			IncludeNodeStats bool `json:"includeNodeStats" validate:"boolean"`
		}
		RegisterPatternHandler(
			PatternHandle{self, "cluster/clear-valkey-cache"},
			PatternConfig{},
			func(request Request) (string, error) {
				return self.valkeyClient.ClearNonEssentialKeys(request.IncludeTraffic, request.IncludePodStats, request.IncludeNodeStats)
			},
		)
	}

	self.RegisterPatternHandlerRaw(
		"print-current-config",
		PatternConfig{
			ResponseSchema: schema.String(),
		},
		func(datagram structs.Datagram) any {
			return self.config.AsEnvs()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "install-metrics-server"},
		PatternConfig{},
		func(request Void) (string, error) {
			return services.InstallMetricsServer()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "install-ingress-controller-traefik"},
		PatternConfig{},
		func(request Void) (string, error) {
			return services.InstallIngressControllerTreafik()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "install-cert-manager"},
		PatternConfig{},
		func(request Void) (string, error) {
			return services.InstallCertManager()
		},
	)

	{
		type Request struct {
			Email string `json:"email" validate:"required,email"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "install-cluster-issuer"},
			PatternConfig{},
			func(request Request) (string, error) {
				secrets.AddSecret(request.Email)
				return services.InstallClusterIssuer(request.Email, 0)
			},
		)
	}

	RegisterPatternHandler(
		PatternHandle{self, "install-metallb"},
		PatternConfig{},
		func(request Void) (string, error) {
			return services.InstallMetalLb()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "install-kepler"},
		PatternConfig{},
		func(request Void) (string, error) {
			return services.InstallKepler()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "uninstall-metrics-server"},
		PatternConfig{},
		func(request Void) (string, error) {
			return services.UninstallMetricsServer()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "uninstall-ingress-controller-traefik"},
		PatternConfig{},
		func(request Void) (string, error) {
			return services.UninstallIngressControllerTreafik()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "uninstall-cert-manager"},
		PatternConfig{},
		func(request Void) (string, error) {
			return services.UninstallCertManager()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "uninstall-cluster-issuer"},
		PatternConfig{},
		func(request Void) (string, error) {
			return services.UninstallClusterIssuer()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "uninstall-metallb"},
		PatternConfig{},
		func(request Void) (string, error) {
			return services.UninstallMetalLb()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "uninstall-kepler"},
		PatternConfig{},
		func(request Void) (string, error) {
			return services.UninstallKepler()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "upgrade-metrics-server"},
		PatternConfig{},
		func(request Void) (string, error) {
			return services.UpgradeMetricsServer()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "upgrade-ingress-controller-traefik"},
		PatternConfig{},
		func(request Void) (string, error) {
			return services.UpgradeIngressControllerTreafik()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "upgrade-cert-manager"},
		PatternConfig{},
		func(request Void) (string, error) {
			return services.UpgradeCertManager()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "upgrade-metallb"},
		PatternConfig{},
		func(request Void) (string, error) {
			return services.UpgradeMetalLb()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "upgrade-kepler"},
		PatternConfig{},
		func(request Void) (string, error) {
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
				if err := self.LoadRequest(&datagram, &data); err != nil {
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
				if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
				if err := self.LoadRequest(&datagram, &data); err != nil {
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

		RegisterPatternHandler(
			PatternHandle{self, "stats/workspace-cpu-utilization"},
			PatternConfig{},
			func(request Request) ([]GenericChartEntry, error) {
				resources, err := self.apiService.GetWorkspaceControllers(request.WorkspaceName)
				if err != nil {
					return nil, err
				}
				return self.dbstats.GetWorkspaceStatsCpuUtilization(request.TimeOffsetMinutes, resources)
			},
		)

		RegisterPatternHandler(
			PatternHandle{self, "stats/workspace-memory-utilization"},
			PatternConfig{},
			func(request Request) ([]GenericChartEntry, error) {
				resources, err := self.apiService.GetWorkspaceControllers(request.WorkspaceName)
				if err != nil {
					return nil, err
				}
				return self.dbstats.GetWorkspaceStatsMemoryUtilization(request.TimeOffsetMinutes, resources)
			},
		)

		RegisterPatternHandler(
			PatternHandle{self, "stats/workspace-traffic-utilization"},
			PatternConfig{},
			func(request Request) ([]GenericChartEntry, error) {
				resources, err := self.apiService.GetWorkspaceControllers(request.WorkspaceName)
				if err != nil {
					return nil, err
				}
				return self.dbstats.GetWorkspaceStatsTrafficUtilization(request.TimeOffsetMinutes, resources)
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
				if err := self.LoadRequest(&datagram, &data); err != nil {
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
				if err := self.LoadRequest(&datagram, &data); err != nil {
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
				if err := self.LoadRequest(&datagram, &data); err != nil {
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
				if err := self.LoadRequest(&datagram, &data); err != nil {
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
				if err := self.LoadRequest(&datagram, &data); err != nil {
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
				if err := self.LoadRequest(&datagram, &data); err != nil {
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
				if err := self.LoadRequest(&datagram, &data); err != nil {
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
				if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
				return err
			}
			return services.Info(data)
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

	RegisterPatternHandler(
		PatternHandle{self, "cluster/list-persistent-volume-claims"},
		PatternConfig{},
		func(request services.ClusterListWorkloads) ([]v1.PersistentVolumeClaim, error) {
			return kubernetes.ListPersistentVolumeClaimsWithFieldSelector(request.Namespace, request.LabelSelector, request.Prefix)
		},
	)

	self.RegisterPatternHandlerRaw(
		"cluster/update-local-tls-secret",
		PatternConfig{
			RequestSchema: schema.Generate(services.ClusterUpdateLocalTlsSecret{}),
		},
		func(datagram structs.Datagram) any {
			data := services.ClusterUpdateLocalTlsSecret{}
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
				return err
			}
			return services.DeleteNamespace(self.eventsClient, data)
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
				return err.Error()
			}
			result, err := kubernetes.BackupNamespace(data.NamespaceName)
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
		RegisterPatternHandler(
			PatternHandle{self, "cluster/machine-stats"},
			PatternConfig{},
			func(request Request) ([]structs.MachineStats, error) {
				return self.dbstats.GetMachineStatsForNodes(request.Nodes), nil
			},
		)
	}

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-repo-add"},
		PatternConfig{},
		func(request helm.HelmRepoAddRequest) (string, error) {
			return helm.HelmRepoAdd(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-repo-patch"},
		PatternConfig{},
		func(request helm.HelmRepoPatchRequest) (string, error) {
			return helm.HelmRepoPatch(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-repo-update"},
		PatternConfig{},
		func(request Void) ([]helm.HelmEntryStatus, error) {
			return helm.HelmRepoUpdate()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-repo-list"},
		PatternConfig{},
		func(request Void) ([]*helm.HelmEntryWithoutPassword, error) {
			return helm.HelmRepoList()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-chart-remove"},
		PatternConfig{}, func(request helm.HelmRepoRemoveRequest) (string, error) {
			return helm.HelmRepoRemove(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-chart-search"},
		PatternConfig{},
		func(request helm.HelmChartSearchRequest) ([]helm.HelmChartInfo, error) {
			return helm.HelmChartSearch(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-chart-install"},
		PatternConfig{},
		func(request helm.HelmChartInstallUpgradeRequest) (string, error) {
			return helm.HelmChartInstall(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-chart-install-oci"},
		PatternConfig{},
		func(request helm.HelmChartOciInstallUpgradeRequest) (string, error) {
			return helm.HelmOciInstall(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-chart-show"},
		PatternConfig{},
		func(request helm.HelmChartShowRequest) (string, error) {
			return helm.HelmChartShow(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-chart-versions"},
		PatternConfig{},
		func(request helm.HelmChartVersionRequest) ([]helm.HelmChartInfo, error) {
			return helm.HelmChartVersion(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-release-upgrade"},
		PatternConfig{},
		func(request helm.HelmChartInstallUpgradeRequest) (string, error) {
			return helm.HelmReleaseUpgrade(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-release-uninstall"},
		PatternConfig{},
		func(request helm.HelmReleaseUninstallRequest) (string, error) {
			return helm.HelmReleaseUninstall(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-release-list"},
		PatternConfig{},
		func(request helm.HelmReleaseListRequest) ([]*release.Release, error) {
			return helm.HelmReleaseList(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-release-status"},
		PatternConfig{},
		func(request helm.HelmReleaseStatusRequest) (*helm.HelmReleaseStatusInfo, error) {
			return helm.HelmReleaseStatus(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-release-history"},
		PatternConfig{},
		func(request helm.HelmReleaseHistoryRequest) ([]*release.Release, error) {
			return helm.HelmReleaseHistory(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-release-rollback"},
		PatternConfig{},
		func(request helm.HelmReleaseRollbackRequest) (string, error) {
			return helm.HelmReleaseRollback(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-release-get"},
		PatternConfig{},
		func(request helm.HelmReleaseGetRequest) (string, error) {
			return helm.HelmReleaseGet(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-release-get-workloads"},
		PatternConfig{},
		func(request helm.HelmReleaseGetWorkloadsRequest) ([]unstructured.Unstructured, error) {
			return helm.HelmReleaseGetWorkloads(self.valkeyClient, request)
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
				return err
			}
			data.Service.AddSecretsToRedaction()
			data.Project.AddSecretsToRedaction()
			return services.DeleteService(self.eventsClient, data)
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
				return err
			}
			return services.ServicePodStatus(self.eventsClient, data)
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
				return err
			}
			return services.StatusServiceDebounced(self.valkeyClient, data)
		},
	)

	self.RegisterPatternHandlerRaw(
		"service/exec-sh-connection-request",
		PatternConfig{
			RequestSchema: schema.Generate(xterm.PodCmdConnectionRequest{}),
		},
		func(datagram structs.Datagram) any {
			data := xterm.PodCmdConnectionRequest{}
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
				return err
			}
			go podEventStreamConnection(data)
			return nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "list/all-workloads"},
		PatternConfig{},
		func(request Void) ([]utils.SyncResourceEntry, error) {
			return kubernetes.GetAvailableResources()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "get/workload-list"},
		PatternConfig{},
		func(request utils.SyncResourceEntry) (unstructured.UnstructuredList, error) {
			return kubernetes.GetUnstructuredResourceListFromStore(request.Group, request.Kind, request.Version, request.Name, request.Namespace)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "get/namespace-workload-list"},
		PatternConfig{},
		func(request kubernetes.GetUnstructuredNamespaceResourceListRequest) ([]unstructured.Unstructured, error) {
			return kubernetes.GetUnstructuredNamespaceResourceList(request.Namespace, request.Whitelist, request.Blacklist)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "get/labeled-workload-list"},
		PatternConfig{},
		func(request kubernetes.GetUnstructuredLabeledResourceListRequest) (unstructured.UnstructuredList, error) {
			return kubernetes.GetUnstructuredLabeledResourceList(request.Label, request.Whitelist, request.Blacklist)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "describe/workload"},
		PatternConfig{},
		func(request utils.SyncResourceItem) (string, error) {
			return kubernetes.DescribeUnstructuredResource(request.Group, request.Version, request.Name, request.Namespace, request.ResourceName)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "create/new-workload"},
		PatternConfig{},
		func(request utils.SyncResourceData) (*unstructured.Unstructured, error) {
			return kubernetes.CreateUnstructuredResource(request.Group, request.Version, request.Name, request.Namespace, request.YamlData)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "get/workload"},
		PatternConfig{},
		func(request utils.SyncResourceItem) (*unstructured.Unstructured, error) {
			return kubernetes.GetUnstructuredResource(request.Group, request.Version, request.Name, request.Namespace, request.ResourceName)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "get/workload-status"},
		PatternConfig{},
		func(request kubernetes.GetWorkloadStatusRequest) ([]kubernetes.WorkloadStatusDto, error) {
			return kubernetes.GetWorkloadStatus(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "get/workload-example"},
		PatternConfig{},
		func(request utils.SyncResourceItem) (string, error) {
			return kubernetes.GetResourceTemplateYaml(request.Group, request.Version, request.Name, request.Kind, request.Namespace, request.ResourceName), nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "update/workload"},
		PatternConfig{},
		func(request utils.SyncResourceData) (*unstructured.Unstructured, error) {
			return kubernetes.UpdateUnstructuredResource(request.Group, request.Version, request.Name, request.Namespace, request.YamlData)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "delete/workload"},
		PatternConfig{},
		func(request utils.SyncResourceItem) (Void, error) {
			return nil, kubernetes.DeleteUnstructuredResource(request.Group, request.Version, request.Name, request.Namespace, request.ResourceName)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "trigger/workload"},
		PatternConfig{},
		func(request utils.SyncResourceItem) (*unstructured.Unstructured, error) {
			return kubernetes.TriggerUnstructuredResource(request.Group, request.Version, request.Name, request.Namespace, request.ResourceName)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "get/workspaces"},
		PatternConfig{},
		func(request Void) ([]GetWorkspaceResult, error) {
			return self.apiService.GetAllWorkspaces()
		},
	)

	{
		type Request struct {
			Name        string                                 `json:"name" validate:"required"`
			DisplayName string                                 `json:"displayName"`
			Resources   []v1alpha1.WorkspaceResourceIdentifier `json:"resources" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "create/workspace"},
			PatternConfig{},
			func(request Request) (string, error) {
				return self.apiService.CreateWorkspace(request.Name, v1alpha1.NewWorkspaceSpec(
					request.DisplayName,
					request.Resources,
				))
			},
		)
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "get/workspace"},
			PatternConfig{},
			func(request Request) (*GetWorkspaceResult, error) {
				return self.apiService.GetWorkspace(request.Name)
			},
		)
	}

	{
		type Request struct {
			Name        string `json:"name" validate:"required"`
			DryRun      bool   `json:"dryRun" validate:"boolean"`
			ReplicaSets bool   `json:"replicaSets" validate:"boolean"`
			Pods        bool   `json:"pods" validate:"boolean"`
			Services    bool   `json:"services" validate:"boolean"`
			Secrets     bool   `json:"secrets" validate:"boolean"`
			ConfigMaps  bool   `json:"configMaps" validate:"boolean"`
			Jobs        bool   `json:"jobs" validate:"boolean"`
			Ingresses   bool   `json:"ingresses" validate:"boolean"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "clean/workspace"},
			PatternConfig{},
			func(request Request) (CleanUpResult, error) {
				return self.moKubernetes.CleanUp(
					self.apiService,
					request.Name,
					request.DryRun,
					request.ReplicaSets,
					request.Pods,
					request.Services,
					request.Secrets,
					request.ConfigMaps,
					request.Jobs,
					request.Ingresses)
			},
		)
	}

	{
		type Request struct {
			Name        string                                 `json:"name" validate:"required"`
			DisplayName string                                 `json:"displayName"`
			Resources   []v1alpha1.WorkspaceResourceIdentifier `json:"resources" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "update/workspace"},
			PatternConfig{},
			func(request Request) (string, error) {
				return self.apiService.UpdateWorkspace(request.Name, v1alpha1.NewWorkspaceSpec(
					request.DisplayName,
					request.Resources,
				))
			},
		)
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "delete/workspace"},
			PatternConfig{},
			func(request Request) (string, error) {
				return self.apiService.DeleteWorkspace(request.Name)
			},
		)
	}

	{
		type Request struct {
			Email *string `json:"email"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "get/users"},
			PatternConfig{},
			func(request Request) ([]v1alpha1.User, error) {
				return self.apiService.GetAllUsers(request.Email)
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

		RegisterPatternHandler(
			PatternHandle{self, "create/user"},
			PatternConfig{},
			func(request Request) (string, error) {
				return self.apiService.CreateUser(request.Name, v1alpha1.NewUserSpec(
					request.FirstName,
					request.LastName,
					request.Email,
				))
			},
		)
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "get/user"},
			PatternConfig{},
			func(request Request) (*v1alpha1.User, error) {
				return self.apiService.GetUser(request.Name)
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

		RegisterPatternHandler(
			PatternHandle{self, "update/user"},
			PatternConfig{},
			func(request Request) (string, error) {
				return self.apiService.UpdateUser(request.Name, v1alpha1.NewUserSpec(
					request.FirstName,
					request.LastName,
					request.Email,
				))
			},
		)
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "delete/user"},
			PatternConfig{},
			func(request Request) (string, error) {
				return self.apiService.DeleteUser(request.Name)
			},
		)
	}

	{

		type Request struct {
			TargetType *string `json:"targetType"`
			TargetName *string `json:"targetName"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "get/grants"},
			PatternConfig{},
			func(request Request) ([]v1alpha1.Grant, error) {
				return self.apiService.GetAllGrants(request.TargetType, request.TargetName)
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

		RegisterPatternHandler(
			PatternHandle{self, "create/grant"},
			PatternConfig{},
			func(request Request) (string, error) {
				return self.apiService.CreateGrant(request.Name, v1alpha1.NewGrantSpec(
					request.Grantee,
					request.TargetType,
					request.TargetName,
					request.Role,
				))
			},
		)
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "get/grant"},
			PatternConfig{},
			func(request Request) (*v1alpha1.Grant, error) {
				return self.apiService.GetGrant(request.Name)
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

		RegisterPatternHandler(
			PatternHandle{self, "update/grant"},
			PatternConfig{},
			func(request Request) (string, error) {
				return self.apiService.UpdateGrant(request.Name, v1alpha1.NewGrantSpec(
					request.Grantee,
					request.TargetType,
					request.TargetName,
					request.Role,
				))
			},
		)
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "delete/grant"},
			PatternConfig{},
			func(request Request) (string, error) {
				return self.apiService.DeleteGrant(request.Name)
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

		RegisterPatternHandler(
			PatternHandle{self, "get/workspace-workloads"},
			PatternConfig{},
			func(request Request) ([]unstructured.Unstructured, error) {
				return self.apiService.GetWorkspaceResources(request.WorkspaceName, request.Whitelist, request.Blacklist, request.NamespaceWhitelist)
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
				return err
			}
			return controllers.DeleteExternalSecretsStore(data)
		},
	)

	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// Labeled Network Policies
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	RegisterPatternHandler(
		PatternHandle{self, "attach/labeled_network_policy"},
		PatternConfig{},
		func(request controllers.AttachLabeledNetworkPolicyRequest) (string, error) {
			return controllers.AttachLabeledNetworkPolicy(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "detach/labeled_network_policy"},
		PatternConfig{},
		func(request controllers.DetachLabeledNetworkPolicyRequest) (string, error) {
			return controllers.DetachLabeledNetworkPolicy(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "list/labeled_network_policy_ports"},
		PatternConfig{},
		func(request Void) ([]dtos.K8sLabeledNetworkPolicyDto, error) {
			return controllers.ListLabeledNetworkPolicyPorts()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "list/conflicting_network_policies"},
		PatternConfig{},
		func(request controllers.ListConflictingNetworkPoliciesRequest) ([]controllers.K8sConflictingNetworkPolicyDto, error) {
			return controllers.ListAllConflictingNetworkPolicies(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "remove/conflicting_network_policies"},
		PatternConfig{},
		func(request controllers.RemoveConflictingNetworkPoliciesRequest) (string, error) {
			return controllers.RemoveConflictingNetworkPolicies(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "list/controller_network_policies"},
		PatternConfig{},
		func(request controllers.ListControllerLabeledNetworkPoliciesRequest) (controllers.ListControllerLabeledNetworkPoliciesResponse, error) {
			return controllers.ListControllerLabeledNetwork(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "update/network_policies_template"},
		PatternConfig{},
		func(request []kubernetes.NetworkPolicy) (Void, error) {
			return nil, controllers.UpdateNetworkPolicyTemplate(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "list/all_network_policies"},
		PatternConfig{},
		func(request Void) ([]controllers.ListNetworkPolicyNamespace, error) {
			return controllers.ListAllNetworkPolicies(self.valkeyClient)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "list/namespace_network_policies"},
		PatternConfig{},
		func(request controllers.ListNamespaceLabeledNetworkPoliciesRequest) ([]controllers.ListNetworkPolicyNamespace, error) {
			return controllers.ListNamespaceNetworkPolicies(self.valkeyClient, request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "enforce/network_policy_manager"},
		PatternConfig{},
		func(request controllers.EnforceNetworkPolicyManagerRequest) (Void, error) {
			return nil, controllers.EnforceNetworkPolicyManager(request.NamespaceName)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "disable/network_policy_manager"},
		PatternConfig{},
		func(request controllers.DisableNetworkPolicyManagerRequest) (Void, error) {
			return nil, controllers.DisableNetworkPolicyManager(request.NamespaceName)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "remove/unmanaged_network_policies"},
		PatternConfig{},
		func(request controllers.RemoveUnmanagedNetworkPoliciesRequest) (Void, error) {
			return nil, controllers.RemoveUnmanagedNetworkPolicies(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "list/only_namespace_network_policies"},
		PatternConfig{},
		func(request controllers.ListNamespaceLabeledNetworkPoliciesRequest) ([]controllers.ListManagedAndUnmanagedNetworkPolicyNamespace, error) {
			return controllers.ListManagedAndUnmanagedNamespaceNetworkPolicies(self.valkeyClient, request)
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
				if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
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
			if err := self.LoadRequest(&datagram, &data); err != nil {
				return err
			}
			go self.xtermService.LiveStreamConnection(data, datagram, self.httpService)
			return nil
		},
	)
}

func (self *socketApi) LoadRequest(datagram *structs.Datagram, data interface{}) error {
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

	validateError := utils.ValidateJSON(data)
	if validateError != nil {
		return validateError
	}

	return nil
}

func (self *socketApi) startK8sManager() {
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

func (self *socketApi) JobServerSendData(jobClient websocket.WebsocketClient, datagram structs.Datagram) {
	go func() {
		err := jobClient.WriteJSON(datagram)
		if err != nil {
			self.logger.Error("Error sending data to EventServer", "error", err)
		}
	}()
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

	job := structs.CreateJob(self.eventsClient, "Upgrade mogenius platform", "UPGRADE", "", "", self.logger)
	job.Start(self.eventsClient)
	kubernetes.UpgradeMyself(self.eventsClient, job, command, &wg)
	wg.Wait()
	job.Finish(self.eventsClient)
	return job
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
