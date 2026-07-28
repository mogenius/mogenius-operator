package core

import (
	"bytes"
	"fmt"
	"log/slog"
	"mogenius-operator/src/ai"
	argocd "mogenius-operator/src/argocd"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/dtos"
	"mogenius-operator/src/helm"
	"mogenius-operator/src/kubernetes"
	moMetrics "mogenius-operator/src/metrics"
	"mogenius-operator/src/networkmonitor"
	"mogenius-operator/src/schema"
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
	"strings"
	"sync"
	"time"

	"encoding/json"

	release "helm.sh/helm/v4/pkg/release/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// crdToAuditObject converts a typed mogenius CRD object (Workspace, User,
// Grant) into an unstructured representation so AddToAuditLog can compute
// a diff. Returns nil when obj is nil or conversion fails — the audit
// entry is then written without a diff.
func crdToAuditObject(obj any, kind string, name string) *unstructured.Unstructured {
	if obj == nil {
		return nil
	}
	if v := reflect.ValueOf(obj); v.Kind() == reflect.Pointer && v.IsNil() {
		return nil
	}
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil
	}
	u := &unstructured.Unstructured{Object: content}
	u.SetAPIVersion(v1alpha1.GroupVersion.String())
	u.SetKind(kind)
	if u.GetName() == "" {
		u.SetName(name)
	}
	return u
}

// Compression threshold - only compress responses larger than 1KB
const compressionThreshold = 1024

// messageWorkerCount is the number of dispatch workers per WS read loop.
// Pre-spawned and reused so we don't pay goroutine-creation cost per message
// or risk an unbounded backlog of goroutines waiting on the K8s API.
const messageWorkerCount = 50

type SocketApi interface {
	Link(
		httpService HttpService,
		xtermService XtermService,
		dbstatsModule ValkeyStatsDb,
		apiService Api,
		moKubernetes MoKubernetes,
		sealedSecret SealedSecretManager,
		aiApi AiApi,
		aiWebsocketConnection ai.AiWebsocketConnection,
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
	AddPatternHandlerForClient(
		pattern string,
		config PatternConfig,
		callback func(datagram structs.Datagram) any,
		client websocket.WebsocketClient,
	)
	PatternConfigs() map[string]PatternConfig
	NormalizePatternName(pattern string) string
	AssertPatternsUnique()
	LoadRequest(datagram *structs.Datagram, data any) error
	GetLogger() *slog.Logger
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

	jobClients   []websocket.WebsocketClient
	eventsClient websocket.WebsocketClient

	config       config.ConfigModule
	valkeyClient valkeyclient.ValkeyClient
	dbstats      ValkeyStatsDb

	status     SocketApiStatus
	statusLock sync.RWMutex

	patternlog *os.File

	logLevelMo bool

	// the patternHandler should only be edited on startup
	patternHandlerLock    sync.RWMutex
	patternHandler        map[string]PatternHandler
	httpService           HttpService
	xtermService          XtermService
	apiService            Api
	moKubernetes          MoKubernetes
	sealedSecret          SealedSecretManager
	argocd                argocd.Argocd
	alertmanager          AlertmanagerService
	aiApi                 AiApi
	aiWebsocketConnection ai.AiWebsocketConnection
}

type PatternHandler struct {
	Config   PatternConfig
	Callback func(datagram structs.Datagram) any
	Client   websocket.WebsocketClient
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
	jobClients []websocket.WebsocketClient,
	eventsClient websocket.WebsocketClient,
	valkeyClient valkeyclient.ValkeyClient,
	argocd argocd.Argocd,
	alertmanager AlertmanagerService,
) SocketApi {
	self := &socketApi{}
	self.config = configModule
	self.jobClients = jobClients
	self.eventsClient = eventsClient
	self.logger = logger
	self.patternHandler = map[string]PatternHandler{}
	self.valkeyClient = valkeyClient
	self.status = NewSocketApiStatus()
	self.statusLock = sync.RWMutex{}
	self.argocd = argocd
	self.alertmanager = alertmanager

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
	aiApi AiApi,
	aiWebsocketConnection ai.AiWebsocketConnection,
) {
	assert.Assert(apiService != nil)
	assert.Assert(httpService != nil)
	assert.Assert(xtermService != nil)
	assert.Assert(dbstatsModule != nil)
	assert.Assert(moKubernetes != nil)
	assert.Assert(aiApi != nil)

	self.apiService = apiService
	self.httpService = httpService
	self.xtermService = xtermService
	self.dbstats = dbstatsModule
	self.moKubernetes = moKubernetes
	self.sealedSecret = sealedSecret
	self.aiApi = aiApi
	self.aiWebsocketConnection = aiWebsocketConnection
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
	seen := make(map[string]struct{}, len(patternConfigs))

	for pattern := range patternConfigs {
		normalizedPattern := self.NormalizePatternName(pattern)
		_, exists := seen[normalizedPattern]
		assert.Assert(!exists, "duplicate normalized pattern", normalizedPattern)
		seen[normalizedPattern] = struct{}{}
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
	registerPatternHandlerInternal(handle, config, nil, callback)
}

// RegisterPatternHandlerForClient registers a pattern handler on a specific WebSocket client connection.
func RegisterPatternHandlerForClient[RequestType any, ResponseType any](
	handle PatternHandle,
	config PatternConfig,
	client websocket.WebsocketClient,
	callback func(datagram structs.Datagram, request RequestType) (ResponseType, error),
) {
	assert.Assert(client != nil, "Client has to be given")
	registerPatternHandlerInternal(handle, config, client, callback)
}

func registerPatternHandlerInternal[RequestType any, ResponseType any](
	handle PatternHandle,
	config PatternConfig,
	client websocket.WebsocketClient,
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

	handle.SocketApi.AddPatternHandlerForClient(handle.Pattern, config, func(datagram structs.Datagram) any {
		var data RequestType
		kind := reflect.TypeOf(data).Kind()

		if kind != reflect.Pointer {
			err := handle.SocketApi.LoadRequest(&datagram, &data)
			if err != nil {
				handle.SocketApi.GetLogger().Error("Error while loading request", "datagram", datagram, "error", err)
				return buildResponse(nil, err)
			}
		}

		if kind == reflect.Pointer && datagram.Payload != nil {
			err := handle.SocketApi.LoadRequest(&datagram, &data)
			if err != nil {
				handle.SocketApi.GetLogger().Error("Error while loading request", "datagram", datagram, "error", err)
				return buildResponse(nil, err)
			}
		}

		result, err := callback(datagram, data)

		return buildResponse(result, err)
	}, client)
}

func (self *socketApi) AddPatternHandler(
	pattern string,
	config PatternConfig,
	callback func(datagram structs.Datagram) any,
) {
	self.AddPatternHandlerForClient(pattern, config, callback, nil)
}

func (self *socketApi) AddPatternHandlerForClient(
	pattern string,
	config PatternConfig,
	callback func(datagram structs.Datagram) any,
	client websocket.WebsocketClient,
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
		Client:   client,
	}
}

func (self *socketApi) PatternConfigs() map[string]PatternConfig {
	patterns := map[string]PatternConfig{}

	for pattern, handler := range self.patternHandler {
		patterns[pattern] = handler.Config
	}

	return patterns
}

func (self *socketApi) GetLogger() *slog.Logger {
	return self.logger
}

func (self *socketApi) registerPatterns() {
	{
		type Response struct {
			BuildInfo struct {
				Version version.Version `json:"version"`
			} `json:"buildInfo"`
			Features struct{}                 `json:"features"`
			Patterns map[string]PatternConfig `json:"patterns,omitempty"`
		}
		RegisterPatternHandler(
			PatternHandle{self, "describe"},
			PatternConfig{},
			func(datagram structs.Datagram, request Void) (Response, error) {
				resp := Response{}
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

		// Dashboards poll this every few seconds. Without caching, every
		// poll triggers a node list, a service list, a country lookup
		// and a Valkey read; under multiple concurrent dashboards this
		// dominated the K8s API call volume.
		clusterResourceInfoCache := utils.NewTTLCache(5*time.Second, func() ClusterResourceInfo {
			errors := []string{}
			nodeStats, nodeErr := self.moKubernetes.GetNodeStats()
			if nodeErr != nil {
				errors = append(errors, nodeErr.Error())
			}
			loadBalancerExternalIps := kubernetes.GetClusterExternalIps()
			country, _ := utils.GuessClusterCountry()
			cniConfig, _ := self.dbstats.GetCniData()
			return ClusterResourceInfo{
				NodeStats:               nodeStats,
				LoadBalancerExternalIps: loadBalancerExternalIps,
				Country:                 country,
				Provider:                string(utils.ClusterProviderCached),
				CniConfig:               cniConfig,
				Errors:                  errors,
			}
		})

		RegisterPatternHandler(
			PatternHandle{self, "cluster/resource-info"},
			PatternConfig{},
			func(datagram structs.Datagram, request Void) (ClusterResourceInfo, error) {
				return clusterResourceInfoCache.Get(), nil
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
				job, err := self.upgradeK8sManager(request.Command)
				return store.AddToAuditLog(datagram, self.logger, job, err, nil, nil)
			},
		)
	}

	RegisterPatternHandler(
		PatternHandle{self, "cluster/force-reconnect"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (bool, error) {
			return kubernetes.ClusterForceReconnect(), nil
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/force-disconnect"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (bool, error) {
			return kubernetes.ClusterForceDisconnect(), nil
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

		// stats/pod/all-for-workspace — full per-pod CPU/memory
		// snapshots for every pod in the namespace (no top-N cap,
		// unlike the utilization aggregations above). Scans valkey
		// directly for every pod-stats stream key in the namespace
		// rather than going through the Workspace CRD, so it also
		// works for namespaces that aren't wired up as mogenius
		// workspaces. Each stream key is
		// `pod-stats:<namespace>:<controllerName>`; the data inside
		// already carries the pod name per entry.
		RegisterPatternHandler(
			PatternHandle{self, "stats/pod/all-for-workspace"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) ([]structs.PodStats, error) {
				if request.TimeOffsetMinutes <= 0 {
					request.TimeOffsetMinutes = 5
				}
				prefix := DB_STATS_POD_STATS_BUCKET_NAME + ":" + request.WorkspaceName + ":"
				keys, err := self.valkeyClient.Keys(prefix + "*")
				if err != nil {
					return nil, err
				}
				out := make([]structs.PodStats, 0)
				for _, k := range keys {
					if !strings.HasPrefix(k, prefix) {
						continue
					}
					controllerName := k[len(prefix):]
					if controllerName == "" {
						continue
					}
					entries := self.dbstats.GetPodStatsEntriesForController(
						"", controllerName, request.WorkspaceName,
						int64(request.TimeOffsetMinutes),
					)
					if entries != nil {
						out = append(out, *entries...)
					}
				}
				return out, nil
			},
		)
	}

	// cluster/dashboard-stats — aggregated metrics for the organization dashboard
	{
		RegisterPatternHandler(
			PatternHandle{self, "cluster/dashboard-stats"},
			PatternConfig{},
			func(datagram structs.Datagram, request Void) (ClusterDashboardStats, error) {
				workspaces, err := self.apiService.GetAllWorkspaces()
				if err != nil {
					return ClusterDashboardStats{}, fmt.Errorf("get workspaces: %w", err)
				}

				names := make([]string, len(workspaces))
				for i, ws := range workspaces {
					names[i] = ws.Name
				}

				return self.dbstats.GetClusterDashboardStats(names, self.apiService.GetWorkspaceControllers)
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
			return ExecutePrometheusQuery(request, self.config, self.logger)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "prometheus/status"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (dtos.ComponentStatus, error) {
			return PrometheusStatus(self.config, self.logger)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "prometheus/is-reachable"},
		PatternConfig{},
		func(datagram structs.Datagram, request PrometheusRequest) (bool, error) {
			data, err := IsPrometheusReachable(request, self.config, self.logger)
			return data, err
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "prometheus/values"},
		PatternConfig{},
		func(datagram structs.Datagram, request PrometheusRequest) ([]string, error) {
			return PrometheusValues(request, self.config, self.logger)
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
		PatternHandle{self, "alertmanager/status"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (dtos.ComponentStatus, error) {
			return self.alertmanager.Status()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "alertmanager/is-reachable"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) (bool, error) {
			return self.alertmanager.IsReachable()
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "alertmanager/alerts/list"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) ([]Alert, error) {
			result, err := self.alertmanager.GetAlerts()
			return result, err
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "alertmanager/alerts/create"},
		PatternConfig{},
		func(datagram structs.Datagram, request []SendAlertRequest) (string, error) {
			err := self.alertmanager.SendAlert(request)
			result := ""
			if err == nil {
				result = "Alerts sent successfully"
			}
			return store.AddToAuditLog(datagram, self.logger, result, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "alertmanager/silences/create"},
		PatternConfig{},
		func(datagram structs.Datagram, request SilenceRequest) (string, error) {
			result, err := self.alertmanager.SilenceAlert(request)
			return store.AddToAuditLog(datagram, self.logger, result, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "alertmanager/silences/list"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) ([]Silence, error) {
			result, err := self.alertmanager.GetSilences()
			return result, err
		},
	)

	{
		type Request struct {
			SilenceID string `json:"silenceId" validate:"required"`
		}
		RegisterPatternHandler(
			PatternHandle{self, "alertmanager/silences/delete"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (string, error) {
				err := self.alertmanager.DeleteSilence(request.SilenceID)
				result := ""
				if err == nil {
					result = "Silence deleted successfully"
				}
				return store.AddToAuditLog(datagram, self.logger, result, err, nil, nil)
			},
		)
	}

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
			result, err := helm.HelmRepoPatch(request)
			return store.AddToAuditLog(datagram, self.logger, result, err, nil, nil)
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
		PatternHandle{self, "cluster/helm-release-list-paginated"},
		PatternConfig{},
		func(datagram structs.Datagram, request helm.HelmReleaseListPaginatedRequest) (helm.HelmReleaseListPaginatedResponse, error) {
			// Empty workspace name = cluster-wide (no filter). A workspace name
			// scopes the result to that workspace's registered helm releases AND
			// Argo applications, resolved here from the Workspace CRD so the
			// operator can filter before paginating.
			var scope *helm.HelmWorkspaceScope
			if request.WorkspaceName != "" {
				workspace, err := self.apiService.GetWorkspace(request.WorkspaceName)
				if err != nil {
					return helm.HelmReleaseListPaginatedResponse{Items: []*helm.HelmRelease{}, TotalCount: 0}, err
				}
				allowed := make(map[string]struct{})
				for _, resource := range workspace.Resources {
					// "helm" keys on (install namespace, release name); "argocd"
					// keys on (Argo install namespace, release name). Both map to
					// the same WorkspaceHelmKey the paginator checks.
					if resource.Type == "helm" || resource.Type == "argocd" {
						allowed[helm.WorkspaceHelmKey(resource.Namespace, resource.Id)] = struct{}{}
					}
				}
				scope = &helm.HelmWorkspaceScope{Allowed: allowed}
			}

			// Argo-CD-managed charts are not helm releases (no helm secret), so
			// the operator lists them separately and merges them into the same
			// sorted/paginated result (MOG-4394). A failure here must not break
			// the (far more important) real release listing.
			argoItems := self.argoHelmReleaseItems()

			return helm.HelmReleaseListPaginated(request, scope, argoItems)
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
		func(datagram structs.Datagram, request helm.HelmReleaseHistoryRequest) ([]release.Release, error) {
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
			result, err := self.argocd.ArgoCdCreateApiToken(request)
			return store.AddToAuditLog(datagram, self.logger, result, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/argo-cd-application-refresh"},
		PatternConfig{},
		func(datagram structs.Datagram, request argocd.ArgoCdApplicationRefreshRequest) (bool, error) {
			result, err := self.argocd.ArgoCdApplicationRefresh(request)
			return store.AddToAuditLog(datagram, self.logger, result, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/argo-cd-application-hard-refresh"},
		PatternConfig{},
		func(datagram structs.Datagram, request argocd.ArgoCdApplicationRefreshRequest) (bool, error) {
			result, err := self.argocd.ArgoCdApplicationHardRefresh(request)
			return store.AddToAuditLog(datagram, self.logger, result, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/argo-cd-application-sync"},
		PatternConfig{},
		func(datagram structs.Datagram, request argocd.ArgoCdApplicationSyncRequest) (bool, error) {
			result, err := self.argocd.ArgoCdApplicationSync(request)
			return store.AddToAuditLog(datagram, self.logger, result, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/argo-cd-application-terminate-operation"},
		PatternConfig{},
		func(datagram structs.Datagram, request argocd.ArgoCdApplicationTerminateOperationRequest) (bool, error) {
			result, err := self.argocd.ArgoCdApplicationTerminateOperation(request)
			return store.AddToAuditLog(datagram, self.logger, result, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "cluster/argo-cd-resource-action"},
		PatternConfig{},
		func(datagram structs.Datagram, request argocd.ArgoCdResourceActionRequest) (bool, error) {
			result, err := self.argocd.ArgoCdResourceAction(request)
			return store.AddToAuditLog(datagram, self.logger, result, err, nil, nil)
		},
	)

	RegisterPatternHandler(
		PatternHandle{self, "service/port-forward-connection-request"},
		PatternConfig{},
		func(datagram structs.Datagram, request xterm.PortForwardConnectionRequest) (Void, error) {
			go xterm.PortForwardStreamConnection(request)
			// Same treatment as exec-sh below: interactive cluster access
			// must leave an audit trail of who opened a tunnel to which pod.
			_, err := store.AddToAuditLog(datagram, self.logger, any(nil), nil, nil, nil)
			if err != nil {
				self.logger.Warn("failed to add event to audit log", "request", request, "error", err)
			}
			return nil, nil
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

	{
		RegisterPatternHandler(
			PatternHandle{self, "get/workload-list-paginated"},
			PatternConfig{},
			func(datagram structs.Datagram, request ResourcesPaginatedRequest) (ResourcesPaginatedResponse, error) {
				return self.apiService.GetResourceListByWhitelistPaginated(request)
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
		PatternHandle{self, "describe/workload"},
		PatternConfig{},
		func(datagram structs.Datagram, request utils.WorkloadSingleRequest) (string, error) {
			// Read operation — no audit log entry (see get/workload): auditing
			// reads spams the per-resource entry budget.
			return kubernetes.DescribeUnstructuredResource(request.ApiVersion, request.Plural, request.Namespace, request.ResourceName)
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
				self.logger.Warn("failed to add event to audit log", "request", request, "error", auditErr)
			}
			if err != nil {
				return nil, err
			}
			// Check if resource still exists with deletionTimestamp (blocked by finalizers)
			obj, getErr := kubernetes.GetUnstructuredResource(request.ApiVersion, request.Plural, request.Namespace, request.ResourceName)
			if getErr != nil {
				if apierrors.IsNotFound(getErr) {
					// Resource is fully deleted
					return nil, nil
				}
				// Other error (network, etc.) - log but don't fail the delete response
				self.logger.Warn("could not verify resource deletion status", "error", getErr)
				return nil, nil
			}
			// Resource still exists - check if it's terminating
			if obj.GetDeletionTimestamp() != nil {
				if blocking := kubernetes.BlockingFinalizers(obj.GetFinalizers()); len(blocking) > 0 {
					return nil, fmt.Errorf("resource is terminating but blocked by finalizers: %v", blocking)
				}
				// Deletion accepted; grace periods and GC finalizers resolve asynchronously.
				return nil, nil
			}
			return nil, nil
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

	{
		type PodLogsRequest struct {
			Namespace string `json:"namespace" validate:"required"`
			PodName   string `json:"podName" validate:"required"`
			Container string `json:"container"`
			TailLines int64  `json:"tailLines"`
			Previous  bool   `json:"previous"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "get/workload/pod-logs"},
			PatternConfig{},
			func(datagram structs.Datagram, request PodLogsRequest) (string, error) {
				tailLines := request.TailLines
				if tailLines <= 0 {
					tailLines = 100
				}
				return kubernetes.GetPodLogs(request.Namespace, request.PodName, request.Container, tailLines, request.Previous)
			},
		)
	}

	{
		type PodEventsRequest struct {
			Namespace string `json:"namespace" validate:"required"`
			PodName   string `json:"podName" validate:"required"`
		}

		type PodEvent struct {
			Timestamp string `json:"timestamp"`
			Reason    string `json:"reason"`
			Message   string `json:"message"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "get/workload/pod-events"},
			PatternConfig{},
			func(datagram structs.Datagram, request PodEventsRequest) ([]PodEvent, error) {
				data, err := self.valkeyClient.List(50, "resources", "v1", "Event", request.Namespace, request.PodName+"*")
				if err != nil {
					return nil, err
				}

				var events []PodEvent
				for _, item := range data {
					var event struct {
						ObjectMeta struct {
							CreationTimestamp string `json:"creationTimestamp"`
						} `json:"metadata"`
						Reason  string `json:"reason"`
						Message string `json:"message"`
					}
					if err := json.Unmarshal([]byte(item), &event); err != nil {
						continue
					}
					events = append(events, PodEvent{
						Timestamp: event.ObjectMeta.CreationTimestamp,
						Reason:    event.Reason,
						Message:   event.Message,
					})
				}
				return events, nil
			},
		)
	}

	RegisterPatternHandler(
		PatternHandle{self, "get/workspaces"},
		PatternConfig{},
		func(datagram structs.Datagram, request Void) ([]GetWorkspaceResult, error) {
			namespace := self.config.Get("MO_OWN_NAMESPACE")
			workspaces, err := store.GetAllWorkspaces(namespace)
			if err != nil {
				return []GetWorkspaceResult{}, err
			}
			result := make([]GetWorkspaceResult, 0, len(workspaces))
			for _, v := range workspaces {
				result = append(result, GetWorkspaceResult{
					Name:              v.Name,
					CreationTimestamp: v.CreationTimestamp,
					Resources:         v.Spec.Resources,
					DashboardRef:      v.Spec.DashboardRef,
				})
			}
			return result, err
		},
	)

	{
		type Request struct {
			Name         string                                 `json:"name" validate:"required"`
			DisplayName  string                                 `json:"displayName"`
			Resources    []v1alpha1.WorkspaceResourceIdentifier `json:"resources" validate:"required"`
			DashboardRef string                                 `json:"dashboardRef"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "create/workspace"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (string, error) {
				spec := v1alpha1.NewWorkspaceSpec(request.DisplayName, request.Resources, request.DashboardRef)
				res, err := self.apiService.CreateWorkspace(request.Name, spec)
				var created *unstructured.Unstructured
				if err == nil {
					created = crdToAuditObject(&v1alpha1.Workspace{
						ObjectMeta: metav1.ObjectMeta{Name: request.Name},
						Spec:       spec,
					}, "Workspace", request.Name)
				}
				return store.AddToAuditLog(datagram, self.logger, res, err, nil, created)
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
				result := NewGetWorkspaceResult(workspace.Name, workspace.CreationTimestamp, workspace.Spec.Resources, workspace.Spec.DashboardRef)
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
				result, err := self.moKubernetes.CleanUp(
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
				if request.DryRun {
					// Dry runs delete nothing — auditing them would only drown
					// out the real clean-up entries.
					return result, err
				}
				return store.AddToAuditLog(datagram, self.logger, result, err, nil, nil)
			},
		)
	}

	{
		type Request struct {
			Name         string                                 `json:"name" validate:"required"`
			DisplayName  string                                 `json:"displayName"`
			Resources    []v1alpha1.WorkspaceResourceIdentifier `json:"resources" validate:"required"`
			DashboardRef string                                 `json:"dashboardRef"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "update/workspace"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (string, error) {
				spec := v1alpha1.NewWorkspaceSpec(request.DisplayName, request.Resources, request.DashboardRef)
				oldWorkspace, _ := store.GetWorkspace(self.config.Get("MO_OWN_NAMESPACE"), request.Name)
				res, err := self.apiService.UpdateWorkspace(request.Name, spec)
				var oldObj, newObj *unstructured.Unstructured
				if oldWorkspace != nil {
					oldObj = crdToAuditObject(oldWorkspace, "Workspace", request.Name)
					updated := oldWorkspace.DeepCopy()
					updated.Spec = spec
					newObj = crdToAuditObject(updated, "Workspace", request.Name)
				}
				return store.AddToAuditLog(datagram, self.logger, res, err, oldObj, newObj)
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
				oldWorkspace, _ := store.GetWorkspace(self.config.Get("MO_OWN_NAMESPACE"), request.Name)
				res, err := self.apiService.DeleteWorkspace(request.Name)
				return store.AddToAuditLog(datagram, self.logger, res, err, crdToAuditObject(oldWorkspace, "Workspace", request.Name), nil)
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
				spec := v1alpha1.NewUserSpec(request.FirstName, request.LastName, request.Email, request.Subject)
				res, err := self.apiService.CreateUser(request.Name, spec)
				var created *unstructured.Unstructured
				if err == nil {
					created = crdToAuditObject(&v1alpha1.User{
						ObjectMeta: metav1.ObjectMeta{Name: request.Name},
						Spec:       spec,
					}, "User", request.Name)
				}
				return store.AddToAuditLog(datagram, self.logger, res, err, nil, created)
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
				spec := v1alpha1.NewUserSpec(request.FirstName, request.LastName, request.Email, request.Subject)
				oldUser, _ := self.apiService.GetUser(request.Name)
				res, err := self.apiService.UpdateUser(request.Name, spec)
				var oldObj, newObj *unstructured.Unstructured
				if oldUser != nil {
					oldObj = crdToAuditObject(oldUser, "User", request.Name)
					updated := oldUser.DeepCopy()
					updated.Spec = spec
					newObj = crdToAuditObject(updated, "User", request.Name)
				}
				return store.AddToAuditLog(datagram, self.logger, res, err, oldObj, newObj)
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
				oldUser, _ := self.apiService.GetUser(request.Name)
				res, err := self.apiService.DeleteUser(request.Name)
				return store.AddToAuditLog(datagram, self.logger, res, err, crdToAuditObject(oldUser, "User", request.Name), nil)
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
				spec := v1alpha1.NewGrantSpec(request.Grantee, request.TargetType, request.TargetName, request.Role)
				res, err := self.apiService.CreateGrant(request.Name, spec)
				var created *unstructured.Unstructured
				if err == nil {
					created = crdToAuditObject(&v1alpha1.Grant{
						ObjectMeta: metav1.ObjectMeta{Name: request.Name},
						Spec:       spec,
					}, "Grant", request.Name)
				}
				return store.AddToAuditLog(datagram, self.logger, res, err, nil, created)
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
				spec := v1alpha1.NewGrantSpec(request.Grantee, request.TargetType, request.TargetName, request.Role)
				oldGrant, _ := self.apiService.GetGrant(request.Name)
				res, err := self.apiService.UpdateGrant(request.Name, spec)
				var oldObj, newObj *unstructured.Unstructured
				if oldGrant != nil {
					oldObj = crdToAuditObject(oldGrant, "Grant", request.Name)
					updated := oldGrant.DeepCopy()
					updated.Spec = spec
					newObj = crdToAuditObject(updated, "Grant", request.Name)
				}
				return store.AddToAuditLog(datagram, self.logger, res, err, oldObj, newObj)
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
				oldGrant, _ := self.apiService.GetGrant(request.Name)
				res, err := self.apiService.DeleteGrant(request.Name)
				return store.AddToAuditLog(datagram, self.logger, res, err, crdToAuditObject(oldGrant, "Grant", request.Name), nil)
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

	{
		// Paginated workspace workloads. Offset/limit lives here, dedup
		// (by metadata.uid) and stable sort happen in the api layer so
		// the slice boundary is deterministic. Older callers continue
		// to use get/workspace-workloads above; this pattern is
		// strictly additive.
		// WorkspaceName is intentionally not "required": the Studio cluster
		// view calls this pattern with an empty workspace to fetch every
		// resource matching the whitelist cluster-wide. The api layer
		// (GetWorkspaceResources) maps "" -> cluster-wide fetch.
		type PaginatedRequest struct {
			WorkspaceName      string                      `json:"workspaceName"`
			Whitelist          []*utils.ResourceDescriptor `json:"whitelist"`
			Blacklist          []*utils.ResourceDescriptor `json:"blacklist"`
			NamespaceWhitelist []string                    `json:"namespaceWhitelist"`
			Offset             int                         `json:"offset"`
			Limit              int                         `json:"limit"`
			SortBy             string                      `json:"sortBy"`
			SortOrder          string                      `json:"sortOrder"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "get/workspace-workloads-paginated"},
			PatternConfig{},
			func(datagram structs.Datagram, request PaginatedRequest) (WorkspaceResourcesPaginatedResponse, error) {
				return self.apiService.GetWorkspaceResourcesPaginated(request.WorkspaceName, WorkspaceResourcesPaginatedRequest{
					Whitelist:          request.Whitelist,
					Blacklist:          request.Blacklist,
					NamespaceWhitelist: request.NamespaceWhitelist,
					Offset:             request.Offset,
					Limit:              request.Limit,
					SortBy:             request.SortBy,
					SortOrder:          request.SortOrder,
				})
			},
		)
	}

	{
		type Request struct {
			AiPromptConfig ai.AiPromptConfig `json:"aiPromptConfig" validate:"required"`
			AiPrompts      ai.AiPrompts      `json:"aiPrompts" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "aiManager/inject-prompt-config"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (Void, error) {
				// Filters in the payload are tolerated for backward
				// compatibility but no longer drive any task creation —
				// event triggers now live on Agent CRs.
				self.aiApi.InjectAiPromptConfig(request.AiPromptConfig, &request.AiPrompts)
				return store.AddToAuditLog[Void](datagram, self.logger, nil, nil, nil, nil)
			},
		)

		RegisterPatternHandler(
			PatternHandle{self, "aiManager/get/prompt-config"},
			PatternConfig{},
			func(datagram structs.Datagram, request Void) (*ai.AiPromptConfig, error) {
				return self.aiApi.GetPromptConfig()
			},
		)
	}

	{
		type Request struct {
			Name string `json:"name"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "get/agents"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) ([]GetAgentResult, error) {
				if request.Name != "" {
					agent, err := self.apiService.GetAgent(request.Name)
					if err != nil || agent == nil {
						return []GetAgentResult{}, err
					}
					return []GetAgentResult{*agent}, nil
				}
				return self.apiService.GetAllAgents()
			},
		)
	}

	{
		type Request struct {
			Name string             `json:"name" validate:"required"`
			Spec v1alpha1.AgentSpec `json:"spec" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "create/agent"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (string, error) {
				var res string
				err := ai.ValidateAgentSpec(request.Spec)
				if err == nil {
					res, err = self.apiService.CreateAgent(request.Name, request.Spec)
				}
				var created *unstructured.Unstructured
				if err == nil {
					created = crdToAuditObject(&v1alpha1.Agent{
						ObjectMeta: metav1.ObjectMeta{Name: request.Name},
						Spec:       request.Spec,
					}, "Agent", request.Name)
				}
				return store.AddToAuditLog(datagram, self.logger, res, err, nil, created)
			},
		)

		RegisterPatternHandler(
			PatternHandle{self, "update/agent"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (string, error) {
				oldAgent, _ := store.GetAgent(self.config.Get("MO_OWN_NAMESPACE"), request.Name)
				var res string
				err := ai.ValidateAgentSpec(request.Spec)
				if err == nil {
					res, err = self.apiService.UpdateAgent(request.Name, request.Spec)
				}
				var oldObj, newObj *unstructured.Unstructured
				if oldAgent != nil {
					oldObj = crdToAuditObject(oldAgent, "Agent", request.Name)
					updated := oldAgent.DeepCopy()
					updated.Spec = request.Spec
					newObj = crdToAuditObject(updated, "Agent", request.Name)
				}
				return store.AddToAuditLog(datagram, self.logger, res, err, oldObj, newObj)
			},
		)
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "delete/agent"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (string, error) {
				oldAgent, _ := store.GetAgent(self.config.Get("MO_OWN_NAMESPACE"), request.Name)
				res, err := self.apiService.DeleteAgent(request.Name)
				return store.AddToAuditLog(datagram, self.logger, res, err, crdToAuditObject(oldAgent, "Agent", request.Name), nil)
			},
		)
	}

	{
		RegisterPatternHandler(
			PatternHandle{self, "get/aimodel-sdks"},
			PatternConfig{},
			func(datagram structs.Datagram, request Void) ([]ai.AiSdkInfo, error) {
				return ai.SupportedAiSdks(), nil
			},
		)
	}

	{
		type Request struct {
			Name string `json:"name"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "get/aimodels"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) ([]GetAiModelResult, error) {
				if request.Name != "" {
					model, err := self.apiService.GetAiModel(request.Name)
					if err != nil || model == nil {
						return []GetAiModelResult{}, err
					}
					return []GetAiModelResult{*model}, nil
				}
				return self.apiService.GetAllAiModels()
			},
		)
	}

	{
		type Request struct {
			Name string               `json:"name" validate:"required"`
			Spec v1alpha1.AiModelSpec `json:"spec" validate:"required"`
			// Optional plaintext API key; the operator provisions a managed
			// Secret from it and wires spec.apiKeySecretRef. The field name
			// must stay in sensitiveAuditPayloadKeys (store) so the audit log
			// redacts it.
			ApiKey string `json:"apiKey"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "create/aimodel"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (string, error) {
				request.Spec = ai.NormalizeAiModelSpec(request.Spec)
				var res string
				// The managed ref must be wired before validation: for
				// openai/anthropic the spec is only valid with a secret ref.
				err := applyManagedApiKeyRef(request.Name, &request.Spec, request.ApiKey)
				if err == nil {
					err = ai.ValidateAiModelSpec(request.Spec)
				}
				if err == nil {
					res, err = self.apiService.CreateAiModel(request.Name, request.Spec, request.ApiKey)
				}
				var created *unstructured.Unstructured
				if err == nil {
					created = crdToAuditObject(&v1alpha1.AiModel{
						ObjectMeta: metav1.ObjectMeta{Name: request.Name},
						Spec:       request.Spec,
					}, "AiModel", request.Name)
				}
				return store.AddToAuditLog(datagram, self.logger, res, err, nil, created)
			},
		)

		RegisterPatternHandler(
			PatternHandle{self, "update/aimodel"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (string, error) {
				request.Spec = ai.NormalizeAiModelSpec(request.Spec)
				oldModel, _ := store.GetAiModel(self.config.Get("MO_OWN_NAMESPACE"), request.Name)
				var res string
				err := applyManagedApiKeyRef(request.Name, &request.Spec, request.ApiKey)
				if err == nil {
					err = ai.ValidateAiModelSpec(request.Spec)
				}
				if err == nil {
					res, err = self.apiService.UpdateAiModel(request.Name, request.Spec, request.ApiKey)
				}
				var oldObj, newObj *unstructured.Unstructured
				if oldModel != nil {
					oldObj = crdToAuditObject(oldModel, "AiModel", request.Name)
					updated := oldModel.DeepCopy()
					updated.Spec = request.Spec
					newObj = crdToAuditObject(updated, "AiModel", request.Name)
				}
				return store.AddToAuditLog(datagram, self.logger, res, err, oldObj, newObj)
			},
		)
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "delete/aimodel"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (string, error) {
				oldModel, _ := store.GetAiModel(self.config.Get("MO_OWN_NAMESPACE"), request.Name)
				res, err := self.apiService.DeleteAiModel(request.Name)
				return store.AddToAuditLog(datagram, self.logger, res, err, crdToAuditObject(oldModel, "AiModel", request.Name), nil)
			},
		)
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "test/aimodel"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (*ai.AiModelTestResult, error) {
				return self.aiApi.TestAiModel(request.Name)
			},
		)
	}

	{
		type Request struct {
			AgentName string `json:"agentName" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "aiManager/trigger/agent"},
			PatternConfig{NeedsUser: true},
			func(datagram structs.Datagram, request Request) (string, error) {
				// GitOps-native: request a run by bumping the agent's
				// run-request annotation. The agent reconciler enqueues the
				// actual run within the informer's latency; the new task then
				// arrives via the normal AI event stream.
				res, err := self.apiService.RequestAgentRun(request.AgentName)
				return store.AddToAuditLog(datagram, self.logger, res, err, nil, nil)
			},
		)
	}

	{
		type Request struct {
			TaskId string `json:"taskId" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "aiManager/approve/task"},
			PatternConfig{NeedsUser: true},
			func(datagram structs.Datagram, request Request) (*ai.AiTask, error) {
				task, err := self.aiApi.ApproveTask(request.TaskId, datagram.User, datagram.Workspace)
				return store.AddToAuditLog(datagram, self.logger, task, err, nil, nil)
			},
		)
	}

	{
		type Request struct {
			TaskId string `json:"taskId" validate:"required"`
			Reason string `json:"reason"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "aiManager/reject/task"},
			PatternConfig{NeedsUser: true},
			func(datagram structs.Datagram, request Request) (*ai.AiTask, error) {
				task, err := self.aiApi.RejectTask(request.TaskId, datagram.User, request.Reason)
				return store.AddToAuditLog(datagram, self.logger, task, err, nil, nil)
			},
		)
	}

	{
		type Request struct {
			TaskId string `json:"taskId" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "aiManager/cancel/task"},
			PatternConfig{NeedsUser: true},
			func(datagram structs.Datagram, request Request) (*ai.AiTask, error) {
				task, err := self.aiApi.CancelTask(request.TaskId, datagram.User)
				return store.AddToAuditLog(datagram, self.logger, task, err, nil, nil)
			},
		)
	}

	{
		type Request struct {
			TaskId string `json:"taskId" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "aiManager/delete/task"},
			PatternConfig{NeedsUser: true},
			func(datagram structs.Datagram, request Request) (*ai.AiTask, error) {
				task, err := self.aiApi.DeleteTask(request.TaskId, datagram.User)
				return store.AddToAuditLog(datagram, self.logger, task, err, nil, nil)
			},
		)
	}

	{

		type Request struct {
			Workspace *string `json:"workspace"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "aiManager/status"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (ai.AiManagerStatus, error) {
				return self.aiApi.GetStatus(request.Workspace), nil
			},
		)
	}

	{
		RegisterPatternHandler(
			PatternHandle{self, "aiManager/get/models"},
			PatternConfig{},
			func(datagram structs.Datagram, request *ai.ModelsRequest) ([]string, error) {
				return self.aiApi.GetAvailableModels(request)
			},
		)
	}

	{
		type Request struct {
			Name string `json:"name" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "reset/aimodel-usage"},
			PatternConfig{NeedsUser: true},
			func(datagram structs.Datagram, request Request) (string, error) {
				// GitOps-native: request the reset by bumping the model's
				// reset-usage annotation; the AiModel reconciler performs the
				// actual reset (idempotent via status.lastUsageResetAt).
				res, err := self.apiService.RequestAiModelUsageReset(request.Name)
				return store.AddToAuditLog(datagram, self.logger, res, err, nil, nil)
			},
		)
	}

	{
		RegisterPatternHandler(
			PatternHandle{self, "aiManager/delete-all-data"},
			PatternConfig{},
			func(datagram structs.Datagram, request Void) (Void, error) {
				err := self.aiApi.DeleteAllAiData()
				return store.AddToAuditLog[Void](datagram, self.logger, nil, err, nil, nil)
			},
		)
	}

	{
		type Request struct {
			Workspace string `json:"workspace"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "aiManager/get/tasks"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) ([]ai.AiTask, error) {
				var tasks []ai.AiTask
				var err error
				if request.Workspace == "" {
					tasks, err = self.aiApi.GetAllAiTasks()
				} else {
					tasks, err = self.aiApi.GetAiTasksForWorkspace(request.Workspace)
				}

				return tasks, err
			},
		)
	}

	{
		type Request struct {
			RunId string `json:"runId"`
		}

		// One agent run assembled from its primary task: metadata, the
		// recorded ReAct steps and the IDs of all finding tasks.
		RegisterPatternHandler(
			PatternHandle{self, "aiManager/get/run"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (*ai.AiRun, error) {
				if request.RunId == "" {
					return nil, fmt.Errorf("runId is required")
				}
				return self.aiApi.GetRun(request.RunId)
			},
		)
	}

	{
		RegisterPatternHandler(
			PatternHandle{self, "aiManager/detail/tasks"},
			PatternConfig{},
			func(datagram structs.Datagram, request utils.WorkloadSingleRequest) ([]ai.AiTask, error) {
				return self.aiApi.GetAiTasksForResource(request)
			},
		)
	}

	{
		type Request struct {
			TaskId string `json:"taskId" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "aiManager/read/task"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (Void, error) {
				err := self.aiApi.UpdateTaskReadState(request.TaskId, &datagram.User)
				return store.AddToAuditLog[Void](datagram, self.logger, nil, err, nil, nil)
			},
		)
	}

	{
		type Request struct {
			TaskId string         `json:"taskId" validate:"required"`
			State  ai.AiTaskState `json:"state" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "aiManager/update/task"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (Void, error) {
				err := self.aiApi.UpdateTaskState(request.TaskId, request.State)
				return store.AddToAuditLog[Void](datagram, self.logger, nil, err, nil, nil)
			},
		)
	}

	{
		type Request struct {
			Workspace *string `json:"workspace"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "aiManager/latest/task"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (*ai.AiTaskLatest, error) {
				return self.aiApi.GetLatestTask(request.Workspace)
			},
		)
	}

	// Deprecated: will be removed in future versions
	RegisterPatternHandler(
		PatternHandle{self, "storage/create-volume"},
		PatternConfig{},
		func(datagram structs.Datagram, request services.NfsVolumeRequest) (bool, error) {
			res := services.CreateMogeniusNfsVolume(self.eventsClient, request)
			var resErr error
			if !res.Success {
				resErr = fmt.Errorf("%s", res.Error)
			}
			_, auditErr := store.AddToAuditLog(datagram, self.logger, res, resErr, nil, nil)
			if auditErr != nil {
				self.logger.Warn("failed to add event to audit log", "request", request, "error", auditErr)
			}
			if resErr != nil {
				return false, resErr
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
			var resErr error
			if !res.Success {
				resErr = fmt.Errorf("%s", res.Error)
			}
			_, auditErr := store.AddToAuditLog(datagram, self.logger, res, resErr, nil, nil)
			if auditErr != nil {
				self.logger.Warn("failed to add event to audit log", "request", request, "error", auditErr)
			}
			if resErr != nil {
				return false, resErr
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
			Search        string `json:"search"`
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
				data, size, err := store.ListAuditLog(request.Limit, request.Offset, namespaces, clusterwide, request.WorkspaceName, request.Search)
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

		RegisterPatternHandler(
			PatternHandle{self, "live-stream/ai-manager-chat-request"},
			PatternConfig{},
			func(datagram structs.Datagram, request ai.ChatRequest) (Void, error) {
				go self.aiWebsocketConnection.LiveStreamAiManagerChatRequest(request, datagram)
				return nil, nil
			},
		)
	}

	{
		type Request struct {
			Namespace string `json:"namespace" validate:"required"`
			Name      string `json:"name" validate:"required"`
		}

		RegisterPatternHandler(
			PatternHandle{self, "sealed-secret/create-from-existing"},
			PatternConfig{},
			func(datagram structs.Datagram, request Request) (*unstructured.Unstructured, error) {
				// The created SealedSecret only carries encrypted data; the
				// source Secret itself never enters the audit entry.
				created, err := self.sealedSecret.CreateSealedSecretFromExisting(request.Name, request.Namespace)
				return store.AddToAuditLog(datagram, self.logger, created, err, nil, created)
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

				// Fetch cpu/memory/traffic live-stats for all nodes with a
				// single MGET instead of 3 sequential GETs per node.
				// Dashboards poll this endpoint continuously.
				keys := make([]string, 0, len(nodes)*3)
				for _, node := range nodes {
					keys = append(keys,
						DB_STATS_LIVE_BUCKET_NAME+":"+DB_STATS_CPU_NAME+":"+node.Name,
						DB_STATS_LIVE_BUCKET_NAME+":"+DB_STATS_MEMORY_NAME+":"+node.Name,
						DB_STATS_LIVE_BUCKET_NAME+":"+DB_STATS_TRAFFIC_NAME+":"+node.Name,
					)
				}
				var values []string
				if len(keys) > 0 {
					client := self.valkeyClient.GetValkeyClient()
					values, err = client.Do(self.valkeyClient.GetContext(), client.B().Mget().Key(keys...).Build()).AsStrSlice()
				}
				if err != nil || len(values) != len(keys) {
					// keep prior behaviour: missing metrics stay empty
					values = make([]string, len(keys))
				}

				for i, node := range nodes {
					nodeMetrics := NodeMetrics{
						NodeName: node.Name,
						Cpu:      make(map[string]any),
						Memory:   make(map[string]any),
						Traffic:  make([]networkmonitor.PodNetworkStats, 0),
					}
					if v := values[i*3]; v != "" {
						_ = json.Unmarshal([]byte(v), &nodeMetrics.Cpu)
					}
					if v := values[i*3+1]; v != "" {
						_ = json.Unmarshal([]byte(v), &nodeMetrics.Memory)
					}
					if v := values[i*3+2]; v != "" {
						_ = json.Unmarshal([]byte(v), &nodeMetrics.Traffic)
					}
					result.Nodes = append(result.Nodes, nodeMetrics)
				}

				return result, nil
			},
		)
	}
}

// argoHelmReleaseItems returns the Argo-CD-managed helm charts as pseudo
// helm-release entries to be merged into the paginated release list. Errors are
// logged and swallowed: Argo is optional and must never break the real release
// listing.
func (self *socketApi) argoHelmReleaseItems() []*helm.HelmRelease {
	apps, err := self.argocd.ListHelmReleaseApplications()
	if err != nil {
		self.logger.Warn("failed to list Argo CD applications for helm release list", "error", err.Error())
		return nil
	}
	items := make([]*helm.HelmRelease, 0, len(apps))
	for _, app := range apps {
		items = append(items, helm.NewArgoHelmRelease(app.ReleaseName, &helm.ArgoReleaseInfo{
			Application:     app.Application,
			ParentName:      app.Name,
			ParentNamespace: app.Namespace,
			ValuesObject:    app.ValuesObject,
			ChartName:       app.ChartName,
			Version:         app.TargetRevision,
			DestNamespace:   app.DestNamespace,
			RepoName:        app.RepoName,
			CreatedAt:       app.CreatedAt,
		}))
	}
	return items
}

func (self *socketApi) LoadRequest(datagram *structs.Datagram, data any) error {
	var payloadBytes []byte
	if raw, ok := datagram.Payload.(json.RawMessage); ok {
		// ParseDatagram already captured the payload as raw JSON; decode it
		// straight into the typed request without a re-marshal.
		payloadBytes = raw
	} else {
		marshaled, err := json.Marshal(datagram.Payload)
		if err != nil {
			datagram.Err = err.Error()
			return err
		}
		payloadBytes = marshaled
	}

	if err := json.Unmarshal(payloadBytes, data); err != nil {
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
	// Start the main read loop for the first jobClient (includes file upload support)
	self.startJobClientReadLoop()

	// Start a read loop for each additional jobClient
	for i := 1; i < len(self.jobClients); i++ {
		self.startClientReadLoop(self.jobClients[i])
	}
}

// startClientReadLoop starts a message read loop for a non-default WebSocket client.
// It only dispatches patterns that are explicitly registered for this client.
func (self *socketApi) startClientReadLoop(client websocket.WebsocketClient) {
	go func() {
		jobs := make(chan structs.Datagram, messageWorkerCount)
		defer close(jobs) // signals workers to exit when the read loop ends

		for range messageWorkerCount {
			go self.dispatchClientMessages(client, jobs)
		}

		for !client.IsTerminated() {
			_, message, err := client.ReadMessage()
			if err != nil {
				self.logger.Error("failed to read message from websocket connection", "error", err)
				time.Sleep(time.Second)
				continue
			}
			if len(message) == 0 {
				continue
			}

			datagram, err := self.ParseDatagram(message)
			if err != nil {
				self.logger.Error("failed to parse datagram", "error", err)
				continue
			}

			datagram.DisplayReceiveSummary(self.logger)

			// Blocks when all workers are busy. This backpressures the read
			// loop instead of spawning unbounded goroutines.
			jobs <- datagram
		}
		self.logger.Debug("client messagehandler finished as the websocket client was terminated")
	}()
}

// dispatchClientMessages runs one worker that looks up the handler for each
// incoming datagram and either executes it or replies with a no-handler error.
// Exits when jobs is closed.
func (self *socketApi) dispatchClientMessages(client websocket.WebsocketClient, jobs <-chan structs.Datagram) {
	for datagram := range jobs {
		self.patternHandlerLock.RLock()
		handler, exists := self.patternHandler[datagram.Pattern]
		self.patternHandlerLock.RUnlock()

		if exists && (handler.Client == nil || handler.Client == client) {
			self.handlePatternRequest(datagram, client)
		} else {
			self.sendNoHandlerError(client, datagram, true)
		}
	}
}

// sendNoHandlerError writes a structured error datagram back to the client
// for an unrecognized pattern. perClient indicates whether the warning log
// should mention the specific client (used by the per-client read loop).
func (self *socketApi) sendNoHandlerError(client websocket.WebsocketClient, datagram structs.Datagram, perClient bool) {
	if perClient {
		self.logger.Warn("no handler for pattern found on client", "pattern", datagram.Pattern, "client", self.clientName(client))
	} else {
		self.logger.Warn("no handler for pattern found", "pattern", datagram.Pattern)
	}

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

	self.JobServerSendData(client, result)
}

func (self *socketApi) startJobClientReadLoop() {
	go func() {
		var preparedFileName *string
		var preparedFileRequest *services.FilesUploadRequest
		var openFile *os.File

		jobs := make(chan structs.Datagram, messageWorkerCount)
		defer close(jobs)

		for range messageWorkerCount {
			go self.dispatchJobClientMessages(jobs)
		}

		for !self.jobClients[0].IsTerminated() {
			_, message, err := self.jobClients[0].ReadMessage()
			if err != nil {
				self.logger.Error("failed to read message from websocket connection", "error", err)
				time.Sleep(time.Second) // wait before next attempt to read
				continue
			}
			if len(message) == 0 {
				continue
			}
			if bytes.HasPrefix(message, []byte("######START_UPLOAD######;")) {
				preparedFileName = new(fmt.Sprintf("/tmp/%s.zip", utils.NanoId()))
				openFile, err = os.OpenFile(*preparedFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					self.logger.Error("Cannot open uploadfile", "filename", *preparedFileName, "error", err)
					preparedFileName = nil
					openFile = nil
				}
				continue
			}
			if bytes.HasPrefix(message, []byte("######END_UPLOAD######;")) {
				if openFile != nil {
					openFile.Close()
				}
				var uploadErr error
				if preparedFileName != nil && preparedFileRequest != nil {
					uploadErr = services.Uploaded(*preparedFileName, *preparedFileRequest)
					if uploadErr != nil {
						self.logger.Error("Error uploading file", "error", uploadErr)
					}
				} else if preparedFileName == nil {
					uploadErr = fmt.Errorf("upload failed: could not open temporary file")
				}
				if preparedFileName != nil {
					os.Remove(*preparedFileName)
				}

				if preparedFileRequest != nil {
					ack := structs.CreateDatagramAck("ack:files/upload:end", preparedFileRequest.Id)
					if uploadErr != nil {
						ack.Err = uploadErr.Error()
					}
					go self.JobServerSendData(self.jobClients[0], ack)
				}

				preparedFileName = nil
				preparedFileRequest = nil
				continue
			}

			if preparedFileName != nil {
				_, err := openFile.Write(message)
				if err != nil {
					self.logger.Error("Error writing to file", "error", err)
				}
				continue
			}

			datagram, err := self.ParseDatagram(message)
			if err != nil {
				self.logger.Error("failed to parse datagram", "error", err)
				continue
			}

			datagram.DisplayReceiveSummary(self.logger)

			// TODO: refactor! @bene
			if datagram.Pattern == "files/upload" {
				preparedFileRequest = self.executeBinaryRequestUpload(datagram)

				var ack = structs.CreateDatagramAck("ack:files/upload:datagram", datagram.Id)
				go self.JobServerSendData(self.jobClients[0], ack)
				continue
			}

			// Blocks when all workers are busy (backpressures the read loop
			// instead of spawning unbounded goroutines).
			jobs <- datagram
		}
		self.logger.Debug("api messagehandler finished as the websocket client was terminated")
	}()
}

// dispatchJobClientMessages runs one worker for the default jobClient read
// loop. Looks up the handler for each incoming datagram and either executes
// it or replies with a no-handler error. Exits when jobs is closed.
func (self *socketApi) dispatchJobClientMessages(jobs <-chan structs.Datagram) {
	client := self.jobClients[0]
	for datagram := range jobs {
		if self.patternHandlerExists(datagram.Pattern) {
			self.handlePatternRequest(datagram, client)
		} else {
			self.sendNoHandlerError(client, datagram, false)
		}
	}
}

// clientName returns a human-readable label for the given client, used in logs.
func (self *socketApi) clientName(client websocket.WebsocketClient) string {
	for i, c := range self.jobClients {
		if c == client {
			return fmt.Sprintf("jobClient-%d", i)
		}
	}
	if client == self.eventsClient {
		return "eventsClient"
	}
	return "unknown"
}

func (self *socketApi) handlePatternRequest(datagram structs.Datagram, responseClient websocket.WebsocketClient) {
	self.logger.Info("handling pattern request", "pattern", datagram.Pattern, "wsClient", self.clientName(responseClient))

	if datagram.Zlib {
		decompressedData, err := utils.TryZlibDecompress(datagram.Payload)
		if err != nil {
			self.logger.Error("failed to decompress payload", "error", err)
			result := structs.Datagram{
				Id:      datagram.Id,
				Pattern: datagram.Pattern,
				Payload: struct {
					Status  string `json:"status"`
					Message string `json:"message"`
				}{
					Status:  "error",
					Message: fmt.Sprintf("failed to decompress payload: %s", err.Error()),
				},
				CreatedAt: datagram.CreatedAt,
				Zlib:      false,
			}
			self.JobServerSendData(responseClient, result)
			return
		}
		datagram.Payload = decompressedData
	}

	start := time.Now()
	responsePayload := self.ExecuteCommandRequest(datagram)
	executionTime := time.Since(start)
	self.logdatagram(executionTime, datagram)

	sendStart := time.Now()

	// Marshal the response payload once, here in the worker goroutine (not on
	// the single write thread). Large payloads are zlib-compressed; smaller
	// ones are handed on as raw JSON so the envelope marshal in
	// JobServerSendData reuses these bytes instead of walking the response
	// struct again via reflection.
	var size int64
	var payload any
	var shouldCompress bool

	payloadBytes, marshalErr := json.Marshal(responsePayload)
	if marshalErr != nil {
		// Fall back to letting the envelope marshal handle it.
		self.logger.Error("failed to marshal response payload", "pattern", datagram.Pattern, "error", marshalErr)
		payload = responsePayload
	} else {
		size = int64(len(payloadBytes))
		if len(payloadBytes) > compressionThreshold {
			if compressed, compressErr := utils.ZlibCompress(payloadBytes); compressErr == nil {
				payload = compressed
				shouldCompress = true
			} else {
				self.logger.Error("failed to compress response payload", "pattern", datagram.Pattern, "error", compressErr)
				payload = json.RawMessage(payloadBytes)
			}
		} else {
			payload = json.RawMessage(payloadBytes)
		}
	}

	result := structs.Datagram{
		Id:        datagram.Id,
		Pattern:   datagram.Pattern,
		Payload:   payload,
		CreatedAt: datagram.CreatedAt,
		Zlib:      shouldCompress,
	}

	self.JobServerSendData(responseClient, result)
	sendTime := time.Since(sendStart)
	self.logPattern(executionTime, sendTime, datagram, size)
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

	// Decode with the payload captured as raw JSON instead of a generic map.
	// LoadRequest then unmarshals it once, directly into the typed request
	// struct, removing a redundant decode-to-map followed by a re-marshal on
	// every inbound request. A json.RawMessage round-trips to its own bytes,
	// so the zlib-decompress and audit-log paths keep working unchanged.
	raw := struct {
		structs.Datagram
		Payload json.RawMessage `json:"payload,omitempty"`
	}{Datagram: datagram}

	if err := json.Unmarshal(data, &raw); err != nil {
		self.logger.Error("failed to unmarshal", "error", err)
		return datagram, err
	}

	datagram = raw.Datagram
	if len(raw.Payload) > 0 {
		datagram.Payload = raw.Payload
	}

	validationErr := utils.ValidateJSON(datagram)
	if validationErr != nil {
		self.logger.Error("validaten failed for datagram", "pattern", datagram.Pattern, "validationErr", validationErr)
		return datagram, fmt.Errorf("validaten failed for datagram: %s", strings.Join(validationErr.Errors, " | "))
	}

	return datagram, nil
}

func (self *socketApi) JobServerSendData(jobClient websocket.WebsocketClient, datagram structs.Datagram) {
	// Marshal here (in the caller's goroutine) and send the bytes via WriteRaw.
	// This keeps JSON encoding off the connection's single write thread, so
	// concurrent responses no longer serialize on the marshaling step.
	data, err := json.Marshal(datagram)
	if err != nil {
		self.logger.Error("failed to marshal datagram", "pattern", datagram.Pattern, "error", err)
		return
	}
	if err := jobClient.WriteRaw(data); err != nil {
		self.logger.Error("Error sending data to EventServer", "error", err)
	}
}

func (self *socketApi) ExecuteCommandRequest(datagram structs.Datagram) any {
	if patternHandler, ok := self.patternHandler[datagram.Pattern]; ok {
		start := time.Now()
		result := patternHandler.Callback(datagram)
		moMetrics.ObservePatternDuration(datagram.Pattern, time.Since(start).Seconds())
		return result
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

	if self.config.Get("MO_ENABLE_AUTO_UPGRADE") != "true" {
		msg := "Automatic Upgrades are disabled. If you are using GitOps please update the Operator in your GitOps Repository"
		job.Fail(msg)
		job.Finished = time.Now()
		structs.ReportJobStateToServer(self.eventsClient, job)
		return job, fmt.Errorf("%s", msg)
	}

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
		// "sh" is a shell-open request: RunExec auto-detects an available shell
		// (bash, sh, ash) instead of assuming sh, which fails on images that ship
		// only bash/ash or no plain sh
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
