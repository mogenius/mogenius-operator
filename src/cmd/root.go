/*
Copyright © 2022 mogenius, Benedikt Iltisberger
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/utils"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	punqDtos "github.com/mogenius/punq/dtos"
	punq "github.com/mogenius/punq/kubernetes"

	cc "github.com/ivanpirog/coloredcobra"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

const defaultLogDir string = "logs"

var slogManager *logging.SlogManager
var cmdLogger *slog.Logger
var klogLogger *slog.Logger
var cmdConfig *config.Config

var rootCmd = &cobra.Command{
	Use:   "mogenius-k8s-manager",
	Short: "Control your kubernetes cluster the easy way",
	Long: `
Use mogenius-k8s-manager to control your kubernetes cluster. 🚀`,
}

// TODO: this needs to be integrated in some smarter way
func preRun() {
	utils.PrintVersionInfo()
	cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

	if utils.ClusterProviderCached == punqDtos.UNKNOWN {
		foundProvider, err := punq.GuessClusterProvider(nil)
		if err != nil {
			cmdLogger.Error("GuessClusterProvider", "error", err)
		}
		utils.ClusterProviderCached = foundProvider
		cmdLogger.Info("🎲 🎲 🎲 ClusterProvider", "foundProvider", string(foundProvider))
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cc.Init(&cc.Config{
		RootCmd:  rootCmd,
		Headings: cc.HiCyan + cc.Bold + cc.Underline,
		Commands: cc.HiYellow + cc.Bold,
		Example:  cc.Italic,
		ExecName: cc.Bold,
		Flags:    cc.Bold,
	})

	err := rootCmd.Execute()
	if err != nil {
		cmdLogger.Error("rootCmd failed", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	}
}

func init() {
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
	slogManager = logging.NewSlogManager(logDir)
	cmdLogger = slogManager.CreateLogger("cmd")
	klogLogger = slogManager.CreateLogger("klog")
	klog.SetSlogLogger(klogLogger)

	//===============================================================
	//====================== Initialize Config ======================
	//===============================================================
	assert.Assert(slogManager != nil, "slogManager has to be initialized before cmdConfig")
	cmdConfig = config.NewConfig()
	cmdConfig.WithCobraCmd(rootCmd)
	cmdConfig.OnChanged(nil, func(key string, value string, isSecret bool) {
		configValues := cmdConfig.GetAll()
		logging.UpdateConfigSecrets(configValues)
	})
	cmdConfig.OnFinalized(applyStageOverrides)
	cmdConfig.OnFinalized(func() {
		enabled, err := strconv.ParseBool(cmdConfig.Get("MO_LOG_STDERR"))
		assert.Assert(err == nil)
		slogManager.SetStderr(enabled)

		logLevel := cmdConfig.Get("MO_LOG_LEVEL")
		err = slogManager.SetLogLevel(logLevel)
		assert.Assert(err == nil)

		logFilter := cmdConfig.Get("MO_LOG_FILTER")
		err = slogManager.SetLogFilter(logFilter)
		if err != nil {
			panic(fmt.Errorf("failed to configure logfilter: %s", err.Error()))
		}
	})
	defer cmdConfig.Init()
	// shutdown hook to detect unused keys
	shutdown.Add(func() {
		usages := cmdConfig.GetUsage()
		for _, usage := range usages {
			if usage.GetCalls == 0 {
				cmdLogger.Warn("config key might be unused", "usage", usage)
			} else {
				cmdLogger.Debug("config usage", "usage", usage)
			}
		}
	})
	initConfigDeclarations()

	//===============================================================
	//========================== Post Init ==========================
	//===============================================================
	// The value of "MO_LOG_DIR" is explicitly requested once to silence
	// the warning of being unused. Due to initialization order the
	// logger directly requests os.Getenv("MO_LOG_DIR") for the value.
	_, err := cmdConfig.TryGet("MO_LOG_DIR")
	assert.Assert(err == nil, "MO_LOG_DIR has to be declared before it is requested.")
}

func initConfigDeclarations() {
	workDir, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("failed to get current workdir: %s", err.Error()))
	}
	assert.Assert(cmdConfig != nil, "This has to be called **after** initializing `cmdConfig`")
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:         "MO_API_KEY",
		Description: utils.Pointer("API key to access the server"),
		IsSecret:    true,
		Envs:        []string{"api_key"},
		Cobra: &interfaces.ConfigCobraFlags{
			Name: "api-key",
		},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:         "MO_CLUSTER_NAME",
		Description: utils.Pointer("the name of the kubernetes cluster"),
		Envs:        []string{"cluster_name"},
		Cobra: &interfaces.ConfigCobraFlags{
			Name: "cluster-name",
		},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:         "MO_CLUSTER_MFA_ID",
		Description: utils.Pointer("NanoId of the Kubernetes Cluster for MFA purpose"),
		IsSecret:    true,
		Cobra: &interfaces.ConfigCobraFlags{
			Name: "mfa-id",
		},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_STAGE",
		DefaultValue: utils.Pointer("prod"),
		Description:  utils.Pointer("the stage automatically overrides API server configs"),
		Envs:         []string{"STAGE", "stage"},
		Cobra: &interfaces.ConfigCobraFlags{
			Name: "stage",
		},
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
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
		Description:  utils.Pointer("the Namespace of mogenius platform"),
		Envs:         []string{"OWN_NAMESPACE"},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
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
	cmdConfig.Declare(interfaces.ConfigDeclaration{
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
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_HELM_DATA_PATH",
		DefaultValue: utils.Pointer(filepath.Join(workDir, "helm-data")),
		Description:  utils.Pointer("path to the helm data"),
		Envs:         []string{"helm_data_path"},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_GIT_VAULT_DATA_PATH",
		DefaultValue: utils.Pointer(filepath.Join(workDir, "git-vault-data")),
		Description:  utils.Pointer("path to the git vault data"),
		Envs:         []string{"git_vault_data_path"},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_PATH",
		DefaultValue: utils.Pointer(filepath.Join(workDir, "mogenius.db")),
		Description:  utils.Pointer("path to the bbolt database"),
		Envs:         []string{"bbolt_db_path"},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_STATS_PATH",
		DefaultValue: utils.Pointer(filepath.Join(workDir, "mogenius-stats.db")),
		Description:  utils.Pointer("path to the bbolt database"),
		Envs:         []string{"bbolt_db_path"},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
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
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_GIT_USER_NAME",
		DefaultValue: utils.Pointer("mogenius git-user"),
		Description:  utils.Pointer("user name which is used when interacting with git"),
		Envs:         []string{"git_user_name"},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_GIT_USER_EMAIL",
		DefaultValue: utils.Pointer("git@mogenius.com"),
		Description:  utils.Pointer("email address which is used when interacting with git"),
		Envs:         []string{"git_user_email"},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_LOCAL_CONTAINER_REGISTRY_HOST",
		DefaultValue: utils.Pointer("mocr.local.mogenius.io"),
		Description:  utils.Pointer("local container registry inside the cluster"),
		Envs:         []string{"local_registry_host"},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
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
	cmdConfig.Declare(interfaces.ConfigDeclaration{
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
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_DEFAULT_MOUNT_PATH",
		DefaultValue: utils.Pointer(filepath.Join(workDir, "mo-data")),
		Description:  utils.Pointer("all containers have access to this mount point"),
		Envs:         []string{"default_mount_path"},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
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
	cmdConfig.Declare(interfaces.ConfigDeclaration{
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
	cmdConfig.Declare(interfaces.ConfigDeclaration{
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
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_LOG_LEVEL",
		DefaultValue: utils.Pointer("info"),
		Description:  utils.Pointer(`a log level: "debug", "info", "warn" or "error"`),
		Cobra: &interfaces.ConfigCobraFlags{
			Name: "log-level",
		},
		Validate: func(val string) error {
			allowedLogLevels := []string{"debug", "info", "warn", "error"}
			if !slices.Contains(allowedLogLevels, val) {
				return fmt.Errorf("'MO_LOG_LEVEL' needs to be one of '%v' but is '%s'", allowedLogLevels, val)
			}
			return nil
		},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_LOG_FILTER",
		DefaultValue: utils.Pointer(""),
		Description:  utils.Pointer("Comma separated list of components for which logs should be enabled. If none are defined all logs are collected."),
		Cobra: &interfaces.ConfigCobraFlags{
			Name: "log-filter",
		},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_LOG_STDERR",
		DefaultValue: utils.Pointer("true"),
		Description:  utils.Pointer("enable logging to stderr"),
		Cobra: &interfaces.ConfigCobraFlags{
			Name: "log-stderr",
		},
		Validate: func(value string) error {
			_, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("'MO_LOG_STDERR' needs to be a boolean: %s", err.Error())
			}
			return nil
		},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_LOG_DIR",
		DefaultValue: utils.Pointer(defaultLogDir),
		Description:  utils.Pointer(`path in which logs are stored in the filesystem`),
		ReadOnly:     true,
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_DEBUG",
		DefaultValue: utils.Pointer("false"),
		Description:  utils.Pointer("enable debug mode"),
		Cobra: &interfaces.ConfigCobraFlags{
			Name:  "debug",
			Short: utils.Pointer("d"),
		},
		Validate: func(value string) error {
			_, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("'MO_DEBUG' needs to be a boolean: %s", err.Error())
			}
			return nil
		},
	})
}

func applyStageOverrides() {
	stage := cmdConfig.Get("MO_STAGE")
	switch stage {
	case "prod":
		cmdConfig.Set("MO_API_SERVER", "wss://k8s-ws.mogenius.com/ws")
		cmdConfig.Set("MO_EVENT_SERVER", "wss://k8s-dispatcher.mogenius.com/ws")
	case "pre-prod":
		cmdConfig.Set("MO_API_SERVER", "wss://k8s-ws.pre-prod.mogenius.com/ws")
		cmdConfig.Set("MO_EVENT_SERVER", "wss://k8s-dispatcher.pre-prod.mogenius.com/ws")
	case "dev":
		cmdConfig.Set("MO_API_SERVER", "wss://k8s-ws.dev.mogenius.com/ws")
		cmdConfig.Set("MO_EVENT_SERVER", "wss://k8s-dispatcher.dev.mogenius.com/ws")
	case "local":
		cmdConfig.Set("MO_API_SERVER", "ws://127.0.0.1:8080/ws")
		cmdConfig.Set("MO_EVENT_SERVER", "ws://127.0.0.1:8080/ws")
	case "":
		// does not override
	}
}
