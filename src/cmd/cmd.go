package cmd

import (
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/alecthomas/kong"
	"k8s.io/klog/v2"
)

const defaultLogDir string = "logs"

var CLI struct {
	// Commands
	Clean   struct{} `cmd:"" help:"remove the operator from the cluster"`
	Cluster struct{} `cmd:"" help:"start the operator"`
	Config  struct{} `cmd:"" help:"print application config in ENV format"`
	Install struct{} `cmd:"" help:"install the operator into your cluster"`
	System  struct{} `cmd:"" help:"check the system for all required components and offer healing"`
	Version struct{} `cmd:"" help:"print version information" default:"1"`
	Exec    execArgs `cmd:"" help:"open an interactive shell inside a container"`
	Logs    logArgs  `cmd:"" help:"retrieve streaming logs of a container"`
}

func Run() error {
	//===============================================================
	//====================== Initialize Logger ======================
	//===============================================================
	// Since the ConfigModule is initialized AFTER the LoggingModule
	// this is an edge case. We have to directly access the MO_LOG_DIR
	// variable. For documentation purposes there is also a key in the
	// ConfigModule which loads the same ENV variable.
	logDir := defaultLogDir
	if path := os.Getenv("MO_LOG_DIR"); path != "" {
		logDir = path
	}
	slogManager := logging.NewSlogManager(logDir)
	cmdLogger := slogManager.CreateLogger("cmd")
	klogLogger := slogManager.CreateLogger("klog")
	klog.SetSlogLogger(klogLogger)

	//===============================================================
	//====================== Initialize Config ======================
	//===============================================================
	configModule := config.NewConfig()
	configModule.OnChanged(nil, func(key string, value string, isSecret bool) {
		logging.UpdateConfigSecrets(configModule.GetAll())
	})
	LoadConfigDeclarations(configModule)
	configModule.LoadEnvs()
	ApplyStageOverrides(configModule)

	//===============================================================
	//==================== Update Logger Config =====================
	//===============================================================
	enabled, err := strconv.ParseBool(configModule.Get("MO_LOG_STDERR"))
	assert.Assert(err == nil, err)
	slogManager.SetStderr(enabled)

	logLevel := configModule.Get("MO_LOG_LEVEL")
	err = slogManager.SetLogLevel(logLevel)
	assert.Assert(err == nil, err)

	logFilter := configModule.Get("MO_LOG_FILTER")
	err = slogManager.SetLogFilter(logFilter)
	if err != nil {
		panic(fmt.Errorf("failed to configure logfilter: %s", err.Error()))
	}
	// The value of "MO_LOG_DIR" is explicitly requested once to silence
	// the warning of being unused. Due to initialization order the
	// logger directly requests os.Getenv("MO_LOG_DIR") for the value.
	_, err = configModule.TryGet("MO_LOG_DIR")
	assert.Assert(err == nil, "MO_LOG_DIR has to be declared before it is requested.")

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
		Description: utils.Pointer("the name of the kubernetes cluster"),
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
		Key:          "MO_BBOLT_DB_PATH",
		DefaultValue: utils.Pointer(filepath.Join(workDir, "mogenius.db")),
		Description:  utils.Pointer("path to the bbolt database"),
		Envs:         []string{"bbolt_db_path"},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_STATS_PATH",
		DefaultValue: utils.Pointer(filepath.Join(workDir, "mogenius-stats.db")),
		Description:  utils.Pointer("path to the bbolt database"),
		Envs:         []string{"bbolt_db_path"},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_STATS_MAX_DATA_POINTS",
		DefaultValue: utils.Pointer("6000"),
		Description:  utils.Pointer(`after n data points in bucket will be overwritten following the "Last In - First Out" principle`),
		Envs:         []string{"max_data_points"},
		Validate: func(value string) error {
			_, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("'MO_BBOLT_DB_STATS_MAX_DATA_POINTS' needs to be an integer: %s", err.Error())
			}
			return nil
		},
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
		Key:          "MO_BUILDER_BUILD_TIMEOUT",
		DefaultValue: utils.Pointer("3600"),
		Description:  utils.Pointer("seconds until the build will be canceled"),
		Envs:         []string{"max_build_time"},
		Validate: func(value string) error {
			_, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("'MO_BUILDER_BUILD_TIMEOUT' needs to be an integer: %s", err.Error())
			}
			return nil
		},
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_BUILDER_MAX_CONCURRENT_BUILDS",
		DefaultValue: utils.Pointer("1"),
		Description:  utils.Pointer("number of concurrent builds"),
		Envs:         []string{"max_concurrent_builds"},
		Validate: func(value string) error {
			_, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("'MO_BUILDER_MAX_CONCURRENT_BUILDS' needs to be an integer: %s", err.Error())
			}
			return nil
		},
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
				return fmt.Errorf("'MO_BUILDER_MAX_CONCURRENT_BUILDS' needs to be an integer: %s", err.Error())
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
		Key:          "MO_LOG_STDERR",
		DefaultValue: utils.Pointer("true"),
		Description:  utils.Pointer("enable logging to stderr"),
		Validate: func(value string) error {
			_, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("'MO_LOG_STDERR' needs to be a boolean: %s", err.Error())
			}
			return nil
		},
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
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_DEBUG",
		DefaultValue: utils.Pointer("false"),
		Description:  utils.Pointer("enable debug mode"),
		Validate: func(value string) error {
			_, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("'MO_DEBUG' needs to be a boolean: %s", err.Error())
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
