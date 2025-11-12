package core

import (
	"fmt"
	"log/slog"
	argocd "mogenius-operator/src/argocd"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/dtos"
	"mogenius-operator/src/helm"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/networkmonitor"
	"mogenius-operator/src/schema"
	"mogenius-operator/src/secrets"
	"mogenius-operator/src/services"
	"mogenius-operator/src/shutdown"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"mogenius-operator/src/version"
	"mogenius-operator/src/websocket"
	"mogenius-operator/src/xterm"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"helm.sh/helm/v3/pkg/release"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type SocketApi interface {
	Link(
		httpService HttpService,
		xtermService XtermService,
		dbstatsModule ValkeyStatsDb,
		apiService Api,
		moKubernetes MoKubernetes,
		sealedSecret SealedSecretManager,
	)
	Run()
	Status() SocketApiStatus
	ExecuteCommandRequest(datagram structs.Datagram) any
	ParseDatagram(data []byte) (structs.Datagram, error)
	AddPatternHandler(
		pattern string,
		config PatternConfig,
		callback func(datagram structs.Datagram) any,
	)
	PatternConfigs() map[string]PatternConfig
	NormalizePatternName(pattern string) string
	AssertPatternsUnique()
	LoadRequest(datagram *structs.Datagram, data any) error
}

type SocketApiStatus struct {
	IsRunning bool `json:"is_running"`
}

func NewSocketApiStatus() SocketApiStatus {
	status := SocketApiStatus{}
	return status
}

func (self *SocketApiStatus) Clone() SocketApiStatus {
	return SocketApiStatus{
		self.IsRunning,
	}
}

type socketApi struct {
	logger *slog.Logger

	jobClient    websocket.WebsocketClient
	eventsClient websocket.WebsocketClient

	config       config.ConfigModule
	valkeyClient valkeyclient.ValkeyClient
	dbstats      ValkeyStatsDb

	status     SocketApiStatus
	statusLock sync.RWMutex

	patternlog *os.File

	logLevelMo bool

	// the patternHandler should only be edited on startup
	patternHandlerLock sync.RWMutex
	patternHandler     map[string]PatternHandler
	httpService        HttpService
	xtermService       XtermService
	apiService         Api
	moKubernetes       MoKubernetes
	sealedSecret       SealedSecretManager
	argocd             argocd.Argocd
}

type PatternHandler struct {
	Config   PatternConfig
	Callback func(datagram structs.Datagram) any
}

type PatternConfig struct {
	Deprecated        bool   `json:"deprecated,omitempty"`
	DeprecatedMessage string `json:"deprecatedMessage,omitempty"`
	NeedsUser         bool   `json:"needsUser,omitempty"`
	// @readonly: do not set this manually
	LegacyResponseLayout bool `json:"legacyResponseLayout,omitempty"`
	// @readonly: do not set this manually
	RequestSchema *schema.Schema `json:"requestSchema,omitempty"`
	// @readonly: do not set this manually
	ResponseSchema *schema.Schema `json:"responseSchema,omitempty"`
}

type Void *struct{}

func NewSocketApi(
	logger *slog.Logger,
	configModule config.ConfigModule,
	jobClient websocket.WebsocketClient,
	eventsClient websocket.WebsocketClient,
	valkeyClient valkeyclient.ValkeyClient,
	argocd argocd.Argocd,
) SocketApi {
	self := &socketApi{}
	self.config = configModule
	self.jobClient = jobClient
	self.eventsClient = eventsClient
	self.logger = logger
	self.patternHandler = map[string]PatternHandler{}
	self.valkeyClient = valkeyClient
	self.status = NewSocketApiStatus()
	self.statusLock = sync.RWMutex{}
	self.argocd = argocd

	self.loadpatternlogger()
	self.registerPatterns()

	return self
}

func (self *socketApi) Link(
	httpService HttpService,
	xtermService XtermService,
	dbstatsModule ValkeyStatsDb,
	apiService Api,
	moKubernetes MoKubernetes,
	sealedSecret SealedSecretManager,
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
	self.sealedSecret = sealedSecret
}

func (self *socketApi) Run() {
	assert.Assert(self.apiService != nil)
	assert.Assert(self.httpService != nil)
	assert.Assert(self.xtermService != nil)

	self.AssertPatternsUnique()
	self.startMessageHandler()

	self.statusLock.Lock()
	self.status.IsRunning = true
	self.statusLock.Unlock()
}

func (self *socketApi) Status() SocketApiStatus {
	self.statusLock.RLock()
	defer self.statusLock.RUnlock()
	return self.status.Clone()
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

// Register a pattern handler which responds with Payload{"status":"success"|"error","data":{...}}
//
// The API response field `"payload"` is a tagged union type with two fields called `status` and `data`.
//
// if `status` equals "success" the field `data` contains `ResponseType`
// if `status` equals "error" the field `data` contains a `string` with the an message
func RegisterPatternHandler[RequestType any, ResponseType any](
	handle PatternHandle,
	config PatternConfig,
	callback func(datagram structs.Datagram, request RequestType) (ResponseType, error),
) {
	assert.Assert(handle.SocketApi != nil, "SocketApi has to be given")
	assert.Assert(handle.Pattern != "", "Pattern has to be defined")

	type Result struct {
		Status  string       `json:"status"` // success, error
		Message string       `json:"message,omitempty"`
		Data    ResponseType `json:"data"`
	}

	buildResponse := func(result any, err error) Result {
		if err != nil {
			return Result{
				Status:  "error",
				Message: err.Error(),
			}
		}
		if str, ok := result.(string); ok {
			return Result{
				Status:  "success",
				Message: str,
			}
		}
		return Result{
			Status: "success",
			Data:   result.(ResponseType),
		}
	}

	config.LegacyResponseLayout = false

	assert.Assert(config.RequestSchema == nil, "config.RequestSchema should be empty", "RegisterPatternHandler overrides this field.")
	var requestType RequestType
	config.RequestSchema = schema.Generate(requestType)

	assert.Assert(config.ResponseSchema == nil, "config.ResponseSchema should be empty", "RegisterPatternHandler overrides this field.")
	var responseType Result
	config.ResponseSchema = schema.Generate(responseType)

	handle.SocketApi.AddPatternHandler(handle.Pattern, config, func(datagram structs.Datagram) any {
		var data RequestType
		kind := reflect.TypeOf(data).Kind()

		if kind != reflect.Pointer {
			err := handle.SocketApi.LoadRequest(&datagram, &data)
			if err != nil {
				return buildResponse(nil, err)
			}
		}

		if kind == reflect.Pointer && datagram.Payload != nil {
			err := handle.SocketApi.LoadRequest(&datagram, &data)
			if err != nil {
				return buildResponse(nil, err)
			}
		}

		result, err := callback(datagram, data)

		return buildResponse(result, err)
	})
}

func (self *socketApi) AddPatternHandler(
	pattern string,
	config PatternConfig,
	callback func(datagram structs.Datagram) any,
) {
	assert.Assert(config.RequestSchema != nil, "config.RequestSchema has to be set")
	assert.Assert(config.ResponseSchema != nil, "config.ResponseSchema has to be set")

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
			func(datagram structs.Datagram, request Void) (Response, error) {
				resp := Response{}
				resp.BuildInfo.BuildType = utils.STAGE_PROD
				if utils.IsDevBuild() {
					resp.BuildInfo.BuildType = utils.STAGE_DEV
				}
				resp.BuildInfo.Version = *version.NewVersion()
				resp.Patterns = self.PatternConfigs()
				return resp, nil
			})
	}

	{
		type ClusterResourceInfo struct {
			LoadBalancerExternalIps []string              `json:"loadBalancerExternalIps"`
			NodeStats               []dtos.NodeStat       `json:"nodeStats"`
			Country                 *utils.CountryDetails `json:"country"`
			Provider                string                `json:"provider"`
			CniConfig               []structs.CniData     `json:"cniConfig"`
			Errors                  []string              `json:"error,omitempty"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "cluster/resource-info"},
			PatternConfig{},
			func(datagram structs.Datagram, request Void) (ClusterResourceInfo, error) {
				errors := []string{}
				nodeStats, nodeErr := self.moKubernetes.GetNodeStats()
				if nodeErr != nil {
					errors = append(errors, nodeErr.Error())
				}
				loadBalancerExternalIps := kubernetes.GetClusterExternalIps()
				country, _ := utils.GuessClusterCountry()
				cniConfig, _ := self.dbstats.GetCniData()
				response := ClusterResourceInfo{
					NodeStats:               nodeStats,
					LoadBalancerExternalIps: loadBalancerExternalIps,
					Country:                 country,
					Provider:                string(utils.ClusterProviderCached),
					CniConfig:               cniConfig,
					Errors:                  errors,
				}
				return response, nil
			},
		)
	}

	{
		type Request struct {
			Command string `json:"command" validate:"required"` // complete helm command from platform ui
		}

		RegisterPatternHandler(
			PatternHandle{self, "UpgradeK8sManager"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (*structs.Job, error) {
				return self.upgradeK8sManager(request.Command)
			},
		)
	}

	RegisterPatternHandler(
		PatternHandle{self, "cluster/force-reconnect"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (bool, error) {
			time.Sleep(1 * time.Second)
			return kubernetes.ClusterForceReconnect(), nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/force-disconnect"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (bool, error) {
			time.Sleep(1 * time.Second)
			return kubernetes.ClusterForceDisconnect(), nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "system/check"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (services.SystemCheckResponse, error) {
			return services.SystemCheck(), nil
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
			func(datagram structs.Datagram, request Request) (string, error) {
				return self.valkeyClient.ClearNonEssentialKeys(request.IncludeTraffic, request.IncludePodStats, request.IncludeNodeStats)
			},
		)
	}

	RegisterPatternHandler(
		PatternHandle{self, "install-metrics-server"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (string, error) {
			return services.InstallMetricsServer()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "install-ingress-controller-traefik"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (string, error) {
			return services.InstallIngressControllerTreafik()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "install-cert-manager"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (string, error) {
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
			func(datagram structs.Datagram, request Request) (string, error) {
				secrets.AddSecret(request.Email)
				return services.InstallClusterIssuer(request.Email, 0)
			},
		)
	}

	RegisterPatternHandler(
		PatternHandle{self, "install-metallb"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (string, error) {
			return services.InstallMetalLb()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "install-kepler"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (string, error) {
			return services.InstallKepler()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "uninstall-metrics-server"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (string, error) {
			return services.UninstallMetricsServer()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "uninstall-ingress-controller-traefik"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (string, error) {
			return services.UninstallIngressControllerTreafik()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "uninstall-cert-manager"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (string, error) {
			return services.UninstallCertManager()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "uninstall-cluster-issuer"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (string, error) {
			return services.UninstallClusterIssuer()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "uninstall-metallb"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (string, error) {
			return services.UninstallMetalLb()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "uninstall-kepler"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (string, error) {
			return services.UninstallKepler()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "upgrade-metrics-server"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (string, error) {
			return services.UpgradeMetricsServer()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "upgrade-ingress-controller-traefik"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (string, error) {
			return services.UpgradeIngressControllerTreafik()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "upgrade-cert-manager"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (string, error) {
			return services.UpgradeCertManager()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "upgrade-metallb"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (string, error) {
			return services.UpgradeMetalLb()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "upgrade-kepler"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (string, error) {
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

		RegisterPatternHandler(
			PatternHandle{self, "stats/pod/all-for-controller"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (*[]structs.PodStats, error) {
				if request.TimeOffsetMinutes <= 0 {
					request.TimeOffsetMinutes = 60 * 24 // 1 day
				}
				entries := self.dbstats.GetPodStatsEntriesForController(request.Kind, request.Name, request.Namespace, int64(request.TimeOffsetMinutes))
				return entries, nil
			},
		)

		RegisterPatternHandler(
			PatternHandle{self, "stats/traffic/all-for-controller"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (*[]networkmonitor.PodNetworkStats, error) {
				if request.TimeOffsetMinutes <= 0 {
					request.TimeOffsetMinutes = 60 * 24 // 1 day
				}
				stats := self.dbstats.GetTrafficStatsEntriesForController(request.Kind, request.Name, request.Namespace, int64(request.TimeOffsetMinutes))
				return stats, nil
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
			func(datagram structs.Datagram, request Request) ([]GenericChartEntry, error) {
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
			func(datagram structs.Datagram, request Request) ([]GenericChartEntry, error) {
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
			func(datagram structs.Datagram, request Request) ([]GenericChartEntry, error) {
				resources, err := self.apiService.GetWorkspaceControllers(request.WorkspaceName)
				if err != nil {
					return nil, err
				}
				return self.dbstats.GetWorkspaceStatsTrafficUtilization(request.TimeOffsetMinutes, resources)
			},
		)
	}

	// Deprecated: will be removed in future versions
	{
		type Request struct {
			Folder dtos.PersistentFileRequestDto `json:"folder" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "files/list"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) ([]dtos.PersistentFileDto, error) {
				return services.List(request.Folder)
			},
		)
	}

	// Deprecated: will be removed in future versions
	{
		type Request struct {
			Folder dtos.PersistentFileRequestDto `json:"folder" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "files/create-folder"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (bool, error) {
				return true, services.CreateFolder(request.Folder)
			},
		)
	}

	// Deprecated: will be removed in future versions
	{
		type Request struct {
			File    dtos.PersistentFileRequestDto `json:"file" validate:"required"`
			NewName string                        `json:"newName" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "files/rename"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (bool, error) {
				return true, services.Rename(request.File, request.NewName)
			},
		)
	}

	// Deprecated: will be removed in future versions
	{
		type Request struct {
			File dtos.PersistentFileRequestDto `json:"file" validate:"required"`
			Uid  string                        `json:"uid" validate:"required"`
			Gid  string                        `json:"gid" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "files/chown"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (bool, error) {
				return true, services.Chown(request.File, request.Uid, request.Gid)
			},
		)
	}

	// Deprecated: will be removed in future versions
	{
		type Request struct {
			File dtos.PersistentFileRequestDto `json:"file" validate:"required"`
			Mode string                        `json:"mode" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "files/chmod"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (bool, error) {
				return true, services.Chmod(request.File, request.Mode)
			},
		)
	}

	// Deprecated: will be removed in future versions
	{
		type Request struct {
			File dtos.PersistentFileRequestDto `json:"file" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "files/delete"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (bool, error) {
				return true, services.Delete(request.File)
			},
		)
	}

	// Deprecated: will be removed in future versions
	{
		type Request struct {
			File   dtos.PersistentFileRequestDto `json:"file" validate:"required"`
			PostTo string                        `json:"postTo" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "files/download"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (services.FilesDownloadResponse, error) {
				return services.Download(request.File, request.PostTo)
			},
		)
	}

	// Deprecated: will be removed in future versions
	RegisterPatternHandler(
		PatternHandle{self, "files/info"},
		PatternConfig{},
		func(datagram structs.Datagram, request dtos.PersistentFileRequestDto) (dtos.PersistentFileDto, error) {
			return services.Info(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "prometheus/query"},
		PatternConfig{},
		func(datagram structs.Datagram, request PrometheusRequest) (*PrometheusQueryResponse, error) {
			return ExecutePrometheusQuery(request, self.config)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "prometheus/is-reachable"},
		PatternConfig{},
		func(datagram structs.Datagram, request PrometheusRequest) (bool, error) {
			data, err := IsPrometheusReachable(request, self.config)
			return data, err
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "prometheus/values"},
		PatternConfig{},
		func(datagram structs.Datagram, request PrometheusRequest) ([]string, error) {
			return PrometheusValues(request, self.config)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "prometheus/charts/add"},
		PatternConfig{},
		func(datagram structs.Datagram, request PrometheusRequestRedis) (*string, error) {
			return PrometheusSaveQueryToRedis(self.valkeyClient, request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "prometheus/charts/remove"},
		PatternConfig{},
		func(datagram structs.Datagram, request PrometheusRequestRedis) (*string, error) {
			return PrometheusRemoveQueryFromRedis(self.valkeyClient, request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "prometheus/charts/get"},
		PatternConfig{},
		func(datagram structs.Datagram, request PrometheusRequestRedis) (*PrometheusStoreObject, error) {
			return PrometheusGetQueryFromRedis(self.valkeyClient, request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "prometheus/charts/list"},
		PatternConfig{},
		func(datagram structs.Datagram, request PrometheusRequestRedisList) (map[string]PrometheusStoreObject, error) {
			result, err := PrometheusListQueriesFromRedis(self.valkeyClient, request)
			return result, err
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/list-persistent-volume-claims"},
		PatternConfig{},
		func(datagram structs.Datagram, request services.ClusterListWorkloads) ([]v1.PersistentVolumeClaim, error) {
			return kubernetes.ListPersistentVolumeClaimsWithFieldSelector(request.Namespace, request.LabelSelector, request.Prefix)
		},
	)

	{
		type Request struct {
			Nodes []string `json:"nodes" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "cluster/machine-stats"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) ([]structs.MachineStats, error) {
				return self.dbstats.GetMachineStatsForNodes(request.Nodes), nil
			},
		)
	}

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-repo-add"},
		PatternConfig{},
		func(datagram structs.Datagram, request helm.HelmRepoAddRequest) (string, error) {
			result, err := helm.HelmRepoAdd(request)
			return store.AddToAuditLog(datagram, self.logger, result, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-repo-patch"},
		PatternConfig{},
		func(datagram structs.Datagram, request helm.HelmRepoPatchRequest) (string, error) {
			return helm.HelmRepoPatch(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-repo-update"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) ([]helm.HelmEntryStatus, error) {
			result, err := helm.HelmRepoUpdate()
			return store.AddToAuditLog(datagram, self.logger, result, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-repo-list"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) ([]*helm.HelmEntryWithoutPassword, error) {
			return helm.HelmRepoList()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-chart-remove"},
		PatternConfig{}, func(datagram structs.Datagram, request helm.HelmRepoRemoveRequest) (string, error) {
			res, err := helm.HelmRepoRemove(request)
			return store.AddToAuditLog(datagram, self.logger, res, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-chart-search"},
		PatternConfig{},
		func(datagram structs.Datagram, request helm.HelmChartSearchRequest) ([]helm.HelmChartInfo, error) {
			return helm.HelmChartSearch(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-chart-install"},
		PatternConfig{},
		func(datagram structs.Datagram, request helm.HelmChartInstallUpgradeRequest) (string, error) {
			res, err := helm.HelmChartInstall(request)
			return store.AddToAuditLog(datagram, self.logger, res, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-chart-install-oci"},
		PatternConfig{},
		func(datagram structs.Datagram, request helm.HelmChartOciInstallUpgradeRequest) (string, error) {
			res, err := helm.HelmOciInstall(request)
			return store.AddToAuditLog(datagram, self.logger, res, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-chart-show"},
		PatternConfig{},
		func(datagram structs.Datagram, request helm.HelmChartShowRequest) (string, error) {
			return helm.HelmChartShow(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-chart-versions"},
		PatternConfig{},
		func(datagram structs.Datagram, request helm.HelmChartVersionRequest) ([]helm.HelmChartInfo, error) {
			return helm.HelmChartVersion(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-release-upgrade"},
		PatternConfig{},
		func(datagram structs.Datagram, request helm.HelmChartInstallUpgradeRequest) (string, error) {
			res, err := helm.HelmReleaseUpgrade(request)
			return store.AddToAuditLog(datagram, self.logger, res, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-release-uninstall"},
		PatternConfig{},
		func(datagram structs.Datagram, request helm.HelmReleaseUninstallRequest) (string, error) {
			res, err := helm.HelmReleaseUninstall(request)
			return store.AddToAuditLog(datagram, self.logger, res, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-release-list"},
		PatternConfig{},
		func(datagram structs.Datagram, request helm.HelmReleaseListRequest) ([]*helm.HelmRelease, error) {
			return helm.HelmReleaseList(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-release-status"},
		PatternConfig{},
		func(datagram structs.Datagram, request helm.HelmReleaseStatusRequest) (*helm.HelmReleaseStatusInfo, error) {
			return helm.HelmReleaseStatus(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-release-history"},
		PatternConfig{},
		func(datagram structs.Datagram, request helm.HelmReleaseHistoryRequest) ([]*release.Release, error) {
			return helm.HelmReleaseHistory(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-release-rollback"},
		PatternConfig{},
		func(datagram structs.Datagram, request helm.HelmReleaseRollbackRequest) (string, error) {
			result, err := helm.HelmReleaseRollback(request)
			return store.AddToAuditLog(datagram, self.logger, result, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-release-get"},
		PatternConfig{},
		func(datagram structs.Datagram, request helm.HelmReleaseGetRequest) (string, error) {
			return helm.HelmReleaseGet(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-release-link"},
		PatternConfig{},
		func(datagram structs.Datagram, request helm.HelmReleaseLinkRequest) (string, error) {
			err := helm.SaveRepoNameToValkey(request.Namespace, request.ReleaseName, request.RepoName)
			return store.AddToAuditLog(datagram, self.logger, "", err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/helm-release-get-workloads"},
		PatternConfig{},
		func(datagram structs.Datagram, request helm.HelmReleaseGetWorkloadsRequest) ([]unstructured.Unstructured, error) {
			return helm.HelmReleaseGetWorkloads(self.valkeyClient, request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/argo-cd-create-api-token"},
		PatternConfig{},
		func(datagram structs.Datagram, request argocd.ArgoCdCreateApiTokenRequest) (bool, error) {
			return self.argocd.ArgoCdCreateApiToken(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/argo-cd-application-refresh"},
		PatternConfig{},
		func(datagram structs.Datagram, request argocd.ArgoCdApplicationRefreshRequest) (bool, error) {
			return self.argocd.ArgoCdApplicationRefresh(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "service/exec-sh-connection-request"},
		PatternConfig{},
		func(datagram structs.Datagram, request xterm.PodCmdConnectionRequest) (Void, error) {
			go self.execShConnection(request)
			_, err := store.AddToAuditLog(datagram, self.logger, any(nil), nil, nil, nil)
			if err != nil {
				self.logger.Warn("failed to add event to audit log", "request", request, "error", err)
			}
			return nil, nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "service/log-stream-connection-request"},
		PatternConfig{},
		func(datagram structs.Datagram, request xterm.PodCmdConnectionRequest) (Void, error) {
			go self.logStreamConnection(request)
			return nil, nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/component-log-stream-connection-request"},
		PatternConfig{},
		func(datagram structs.Datagram, request xterm.ComponentLogConnectionRequest) (Void, error) {
			go componentLogStreamConnection(request)
			return nil, nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "service/pod-event-stream-connection-request"},
		PatternConfig{},
		func(datagram structs.Datagram, request xterm.PodEventConnectionRequest) (Void, error) {
			go podEventStreamConnection(request)
			return nil, nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "list/all-resource-descriptors"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) ([]utils.ResourceDescriptor, error) {
			return kubernetes.GetAvailableResources()
		},
	)

	{
		type Request struct {
			Kind       string  `json:"kind"`
			Plural     string  `json:"plural"`
			ApiVersion string  `json:"apiVersion"`
			Namespace  *string `json:"namespace"`
			WithData   *bool   `json:"withData"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "get/workload-list"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (unstructured.UnstructuredList, error) {
				return kubernetes.GetUnstructuredResourceListFromStore(request.ApiVersion, request.Kind, request.Namespace, request.WithData), nil
			},
		)
	}

	RegisterPatternHandler(
		PatternHandle{self, "get/namespace-workload-list"},
		PatternConfig{},
		func(datagram structs.Datagram, request kubernetes.GetUnstructuredNamespaceResourceListRequest) ([]unstructured.Unstructured, error) {
			return kubernetes.GetUnstructuredNamespaceResourceList(request.Namespace, request.Whitelist, request.Blacklist)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "get/labeled-workload-list"},
		PatternConfig{},
		func(datagram structs.Datagram, request kubernetes.GetUnstructuredLabeledResourceListRequest) (unstructured.UnstructuredList, error) {
			return kubernetes.GetUnstructuredLabeledResourceList(request.Label, request.Whitelist, request.Blacklist)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "describe/workload"},
		PatternConfig{},
		func(datagram structs.Datagram, request utils.WorkloadSingleRequest) (string, error) {
			result, err := kubernetes.DescribeUnstructuredResource(request.ApiVersion, request.Plural, request.Namespace, request.ResourceName)
			return store.AddToAuditLog(datagram, self.logger, result, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "create/new-workload"},
		PatternConfig{},
		func(datagram structs.Datagram, request utils.WorkloadChangeRequest) (*unstructured.Unstructured, error) {
			createdRes, err := kubernetes.CreateUnstructuredResource(request.ApiVersion, request.Plural, request.Namespaced, request.YamlData)
			return store.AddToAuditLog(datagram, self.logger, createdRes, err, nil, createdRes)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "get/workload"},
		PatternConfig{},
		func(datagram structs.Datagram, request utils.WorkloadSingleRequest) (*unstructured.Unstructured, error) {
			// we skip the audit log here because this is a read operation and it would spam the logs
			return kubernetes.GetUnstructuredResourceFromStore(request.ApiVersion, request.Kind, request.Namespace, request.ResourceName)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "get/workload-example"},
		PatternConfig{},
		func(datagram structs.Datagram, request utils.ResourceDescriptor) (string, error) {
			return kubernetes.GetResourceTemplateYaml(request.ApiVersion, request.Kind), nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "update/workload"},
		PatternConfig{},
		func(datagram structs.Datagram, request utils.WorkloadChangeRequest) (*unstructured.Unstructured, error) {
			var updatedObj *unstructured.Unstructured
			err := yaml.Unmarshal([]byte(request.YamlData), &updatedObj)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal YAML data: %w", err)
			}
			oldObj, _ := kubernetes.GetUnstructuredResourceFromStore(request.ApiVersion, request.Kind, updatedObj.GetNamespace(), updatedObj.GetName())
			updatedRes, err := kubernetes.UpdateUnstructuredResource(request.ApiVersion, request.Plural, request.Namespaced, request.YamlData)
			return store.AddToAuditLog(datagram, self.logger, updatedRes, err, oldObj, updatedRes)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "delete/workload"},
		PatternConfig{},
		func(datagram structs.Datagram, request utils.WorkloadSingleRequest) (Void, error) {
			objToDel, _ := kubernetes.GetUnstructuredResourceFromStore(request.ApiVersion, request.Kind, request.Namespace, request.ResourceName)
			err := kubernetes.DeleteUnstructuredResource(request.ApiVersion, request.Plural, request.Namespace, request.ResourceName)
			_, auditErr := store.AddToAuditLog(datagram, self.logger, any(nil), err, objToDel, nil)
			if auditErr != nil {
				self.logger.Warn("failed to add event to audit log", "request", request, "error", err)
			}
			return nil, err
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "trigger/workload"},
		PatternConfig{},
		func(datagram structs.Datagram, request utils.WorkloadSingleRequest) (*unstructured.Unstructured, error) {
			res, err := kubernetes.TriggerUnstructuredResource(request.ApiVersion, request.Plural, request.Namespace, request.ResourceName)
			return store.AddToAuditLog(datagram, self.logger, res, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "get/workload-status"},
		PatternConfig{},
		func(datagram structs.Datagram, request kubernetes.GetWorkloadStatusRequest) ([]kubernetes.WorkloadStatusDto, error) {
			return kubernetes.GetWorkloadStatus(request)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "get/workspaces"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) ([]GetWorkspaceResult, error) {
			namespace := self.config.Get("MO_OWN_NAMESPACE")
			workspaces, err := store.GetAllWorkspaces(namespace)
			result := []GetWorkspaceResult{}
			for _, v := range workspaces {
				result = append(result, GetWorkspaceResult{
					Name:              v.Name,
					CreationTimestamp: v.CreationTimestamp,
					Resources:         v.Spec.Resources,
				})
			}
			return result, err
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
			func(datagram structs.Datagram, request Request) (string, error) {

				res, err := self.apiService.CreateWorkspace(request.Name, v1alpha1.NewWorkspaceSpec(
					request.DisplayName,
					request.Resources,
				))
				return store.AddToAuditLog(datagram, self.logger, res, err, nil, nil)
			},
		)
	}

	{
		type Request struct {
			Name      string `json:"name" validate:"required"`
			Namespace string `json:"namespace"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "get/workspace"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (*GetWorkspaceResult, error) {
				namespace := request.Namespace
				if namespace == "" {
					namespace = self.config.Get("MO_OWN_NAMESPACE")
				}

				workspace, err := store.GetWorkspace(namespace, request.Name)
				if err != nil || workspace == nil {
					return nil, err
				}
				result := NewGetWorkspaceResult(workspace.Name, workspace.CreationTimestamp, workspace.Spec.Resources)
				return &result, nil
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
			PatternHandle{self, "workspace/clean-up"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (CleanUpResult, error) {
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
			func(datagram structs.Datagram, request Request) (string, error) {
				res, err := self.apiService.UpdateWorkspace(request.Name, v1alpha1.NewWorkspaceSpec(
					request.DisplayName,
					request.Resources,
				))
				return store.AddToAuditLog(datagram, self.logger, res, err, nil, nil)
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
			func(datagram structs.Datagram, request Request) (string, error) {
				res, err := self.apiService.DeleteWorkspace(request.Name)
				return store.AddToAuditLog(datagram, self.logger, res, err, nil, nil)
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
			func(datagram structs.Datagram, request Request) ([]v1alpha1.User, error) {
				return self.apiService.GetAllUsers(request.Email)
			},
		)
	}

	{
		type Request struct {
			Name      string          `json:"name"`
			FirstName string          `json:"firstName"`
			LastName  string          `json:"lastName"`
			Email     string          `json:"email"`
			Subject   *rbacv1.Subject `json:"subject"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "create/user"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (string, error) {
				res, err := self.apiService.CreateUser(request.Name, v1alpha1.NewUserSpec(
					request.FirstName,
					request.LastName,
					request.Email,
					request.Subject,
				))
				return store.AddToAuditLog(datagram, self.logger, res, err, nil, nil)
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
			func(datagram structs.Datagram, request Request) (*v1alpha1.User, error) {
				return self.apiService.GetUser(request.Name)
			},
		)
	}

	{
		type Request struct {
			Name      string          `json:"name" validate:"required"`
			FirstName string          `json:"firstName"`
			LastName  string          `json:"lastName"`
			Email     string          `json:"email"`
			Subject   *rbacv1.Subject `json:"subject"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "update/user"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (string, error) {
				res, err := self.apiService.UpdateUser(request.Name, v1alpha1.NewUserSpec(
					request.FirstName,
					request.LastName,
					request.Email,
					request.Subject,
				))
				return store.AddToAuditLog(datagram, self.logger, res, err, nil, nil)
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
			func(datagram structs.Datagram, request Request) (string, error) {
				res, err := self.apiService.DeleteUser(request.Name)
				return store.AddToAuditLog(datagram, self.logger, res, err, nil, nil)
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
			func(datagram structs.Datagram, request Request) ([]v1alpha1.Grant, error) {
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
			func(datagram structs.Datagram, request Request) (string, error) {
				res, err := self.apiService.CreateGrant(request.Name, v1alpha1.NewGrantSpec(
					request.Grantee,
					request.TargetType,
					request.TargetName,
					request.Role,
				))
				return store.AddToAuditLog(datagram, self.logger, res, err, nil, nil)
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
			func(datagram structs.Datagram, request Request) (*v1alpha1.Grant, error) {
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
			func(datagram structs.Datagram, request Request) (string, error) {
				res, err := self.apiService.UpdateGrant(request.Name, v1alpha1.NewGrantSpec(
					request.Grantee,
					request.TargetType,
					request.TargetName,
					request.Role,
				))
				return store.AddToAuditLog(datagram, self.logger, res, err, nil, nil)
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
			func(datagram structs.Datagram, request Request) (string, error) {
				res, err := self.apiService.DeleteGrant(request.Name)
				return store.AddToAuditLog(datagram, self.logger, res, err, nil, nil)
			},
		)
	}

	{
		type Request struct {
			WorkspaceName      string                      `json:"workspaceName"`
			Whitelist          []*utils.ResourceDescriptor `json:"whitelist"`
			Blacklist          []*utils.ResourceDescriptor `json:"blacklist"`
			NamespaceWhitelist []string                    `json:"namespaceWhitelist"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "get/workspace-workloads"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) ([]unstructured.Unstructured, error) {
				return self.apiService.GetWorkspaceResources(request.WorkspaceName, request.Whitelist, request.Blacklist, request.NamespaceWhitelist)
			},
		)
	}

	// Deprecated: will be removed in future versions
	RegisterPatternHandler(
		PatternHandle{self, "storage/create-volume"},
		PatternConfig{},
		func(datagram structs.Datagram, request services.NfsVolumeRequest) (bool, error) {
			res := services.CreateMogeniusNfsVolume(self.eventsClient, request)
			_, err := store.AddToAuditLog(datagram, self.logger, res, fmt.Errorf("%s", res.Error), nil, nil)
			if err != nil {
				self.logger.Warn("failed to add event to audit log", "request", request, "error", err)
			}
			if res.Success == false {
				return false, fmt.Errorf("%s", res.Error)
			}
			return true, nil
		},
	)

	// Deprecated: will be removed in future versions
	RegisterPatternHandler(
		PatternHandle{self, "storage/delete-volume"},
		PatternConfig{},
		func(datagram structs.Datagram, request services.NfsVolumeRequest) (bool, error) {
			res := services.DeleteMogeniusNfsVolume(self.eventsClient, request)
			_, err := store.AddToAuditLog(datagram, self.logger, res, fmt.Errorf("%s", res.Error), nil, nil)
			if err != nil {
				self.logger.Warn("failed to add event to audit log", "request", request, "error", err)
			}
			if res.Success == false {
				return false, fmt.Errorf("%s", res.Error)
			}
			return true, nil
		},
	)

	// Deprecated: will be removed in future versions
	RegisterPatternHandler(
		PatternHandle{self, "storage/stats"},
		PatternConfig{},
		func(datagram structs.Datagram, request services.NfsVolumeStatsRequest) (services.NfsVolumeStatsResponse, error) {
			return services.StatsMogeniusNfsVolume(request), nil
		},
	)

	// Deprecated: will be removed in future versions
	RegisterPatternHandler(
		PatternHandle{self, "storage/status"},
		PatternConfig{},
		func(datagram structs.Datagram, request services.NfsStatusRequest) (services.NfsStatusResponse, error) {
			return services.StatusMogeniusNfs(request), nil
		},
	)

	{
		type Request struct {
			Limit         int    `json:"limit" validate:"required"`
			Offset        int    `json:"offset"`
			WorkspaceName string `json:"workspaceName"`
		}

		type Response struct {
			Status  string                 `json:"status"`
			Message string                 `json:"message"`
			Data    store.AuditLogResponse `json:"data"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "audit-log/list"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (Response, error) {
				namespaces := []string{}
				var err error
				clusterwide := true
				if request.WorkspaceName != "" {
					clusterwide = false
					namespaces, err = self.apiService.GetWorkspaceNamespaces(request.WorkspaceName)
					if err != nil {
						return Response{
							Status:  "error",
							Message: fmt.Sprintf("failed to get workspace resources: %s", err.Error()),
							Data: store.AuditLogResponse{
								Data:       []store.AuditLogEntry{},
								TotalCount: 0,
							},
						}, err
					}
				}
				data, size, err := store.ListAuditLog(request.Limit, request.Offset, namespaces, clusterwide)
				if err != nil {
					return Response{
						Status:  "error",
						Message: fmt.Sprintf("failed to list audit log: %s", err.Error()),
						Data: store.AuditLogResponse{
							Data:       []store.AuditLogEntry{},
							TotalCount: 0,
						},
					}, err
				}
				return Response{
					Status:  "success",
					Message: "audit log retrieved successfully",
					Data: store.AuditLogResponse{
						TotalCount: size,
						Data:       data,
					},
				}, nil
			},
		)
	}

	RegisterPatternHandler(
		PatternHandle{self, "live-stream/nodes-traffic"},
		PatternConfig{},
		func(datagram structs.Datagram, request xterm.WsConnectionRequest) (Void, error) {
			go self.xtermService.LiveStreamConnection(request, datagram, self.httpService, self.valkeyClient, []string{})
			return nil, nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "live-stream/nodes-memory"},
		PatternConfig{},
		func(datagram structs.Datagram, request xterm.WsConnectionRequest) (Void, error) {
			go self.xtermService.LiveStreamConnection(request, datagram, self.httpService, self.valkeyClient, []string{})
			return nil, nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "live-stream/nodes-cpu"},
		PatternConfig{},
		func(datagram structs.Datagram, request xterm.WsConnectionRequest) (Void, error) {
			go self.xtermService.LiveStreamConnection(request, datagram, self.httpService, self.valkeyClient, []string{})
			return nil, nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "live-stream/pod-cpu"},
		PatternConfig{},
		func(datagram structs.Datagram, request xterm.WsConnectionRequest) (Void, error) {
			go self.xtermService.LiveStreamConnection(request, datagram, self.httpService, self.valkeyClient, []string{request.PodName})
			return nil, nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "live-stream/pod-memory"},
		PatternConfig{},
		func(datagram structs.Datagram, request xterm.WsConnectionRequest) (Void, error) {
			go self.xtermService.LiveStreamConnection(request, datagram, self.httpService, self.valkeyClient, []string{request.PodName})
			return nil, nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "live-stream/pod-traffic"},
		PatternConfig{},
		func(datagram structs.Datagram, request xterm.WsConnectionRequest) (Void, error) {
			go self.xtermService.LiveStreamConnection(request, datagram, self.httpService, self.valkeyClient, []string{request.PodName})
			return nil, nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "live-stream/workspace-cpu"},
		PatternConfig{},
		func(datagram structs.Datagram, request xterm.WsConnectionRequest) (any, error) {
			podNames, err := self.apiService.GetWorkspacePodsNames(request.Workspace)
			if err != nil {
				return nil, fmt.Errorf("failed to get workspace pods: %w", err)
			}
			go self.xtermService.LiveStreamConnection(request, datagram, self.httpService, self.valkeyClient, podNames)
			return nil, nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "live-stream/workspace-memory"},
		PatternConfig{},
		func(datagram structs.Datagram, request xterm.WsConnectionRequest) (any, error) {
			podNames, err := self.apiService.GetWorkspacePodsNames(request.Workspace)
			if err != nil {
				return nil, fmt.Errorf("failed to get workspace pods: %w", err)
			}
			go self.xtermService.LiveStreamConnection(request, datagram, self.httpService, self.valkeyClient, podNames)
			return nil, nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "live-stream/workspace-traffic"},
		PatternConfig{},
		func(datagram structs.Datagram, request xterm.WsConnectionRequest) (any, error) {
			podNames, err := self.apiService.GetWorkspacePodsNames(request.Workspace)
			if err != nil {
				return nil, fmt.Errorf("failed to get workspace pods: %w", err)
			}
			go self.xtermService.LiveStreamConnection(request, datagram, self.httpService, self.valkeyClient, podNames)
			return nil, nil
		},
	)

	{
		type Request struct {
			Namespace string `json:"namespace" validate:"required"`
			Name      string `json:"name" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "sealed-secret/create-from-existing"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (*unstructured.Unstructured, error) {
				return self.sealedSecret.CreateSealedSecretFromExisting(request.Name, request.Namespace)
			},
		)
	}

	RegisterPatternHandler(
		PatternHandle{self, "sealed-secret/get-certificate"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (*v1.Secret, error) {
			return self.sealedSecret.GetMainSecret()
		},
	)

	// Get live metrics for all nodes from Valkey
	{
		type NodeMetrics struct {
			NodeName string                           `json:"nodeName"`
			Cpu      map[string]any                   `json:"cpu"`
			Memory   map[string]any                   `json:"memory"`
			Traffic  []networkmonitor.PodNetworkStats `json:"traffic"`
		}

		type Response struct {
			Nodes []NodeMetrics `json:"nodes"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "get/nodes-metrics"},
			PatternConfig{},
			func(datagram structs.Datagram, request Void) (Response, error) {
				// Get all nodes from Kubernetes
				nodes, err := self.moKubernetes.GetNodeStats()
				if err != nil {
					return Response{}, fmt.Errorf("failed to get nodes: %w", err)
				}

				result := Response{
					Nodes: make([]NodeMetrics, 0, len(nodes)),
				}

				// For each node, fetch metrics from Valkey
				for _, node := range nodes {
					nodeName := node.Name
					nodeMetrics := NodeMetrics{
						NodeName: nodeName,
						Cpu:      make(map[string]any),
						Memory:   make(map[string]any),
						Traffic:  make([]networkmonitor.PodNetworkStats, 0),
					}

					// Get CPU metrics from Valkey: live-stats:cpu:{nodeName}
					cpuData, err := valkeyclient.GetObjectForKey[map[string]any](
						self.valkeyClient,
						DB_STATS_LIVE_BUCKET_NAME,
						DB_STATS_CPU_NAME,
						nodeName,
					)
					if err == nil && cpuData != nil {
						nodeMetrics.Cpu = *cpuData
					}

					// Get Memory metrics from Valkey: live-stats:memory:{nodeName}
					memData, err := valkeyclient.GetObjectForKey[map[string]any](
						self.valkeyClient,
						DB_STATS_LIVE_BUCKET_NAME,
						DB_STATS_MEMORY_NAME,
						nodeName,
					)
					if err == nil && memData != nil {
						nodeMetrics.Memory = *memData
					}

					// Get Traffic metrics from Valkey: live-stats:traffic:{nodeName}
					trafficData, err := valkeyclient.GetObjectForKey[[]networkmonitor.PodNetworkStats](
						self.valkeyClient,
						DB_STATS_LIVE_BUCKET_NAME,
						DB_STATS_TRAFFIC_NAME,
						nodeName,
					)
					if err == nil && trafficData != nil {
						nodeMetrics.Traffic = *trafficData
					}

					result.Nodes = append(result.Nodes, nodeMetrics)
				}

				return result, nil
			},
		)
	}
}

func (self *socketApi) LoadRequest(datagram *structs.Datagram, data any) error {
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

func (self *socketApi) startMessageHandler() {
	go func() {
		var preparedFileName *string
		var preparedFileRequest *services.FilesUploadRequest
		var openFile *os.File

		messageHandlerSemaphore := make(chan struct{}, 100)

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
				go self.JobServerSendData(self.jobClient, ack)

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
				go self.JobServerSendData(self.jobClient, ack)
				continue
			}

			if self.patternHandlerExists(datagram.Pattern) {
				messageHandlerSemaphore <- struct{}{}
				go func() {
					defer func() {
						<-messageHandlerSemaphore
					}()

					if datagram.Zlib {
						decompressedData, err := utils.TryZlibDecompress(datagram.Payload)
						if err != nil {
							self.logger.Error("failed to decompress payload", "error", err)
							return
						}
						datagram.Payload = decompressedData
					}

					start := time.Now()
					responsePayload := self.ExecuteCommandRequest(datagram)
					executionTime := time.Since(start)
					self.logdatagram(executionTime, datagram)

					sendStart := time.Now()
					compressedData, size, err := utils.TryZlibCompress(responsePayload)
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
					sendTime := time.Since(sendStart)
					self.logPattern(executionTime, sendTime, datagram, size)
				}()
			} else {
				go func() {
					self.logger.Warn("no handler for pattern found", "pattern", datagram.Pattern)

					result := structs.Datagram{
						Id:      datagram.Id,
						Pattern: datagram.Pattern,
						Payload: struct {
							Status  string `json:"status"`
							Message string `json:"message"`
						}{
							Status:  "error",
							Message: fmt.Sprintf("No handler for pattern '%s' found", datagram.Pattern),
						},
						CreatedAt: datagram.CreatedAt,
						Zlib:      false,
					}

					self.JobServerSendData(self.jobClient, result)
				}()
			}
		}
		self.logger.Debug("api messagehandler finished as the websocket client was terminated")
	}()
}

func (self *socketApi) loadpatternlogger() {
	if len(os.Args) < 2 || os.Args[1] != "cluster" {
		return
	}

	if self.config.Get("MO_LOG_LEVEL") == "mo" {
		self.logLevelMo = true
	} else {
		self.logLevelMo = false
	}

	_, ok := os.LookupEnv("MO_ENABLE_PATTERNLOGGING")
	if ok {
		patternlogpath := "/tmp/patternlogs.jsonl"

		self.logger.Info("patternlogging is enabled", "path", patternlogpath, "args", os.Args)

		patternlog, err := os.Create(patternlogpath)
		assert.Assert(err == nil, err)

		self.patternlog = patternlog
		shutdown.Add(func() {
			err := self.patternlog.Close()
			if err != nil {
				self.logger.Error("failed to close patternlog file", "error", err)
			}
		})
	}
}

func (self *socketApi) logdatagram(executionTime time.Duration, datagram structs.Datagram) {
	if self.patternlog == nil {
		return
	}
	go func() {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		type LogLine struct {
			Time     time.Duration    `json:"time"`
			Datagram structs.Datagram `json:"datagram"`
		}
		ll := LogLine{executionTime, datagram}
		data, err := json.Marshal(ll)
		assert.Assert(err == nil, err)
		jsonline := fmt.Sprintf("%s\n", string(data))
		_, err = self.patternlog.WriteString(jsonline)
		assert.Assert(err == nil, "failed to write patternlog line", err)
	}()
}

func (self *socketApi) logPattern(executionTime time.Duration, sendTime time.Duration, datagram structs.Datagram, size int64) {
	if !self.logLevelMo {
		return
	}
	fmt.Printf("🌐 \033[36m%-50s\033[0m │ exec: \033[33m%5d ms\033[0m │ send: \033[32m%5d ms\033[0m │ size: \033[34m%9s\033[0m │ \033[35m%s\033[0m\n",
		datagram.Pattern,
		executionTime.Milliseconds(),
		sendTime.Milliseconds(),
		utils.BytesToHumanReadable(size),
		datagram.User.Email,
	)
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
	err := jobClient.WriteJSON(datagram)
	if err != nil {
		self.logger.Error("Error sending data to EventServer", "error", err)
	}
}

func (self *socketApi) ExecuteCommandRequest(datagram structs.Datagram) any {
	if patternHandler, ok := self.patternHandler[datagram.Pattern]; ok {
		return patternHandler.Callback(datagram)
	}

	return struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{
		Status:  "error",
		Message: "Pattern not found",
	}
}

func (self *socketApi) upgradeK8sManager(command string) (*structs.Job, error) {
	job := structs.CreateJob(self.eventsClient, "Upgrade mogenius platform", "UPGRADE", "", "", self.logger)
	job.Start(self.eventsClient)
	_, err := kubernetes.UpgradeMyself(self.eventsClient, job, command)
	job.Finish(self.eventsClient)
	return job, err
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
		xterm.GetPreviousLogContent(podCmdConnectionRequest),
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
