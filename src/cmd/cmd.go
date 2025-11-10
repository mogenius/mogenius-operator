package cmd

import (
	"fmt"
	"log/slog"
	"mogenius-operator/src/argocd"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/containerenumerator"
	"mogenius-operator/src/core"
	"mogenius-operator/src/cpumonitor"
	"mogenius-operator/src/helm"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/networkmonitor"
	"mogenius-operator/src/rammonitor"
	"mogenius-operator/src/secrets"
	"mogenius-operator/src/services"
	"mogenius-operator/src/shutdown"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"mogenius-operator/src/version"
	"mogenius-operator/src/watcher"
	"mogenius-operator/src/websocket"
	"mogenius-operator/src/xterm"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/mattn/go-isatty"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/klog/v2"
)

var CLI struct {
	// Commands
	Cluster     struct{}        `cmd:"" help:"start the operator"`
	Nodemetrics nodeMetricsArgs `cmd:"" help:"start the node metrics collector"`
	Config      struct{}        `cmd:"" help:"print application config in ENV format"`
	System      struct{}        `cmd:"" help:"check the system for all required components and offer healing"`
	Version     struct{}        `cmd:"" help:"print version information" default:"1"`
	Patterns    patternsArgs    `cmd:"" help:"print patterns to shell"`
	Exec        execArgs        `cmd:"" help:"open an interactive shell inside a container"`
	Logs        logArgs         `cmd:"" help:"retrieve streaming logs of a container"`
}

func Run() error {
	//===============================================================
	//====================== Initialize Config ======================
	//===============================================================
	configModule := config.NewConfig()
	configModule.OnChanged(nil, func(key string, value string, isSecret bool) {
		secrets.UpdateConfigSecrets(configModule.GetAll())
	})
	LoadConfigDeclarations(configModule)
	configModule.LoadEnvs()
	ApplyStageOverrides(configModule)

	//===============================================================
	//====================== Initialize Logger ======================
	//===============================================================
	logLevel, err := logging.ParseLogLevel(configModule.Get("MO_LOG_LEVEL"))
	if configModule.Get("MO_LOG_LEVEL") == "mo" {
		logLevel = slog.LevelInfo
		err = nil
	}
	assert.Assert(err == nil, "failed to parse log level", err)
	logFilter := []string{}
	moLogFilter := strings.SplitSeq(configModule.Get("MO_LOG_FILTER"), ",")
	for f := range moLogFilter {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		logFilter = append(logFilter, f)
	}
	prettyPrintHandler := logging.NewPrettyPrintHandler(
		os.Stderr,
		isatty.IsTerminal(os.Stderr.Fd()),
		logLevel,
		logFilter,
		secrets.EraseSecrets,
	)
	recordChannelLogLevel := slog.LevelInfo
	if logLevel == slog.LevelDebug {
		recordChannelLogLevel = slog.LevelDebug
	}
	channelHandler := logging.NewRecordChannelHandler(
		512,
		recordChannelLogLevel,
		secrets.EraseSecrets,
	)
	slogManager := logging.NewSlogManager(
		logLevel,
		[]slog.Handler{
			channelHandler,
			prettyPrintHandler,
		},
	)
	cmdLogger := slogManager.CreateLogger("cmd")
	klog.SetSlogLogger(slogManager.CreateLogger("klog"))

	//===============================================================
	//========================= Parse Args ==========================
	//===============================================================
	ctx := kong.Parse(
		&CLI,
		kong.Name("mogenius-operator"),
		kong.Description("kubernetes operator for https://mogenius.com"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: false,
			Summary: true,
			Tree:    true,
		}),
	)

	//===============================================================
	//=================== Setup ENVs for Helm SDK ===================
	//===============================================================
	helm.InitEnvs(configModule)

	//===============================================================
	//======================= Execute Command =======================
	//===============================================================
	switch ctx.Command() {
	case "cluster":
		RunCluster(slogManager, configModule, cmdLogger, channelHandler.GetRecordChannel())
		return nil
	case "nodemetrics":
		RunNodeMetrics(&CLI.Nodemetrics, slogManager, configModule, cmdLogger, channelHandler.GetRecordChannel())
		return nil
	case "system":
		err := RunSystem(slogManager, configModule, cmdLogger, channelHandler.GetRecordChannel())
		if err != nil {
			return err
		}
		return nil
	case "version":
		versionModule := version.NewVersion()
		versionModule.PrintVersionInfo()
		return nil
	case "config":
		fmt.Println(configModule.AsEnvs())
		return nil
	case "exec <command>":
		err := RunExec(&CLI.Exec, cmdLogger, configModule)
		if err != nil {
			return err
		}
		return nil
	case "logs":
		err := RunLogs(&CLI.Logs, cmdLogger, configModule)
		if err != nil {
			return err
		}
		return nil
	case "patterns":
		err := RunPatterns(&CLI.Patterns, slogManager, configModule, cmdLogger, channelHandler.GetRecordChannel())
		if err != nil {
			return err
		}
		return nil
	default:
		return ctx.PrintUsage(true)
	}
}

func LoadConfigDeclarations(configModule *config.Config) {
	assert.Assert(configModule != nil)

	workDir, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("failed to get current workdir: %s", err.Error()))
	}

	configModule.Declare(config.ConfigDeclaration{
		Key:         "MO_API_KEY",
		Description: utils.Pointer("API key to access the server"),
		IsSecret:    true,
		Envs:        []string{"api_key"},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:         "MO_CLUSTER_NAME",
		Description: utils.Pointer("Name of the kubernetes cluster"),
		Envs:        []string{"cluster_name"},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:         "MO_CLUSTER_MFA_ID",
		Description: utils.Pointer("NanoId of the Kubernetes Cluster for MFA purpose"),
		IsSecret:    true,
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_STAGE",
		DefaultValue: utils.Pointer(string(utils.STAGE_PROD)),
		Description:  utils.Pointer("the stage automatically overrides API server configs"),
		Envs:         []string{"STAGE", "stage"},
		Validate: func(val string) error {
			allowedStages := []string{
				utils.STAGE_PROD,
				utils.STAGE_DEV,
			}
			if !slices.Contains(allowedStages, val) {
				return fmt.Errorf("'MO_STAGE' needs to be one of '%v' but is '%s'", allowedStages, val)
			}
			return nil
		},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_HTTP_ADDR",
		DefaultValue: utils.Pointer(":1337"),
		Description:  utils.Pointer("address of the controllers http api server"),
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "CLUSTER_DOMAIN",
		DefaultValue: utils.Pointer("cluster.local"),
		Description:  utils.Pointer("the cluster domain of the kubernetes cluster"),
		Envs:         []string{"CLUSTER_DOMAIN"},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
		Description:  utils.Pointer("the Namespace of mogenius platform"),
		Envs:         []string{"OWN_NAMESPACE"},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "OWN_NODE_NAME",
		DefaultValue: utils.Pointer(os.Getenv("OWN_NODE_NAME")),
		Description:  utils.Pointer("the name of the node this application is running in"),
		Envs:         []string{"OWN_NODE_NAME"},
		Validate: func(val string) error {
			if val == "" {
				return fmt.Errorf("'OWN_NODE_NAME' has to be defined and may not be empty: %#v", val)
			}
			return nil
		},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "OWN_DEPLOYMENT_NAME",
		DefaultValue: utils.Pointer("mogenius-operator"),
		Description:  utils.Pointer("mogenius-operatoroyment this application is running in"),
		Envs:         []string{"OWN_DEPLOYMENT_NAME"},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_API_SERVER",
		Description:  utils.Pointer("URL of API Server"),
		DefaultValue: utils.Pointer(""),
		Validate: func(value string) error {
			_, err := url.Parse(value)
			if err != nil {
				return fmt.Errorf("'MO_API_SERVER' needs to be a URL: %s", err.Error())
			}
			return nil
		},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_EVENT_SERVER",
		Description:  utils.Pointer("URL of Event Server"),
		DefaultValue: utils.Pointer(""),
		Validate: func(value string) error {
			_, err := url.Parse(value)
			if err != nil {
				return fmt.Errorf("'MO_EVENT_SERVER' needs to be a URL: %s", err.Error())
			}
			return nil
		},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_SKIP_TLS_VERIFICATION",
		DefaultValue: utils.Pointer("false"),
		Description:  utils.Pointer("Skip TLS verification for API and Event Server"),
		Validate: func(value string) error {
			_, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("'MO_SKIP_TLS_VERIFICATION' needs to be a boolean: %s", err.Error())
			}
			return nil
		},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:         "MO_VALKEY_ADDR",
		Description: utils.Pointer("Address of operator valkey Server"),
		Validate: func(value string) error {
			_, _, err := net.SplitHostPort(value)
			if err != nil {
				return fmt.Errorf("'MO_VALKEY_ADDR' needs to be a host:port address: %s", err.Error())
			}
			return nil
		},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:         "MO_VALKEY_PASSWORD",
		Description: utils.Pointer("Password of operator valkey Server"),
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_HELM_DATA_PATH",
		DefaultValue: utils.Pointer(filepath.Join(workDir, "helm-data")),
		Description:  utils.Pointer("path to the helm data"),
		Envs:         []string{"helm_data_path"},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_GIT_USER_NAME",
		DefaultValue: utils.Pointer("mogenius git-user"),
		Description:  utils.Pointer("user name which is used when interacting with git"),
		Envs:         []string{"git_user_name"},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_GIT_USER_EMAIL",
		DefaultValue: utils.Pointer("git@mogenius.com"),
		Description:  utils.Pointer("email address which is used when interacting with git"),
		Envs:         []string{"git_user_email"},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_DEFAULT_MOUNT_PATH",
		DefaultValue: utils.Pointer(filepath.Join(workDir, "mo-data")),
		Description:  utils.Pointer("all containers have access to this mount point"),
		Envs:         []string{"default_mount_path"},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_AUDIT_LOG_LIMIT",
		DefaultValue: utils.Pointer("1000"),
		Description:  utils.Pointer("maximum number of audit log entries to persist"),
		Envs:         []string{"audit_log_limit"},
		Validate: func(value string) error {
			_, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("'MO_AUDIT_LOG_LIMIT' needs to be an integer: %s", err.Error())
			}
			return nil
		},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_AUTO_MOUNT_NFS",
		DefaultValue: utils.Pointer("true"),
		Description:  utils.Pointer("if set to true, nfs pvc will automatically be mounted"),
		Envs:         []string{"auto_mount_nfs"},
		Validate: func(value string) error {
			_, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("'MO_AUTO_MOUNT_NFS' needs to be a boolean: %s", err.Error())
			}
			return nil
		},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_ENABLE_POD_STATS_COLLECTOR",
		DefaultValue: utils.Pointer("true"),
		Description:  utils.Pointer("enable collection of pod stats"),
		Validate: func(value string) error {
			_, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("'MO_ENABLE_POD_STATS_COLLECTOR' needs to be a boolean: %s", err.Error())
			}
			return nil
		},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_ENABLE_TRAFFIC_COLLECTOR",
		DefaultValue: utils.Pointer("false"),
		Description:  utils.Pointer("enable collection of network stats"),
		Validate: func(value string) error {
			_, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("'MO_ENABLE_TRAFFIC_COLLECTOR' needs to be a boolean: %s", err.Error())
			}
			return nil
		},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_SNOOPY_IMPLEMENTATION",
		DefaultValue: utils.Pointer("auto"),
		Description:  utils.Pointer("set which implementation for tracking network traffic should be used"),
		Validate: func(value string) error {
			allowedValues := []string{
				"auto",    // choose the best option based on whats available on the machine
				"snoopy",  // use snoopy to collect traffic data through eBPF
				"procdev", // read traffic data from the linux proc device
			}
			if !slices.Contains(allowedValues, value) {
				return fmt.Errorf("'MO_SNOOPY_IMPLEMENTATION' needs to be one of: %#v", allowedValues)
			}
			return nil
		},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "KUBERNETES_DEBUG",
		DefaultValue: utils.Pointer("false"),
		Description:  utils.Pointer("enable kubernetes sdk debug output"),
		Validate: func(value string) error {
			_, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("'KUBERNETES_DEBUG' needs to be a boolean: %s", err.Error())
			}
			return nil
		},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_HOST_PROC_PATH",
		DefaultValue: utils.Pointer("/proc"),
		Description:  utils.Pointer("mountpath of /proc"),
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_LOG_LEVEL",
		DefaultValue: utils.Pointer("info"),
		Description:  utils.Pointer(`a log level: "mo","debug", "info", "warn" or "error"`),
		Validate: func(val string) error {
			allowedLogLevels := []string{"mo", "debug", "info", "warn", "error"}
			if !slices.Contains(allowedLogLevels, val) {
				return fmt.Errorf("'MO_LOG_LEVEL' needs to be one of '%v' but is '%s'", allowedLogLevels, val)
			}
			return nil
		},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_LOG_FILTER",
		DefaultValue: utils.Pointer(""),
		Description:  utils.Pointer("comma separated list of components for which logs should be enabled - if none are defined all logs are collected"),
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_ALLOW_COUNTRY_CHECK",
		DefaultValue: utils.Pointer("true"),
		Description:  utils.Pointer(`allow the operator to determine its location country base on the IP address`),
		Validate: func(value string) error {
			_, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("'MO_ALLOW_COUNTRY_CHECK' needs to be a boolean: %s", err.Error())
			}
			return nil
		},
	})
}

func ApplyStageOverrides(configModule *config.Config) {
	stage := configModule.Get("MO_STAGE")
	switch stage {
	case utils.STAGE_PROD:
		if configModule.Get("MO_API_SERVER") == "" {
			configModule.Set("MO_API_SERVER", "wss://k8s-ws.mogenius.com/ws")
		}
		if configModule.Get("MO_EVENT_SERVER") == "" {
			configModule.Set("MO_EVENT_SERVER", "wss://k8s-dispatcher.mogenius.com/ws")
		}
	case utils.STAGE_DEV:
		if configModule.Get("MO_API_SERVER") == "" {
			configModule.Set("MO_API_SERVER", "wss://k8s-ws.dev.mogenius.com/ws")
		}
		if configModule.Get("MO_EVENT_SERVER") == "" {
			configModule.Set("MO_EVENT_SERVER", "wss://k8s-dispatcher.dev.mogenius.com/ws")
		}
	}
}

// Full initialization process for mogenius-operator clients services (and packages)
func InitializeSystems(
	logManagerModule logging.SlogManager,
	configModule *config.Config,
	cmdLogger *slog.Logger,
	valkeyLogChannel chan logging.LogLine,
) systems {
	assert.Assert(logManagerModule != nil)
	assert.Assert(configModule != nil)
	assert.Assert(cmdLogger != nil)

	// initialize client modules
	valkeyClient := valkeyclient.NewValkeyClient(logManagerModule.CreateLogger("valkey"), configModule)
	clientProvider := k8sclient.NewK8sClientProvider(logManagerModule.CreateLogger("client-provider"), configModule)
	if !clientProvider.RunsInCluster() {
		impersonatedClientProvider, err := clientProvider.WithImpersonate(v1.Subject{
			Kind:      "ServiceAccount",
			Name:      "mogenius-operator-service-account-app",
			Namespace: configModule.Get("MO_OWN_NAMESPACE"),
		})
		assert.Assert(err == nil, err)
		clientProvider = impersonatedClientProvider
	}
	versionModule := version.NewVersion()
	watcherModule := watcher.NewWatcher(logManagerModule.CreateLogger("watcher"), clientProvider)
	shutdown.Add(watcherModule.UnwatchAll)
	jobConnectionClient := websocket.NewWebsocketClient(logManagerModule.CreateLogger("websocket-job-client"))
	shutdown.Add(jobConnectionClient.Terminate)
	eventConnectionClient := websocket.NewWebsocketClient(logManagerModule.CreateLogger("websocket-events-client"))
	shutdown.Add(eventConnectionClient.Terminate)
	containerEnumerator := containerenumerator.NewContainerEnumerator(logManagerModule.CreateLogger("container-enumerator"), configModule, clientProvider)
	cpuMonitor := cpumonitor.NewCpuMonitor(logManagerModule.CreateLogger("cpu-monitor"), configModule, clientProvider, containerEnumerator)
	ramMonitor := rammonitor.NewRamMonitor(logManagerModule.CreateLogger("ram-monitor"), configModule, clientProvider, containerEnumerator)
	networkMonitor := networkmonitor.NewNetworkMonitor(logManagerModule.CreateLogger("network-monitor"), configModule, containerEnumerator, configModule.Get("MO_HOST_PROC_PATH"))

	// golang package setups are deprecated and will be removed in the future by migrating all state to services
	helm.Setup(logManagerModule, configModule, valkeyClient)
	err := kubernetes.Setup(logManagerModule, configModule, clientProvider, valkeyClient)
	assert.Assert(err == nil, err)
	services.Setup(logManagerModule, configModule, clientProvider)
	structs.Setup(logManagerModule)
	xterm.Setup(logManagerModule, valkeyClient)
	utils.Setup(logManagerModule, configModule)
	err = store.Setup(logManagerModule, valkeyClient, configModule.Get("MO_AUDIT_LOG_LIMIT"))
	assert.Assert(err == nil, err)

	// initialization step 1 for services
	argocd := argocd.NewArgoCd(logManagerModule, configModule, clientProvider, valkeyClient)
	workspaceManager := core.NewWorkspaceManager(configModule, clientProvider)
	apiModule := core.NewApi(logManagerModule.CreateLogger("api"), valkeyClient, configModule)
	httpApi := core.NewHttpApi(logManagerModule, configModule)
	socketApi := core.NewSocketApi(logManagerModule.CreateLogger("socketapi"), configModule, jobConnectionClient, eventConnectionClient, valkeyClient, argocd)
	xtermService := core.NewXtermService(logManagerModule.CreateLogger("xterm-service"))
	valkeyLoggerService := core.NewValkeyLogger(valkeyClient, valkeyLogChannel)
	ownerCacheService := core.NewOwnerCacheService(logManagerModule.CreateLogger("owner-cache"), configModule)
	dbstatsService := core.NewValkeyStatsModule(logManagerModule.CreateLogger("db-stats"), configModule, valkeyClient, ownerCacheService)
	podStatsCollector := core.NewPodStatsCollector(logManagerModule.CreateLogger("pod-stats-collector"), configModule, clientProvider)
	nodeMetricsCollector := core.NewNodeMetricsCollector(
		logManagerModule.CreateLogger("traffic-collector"),
		configModule,
		clientProvider,
		cpuMonitor,
		ramMonitor,
		networkMonitor,
	)
	moKubernetes := core.NewMoKubernetes(logManagerModule.CreateLogger("mokubernetes"), configModule, clientProvider)
	mocore := core.NewCore(logManagerModule.CreateLogger("core"), configModule, clientProvider, valkeyClient, eventConnectionClient, jobConnectionClient)
	leaderElector := core.NewLeaderElector(logManagerModule.CreateLogger("leader-elector"), configModule, clientProvider)
	reconciler := core.NewReconciler(logManagerModule.CreateLogger("reconciler"), configModule, clientProvider)
	sealedSecret := core.NewSealedSecretManager(logManagerModule.CreateLogger("sealed-secret"), configModule, clientProvider)

	// initialization step 2 for services
	mocore.Link(moKubernetes)
	podStatsCollector.Link(dbstatsService)
	nodeMetricsCollector.Link(dbstatsService, leaderElector)
	socketApi.Link(httpApi, xtermService, dbstatsService, apiModule, moKubernetes, sealedSecret)
	moKubernetes.Link(dbstatsService)
	httpApi.Link(socketApi, dbstatsService, apiModule, reconciler)
	apiModule.Link(workspaceManager)
	reconciler.Link(leaderElector)

	return systems{
		clientProvider,
		versionModule,
		watcherModule,
		jobConnectionClient,
		eventConnectionClient,
		valkeyClient,
		networkMonitor,
		mocore,
		moKubernetes,
		workspaceManager,
		apiModule,
		socketApi,
		httpApi,
		xtermService,
		valkeyLoggerService,
		podStatsCollector,
		nodeMetricsCollector,
		dbstatsService,
		leaderElector,
		reconciler,
		sealedSecret,
		argocd,
	}
}

type systems struct {
	clientProvider        k8sclient.K8sClientProvider
	versionModule         *version.Version
	watcherModule         watcher.WatcherModule
	jobConnectionClient   websocket.WebsocketClient
	eventConnectionClient websocket.WebsocketClient
	valkeyClient          valkeyclient.ValkeyClient
	networkmonitor        networkmonitor.NetworkMonitor
	core                  core.Core
	moKubernetes          core.MoKubernetes
	workspaceManager      core.WorkspaceManager
	apiModule             core.Api
	socketApi             core.SocketApi
	httpApi               core.HttpService
	xtermService          core.XtermService
	valkeyLoggerService   core.ValkeyLogger
	podStatsCollector     core.PodStatsCollector
	nodeMetricsCollector  core.NodeMetricsCollector
	dbstatsService        core.ValkeyStatsDb
	leaderElector         core.LeaderElector
	reconciler            core.Reconciler
	sealedSecret          core.SealedSecretManager
	argocd                argocd.Argocd
}
