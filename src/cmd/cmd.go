package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/controllers"
	"mogenius-k8s-manager/src/core"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/helm"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/kubernetes"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/secrets"
	"mogenius-k8s-manager/src/services"
	"mogenius-k8s-manager/src/servicesexternal"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeystore"
	"mogenius-k8s-manager/src/version"
	"mogenius-k8s-manager/src/websocket"
	"mogenius-k8s-manager/src/xterm"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"
	"k8s.io/klog/v2"
)

const defaultLogDir string = "logs"

var CLI struct {
	// Commands
	Clean    struct{}     `cmd:"" help:"remove the operator from the cluster"`
	Cluster  struct{}     `cmd:"" help:"start the operator"`
	Config   struct{}     `cmd:"" help:"print application config in ENV format"`
	Install  struct{}     `cmd:"" help:"install the operator into your cluster"`
	System   struct{}     `cmd:"" help:"check the system for all required components and offer healing"`
	Version  struct{}     `cmd:"" help:"print version information" default:"1"`
	Patterns patternsArgs `cmd:"" help:"print patterns to shell"`
	Exec     execArgs     `cmd:"" help:"open an interactive shell inside a container"`
	Logs     logArgs      `cmd:"" help:"retrieve streaming logs of a container"`
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
	// Since the ConfigModule is initialized AFTER the LoggingModule
	// this is an edge case. We have to directly access the MO_LOG_DIR
	// variable. For documentation purposes there is also a key in the
	// ConfigModule which loads the same ENV variable.
	var logDir *string
	if path := configModule.Get("MO_LOG_DIR"); path != "" {
		logDir = &path
	}
	var logFileOpts *logging.SlogManagerOptsLogFile = nil
	if logDir != nil {
		logFileOpts = &logging.SlogManagerOptsLogFile{
			LogDir:             logDir,
			EnableCombinedLog:  true,
			EnableComponentLog: true,
		}
	}
	logFilter := []string{}
	moLogFilter := strings.Split(configModule.Get("MO_LOG_FILTER"), ",")
	for _, f := range moLogFilter {
		if f != "" {
			logFilter = append(logFilter, f)
		}
	}
	logLevel, err := logging.ParseLogLevel(configModule.Get("MO_LOG_LEVEL"))
	assert.Assert(err == nil, "failed to parse log level", err)
	slogManager := logging.NewSlogManager(logging.SlogManagerOpts{
		LogLevel: logLevel,
		ConsoleOpts: &logging.SlogManagerOptsConsole{
			LogFilter: logFilter,
		},
		LogFileOpts: logFileOpts,
		MessageReplace: func(msg string) string {
			msg = secrets.EraseSecrets(msg)
			return msg
		},
	})
	cmdLogger := slogManager.CreateLogger("cmd")
	klogLogger := slogManager.CreateLogger("klog")
	klog.SetSlogLogger(klogLogger)

	//===============================================================
	//========================= Parse Args ==========================
	//===============================================================
	ctx := kong.Parse(
		&CLI,
		kong.Name("mogenius-k8s-manager"),
		kong.Description("kubernetes operator for https://mogenius.com"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: false,
			Summary: true,
			Tree:    true,
		}),
	)

	//===============================================================
	//======================= Execute Command =======================
	//===============================================================
	switch ctx.Command() {
	case "clean":
		err := RunClean(slogManager, configModule, cmdLogger)
		if err != nil {
			return err
		}
		return nil
	case "cluster":
		err := RunCluster(slogManager, configModule, cmdLogger)
		if err != nil {
			return err
		}
		return nil
	case "install":
		err := RunInstall(slogManager, configModule, cmdLogger)
		if err != nil {
			return err
		}
		return nil
	case "system":
		err := RunSystem(slogManager, configModule, cmdLogger)
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
		err := RunExec(&CLI.Exec, cmdLogger)
		if err != nil {
			return err
		}
		return nil
	case "logs":
		err := RunLogs(&CLI.Logs, cmdLogger)
		if err != nil {
			return err
		}
		return nil
	case "patterns":
		err := RunPatterns(&CLI.Patterns, slogManager, configModule, cmdLogger)
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
		DefaultValue: utils.Pointer("prod"),
		Description:  utils.Pointer("the stage automatically overrides API server configs"),
		Envs:         []string{"STAGE", "stage"},
		Validate: func(val string) error {
			allowedStages := []string{
				"prod",
				"pre-prod",
				"dev",
				"local",
				"", // empty to skip overrides
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
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
		Description:  utils.Pointer("the Namespace of mogenius platform"),
		Envs:         []string{"OWN_NAMESPACE"},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:         "MO_API_SERVER",
		Description: utils.Pointer("URL of API Server"),
		Validate: func(value string) error {
			_, err := url.Parse(value)
			if err != nil {
				return fmt.Errorf("'MO_API_SERVER' needs to be a URL: %s", err.Error())
			}
			return nil
		},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:         "MO_EVENT_SERVER",
		Description: utils.Pointer("URL of Event Server"),
		Validate: func(value string) error {
			_, err := url.Parse(value)
			if err != nil {
				return fmt.Errorf("'MO_EVENT_SERVER' needs to be a URL: %s", err.Error())
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
		Key:          "MO_GIT_VAULT_DATA_PATH",
		DefaultValue: utils.Pointer(filepath.Join(workDir, "git-vault-data")),
		Description:  utils.Pointer("path to the git vault data"),
		Envs:         []string{"git_vault_data_path"},
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
		Key:          "MO_LOCAL_CONTAINER_REGISTRY_HOST",
		DefaultValue: utils.Pointer("mocr.local.mogenius.io"),
		Description:  utils.Pointer("local container registry inside the cluster"),
		Envs:         []string{"local_registry_host"},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_DEFAULT_MOUNT_PATH",
		DefaultValue: utils.Pointer(filepath.Join(workDir, "mo-data")),
		Description:  utils.Pointer("all containers have access to this mount point"),
		Envs:         []string{"default_mount_path"},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_UPDATE_INTERVAL",
		DefaultValue: utils.Pointer("86400"),
		Description:  utils.Pointer("time interval between update checks"),
		Envs:         []string{"check_for_updates"},
		Validate: func(value string) error {
			_, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("'MO_UPDATE_INTERVAL' needs to be an integer: %s", err.Error())
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
		Key:          "MO_IGNORE_NAMESPACES",
		DefaultValue: utils.Pointer(`["kube-system"]`),
		Description:  utils.Pointer("list of all ignored namespaces"),
		Envs:         []string{"ignore_namespaces"},
		Validate: func(value string) error {
			var ignoreNamespaces []string
			err := json.Unmarshal([]byte(value), &ignoreNamespaces)
			if err != nil {
				return fmt.Errorf("'MO_IGNORE_NAMESPACES' needs to be a json `[]string`: %s", err.Error())
			}
			return nil
		},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_LOG_LEVEL",
		DefaultValue: utils.Pointer("info"),
		Description:  utils.Pointer(`a log level: "debug", "info", "warn" or "error"`),
		Validate: func(val string) error {
			allowedLogLevels := []string{"debug", "info", "warn", "error"}
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
		Key:          "MO_LOG_DIR",
		DefaultValue: utils.Pointer("logs"),
		Description:  utils.Pointer(`path in which logs are stored in the filesystem`),
		ReadOnly:     true,
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
	case "prod":
		configModule.Set("MO_API_SERVER", "wss://k8s-ws.mogenius.com/ws")
		configModule.Set("MO_EVENT_SERVER", "wss://k8s-dispatcher.mogenius.com/ws")
	case "pre-prod":
		configModule.Set("MO_API_SERVER", "wss://k8s-ws.pre-prod.mogenius.com/ws")
		configModule.Set("MO_EVENT_SERVER", "wss://k8s-dispatcher.pre-prod.mogenius.com/ws")
	case "dev":
		configModule.Set("MO_API_SERVER", "wss://k8s-ws.dev.mogenius.com/ws")
		configModule.Set("MO_EVENT_SERVER", "wss://k8s-dispatcher.dev.mogenius.com/ws")
	case "local":
		configModule.Set("MO_API_SERVER", "ws://127.0.0.1:7011/ws")
		configModule.Set("MO_EVENT_SERVER", "ws://127.0.0.1:7011/ws")
	case "":
		// does not override
	}
}

// Full initialization process for mogenius-k8s-manager clients services (and packages)
func InitializeSystems(
	logManagerModule logging.LogManagerModule,
	configModule *config.Config,
	cmdLogger *slog.Logger,
) systems {
	assert.Assert(logManagerModule != nil)
	assert.Assert(configModule != nil)
	assert.Assert(cmdLogger != nil)

	// initialize client modules
	valkeyModule := valkeystore.NewValkeyStore(logManagerModule.CreateLogger("valkey"), configModule)
	clientProvider := k8sclient.NewK8sClientProvider(logManagerModule.CreateLogger("client-provider"))
	versionModule := version.NewVersion()
	watcherModule := kubernetes.NewWatcher(logManagerModule.CreateLogger("watcher"), clientProvider)
	shutdown.Add(watcherModule.UnwatchAll)
	dbstatsModule := kubernetes.NewValkeyStatsModule(logManagerModule.CreateLogger("db-stats"), configModule)
	jobConnectionClient := websocket.NewWebsocketClient(logManagerModule.CreateLogger("websocket-job-client"))
	shutdown.Add(jobConnectionClient.Terminate)
	eventConnectionClient := websocket.NewWebsocketClient(logManagerModule.CreateLogger("websocket-events-client"))
	shutdown.Add(eventConnectionClient.Terminate)

	// golang package setups are deprecated and will be removed in the future by migrating all state to services
	helm.Setup(logManagerModule, configModule)
	err := mokubernetes.Setup(logManagerModule, configModule, watcherModule, clientProvider, valkeyModule)
	assert.Assert(err == nil, err)
	controllers.Setup(logManagerModule, configModule)
	dtos.Setup(logManagerModule)
	services.Setup(logManagerModule, configModule, clientProvider)
	servicesexternal.Setup(logManagerModule, configModule)
	store.Setup(logManagerModule, valkeyModule)
	structs.Setup(logManagerModule)
	xterm.Setup(logManagerModule, clientProvider, valkeyModule)
	utils.Setup(logManagerModule, configModule)

	// initialization step 1 for services
	workspaceManager := core.NewWorkspaceManager(configModule, clientProvider)
	apiModule := core.NewApi(logManagerModule.CreateLogger("api"))
	httpApi := core.NewHttpApi(logManagerModule, configModule, dbstatsModule, apiModule)
	socketApi := core.NewSocketApi(logManagerModule.CreateLogger("socketapi"), configModule, jobConnectionClient, eventConnectionClient, dbstatsModule)
	xtermService := core.NewXtermService(logManagerModule.CreateLogger("xterm-service"))

	// initialization step 2 for services
	socketApi.Link(httpApi, xtermService, apiModule)
	httpApi.Link(socketApi)
	apiModule.Link(workspaceManager)

	return systems{
		clientProvider,
		versionModule,
		watcherModule,
		dbstatsModule,
		jobConnectionClient,
		eventConnectionClient,
		workspaceManager,
		apiModule,
		socketApi,
		httpApi,
		xtermService,
		valkeyModule,
	}
}

type systems struct {
	clientProvider        k8sclient.K8sClientProvider
	versionModule         *version.Version
	watcherModule         *kubernetes.Watcher
	dbstatsModule         kubernetes.ValkeyStatsDb
	jobConnectionClient   websocket.WebsocketClient
	eventConnectionClient websocket.WebsocketClient
	workspaceManager      core.WorkspaceManager
	apiModule             core.Api
	socketApi             core.SocketApi
	httpApi               core.HttpService
	xtermService          core.XtermService
	valkeyModule          valkeystore.ValkeyStore
}
