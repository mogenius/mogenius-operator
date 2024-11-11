/*
Copyright © 2022 mogenius, Benedikt Iltisberger
*/
package cmd

import (
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/assert"
	"mogenius-k8s-manager/config"
	"mogenius-k8s-manager/interfaces"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logging"
	"mogenius-k8s-manager/shutdown"
	"mogenius-k8s-manager/utils"
	"net/url"
	"os"
	"slices"
	"strconv"

	punqDtos "github.com/mogenius/punq/dtos"
	punq "github.com/mogenius/punq/kubernetes"

	cc "github.com/ivanpirog/coloredcobra"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

type rootCmdConfig struct {
	resetConfig  bool
	stage        string
	debug        bool
	customConfig string
}

const defaultLogDir string = "logs"

var rootConfig rootCmdConfig

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
	if rootConfig.resetConfig {
		utils.DeleteCurrentConfig()
	}
	utils.InitConfigYaml(rootConfig.debug, rootConfig.customConfig, rootConfig.stage)
	punq.InitKubernetes(utils.CONFIG.Kubernetes.RunInCluster)

	if utils.CONFIG.Kubernetes.RunInCluster {
		utils.PrintVersionInfo()
	} else {
		cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())
	}

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
	cmdConfig.OnAfterChange(func(key string, value string, isSecret bool) {
		configValues := cmdConfig.GetAll()
		logging.UpdateConfigSecrets(configValues)
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

	rootCmd.PersistentFlags().BoolVarP(&rootConfig.resetConfig, "reset-config", "k", false, "Reset Config YAML File '~/.mogenius-k8s-manager/config.yaml'.")
	rootCmd.PersistentFlags().StringVarP(&rootConfig.customConfig, "config", "y", "", "Use config from custom location")
}

func initConfigDeclarations() {
	assert.Assert(cmdConfig != nil, "This has to be called **after** initializing `cmdConfig`")
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:         "MO_API_KEY",
		Description: utils.Pointer("Api Key to access the server"),
		IsSecret:    true,
		Envs:        []string{"api_key"},
		Cobra: &interfaces.ConfigCobraFlags{
			Name: "api-key",
		},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:         "MO_CLUSTER_NAME",
		Description: utils.Pointer("The Name of the Kubernetes Cluster"),
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
		Description:  utils.Pointer("mogenius k8s-manager stage"),
		Envs:         []string{"STAGE", "stage"},
		Cobra: &interfaces.ConfigCobraFlags{
			Name: "stage",
		},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
		Description:  utils.Pointer("The Namespace of mogenius platform"),
		Envs:         []string{"OWN_NAMESPACE"},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_API_SERVER",
		DefaultValue: utils.Pointer("wss://127.0.0.1:8080/ws"),
		Description:  utils.Pointer("URL of API Server"),
		Validate: func(value string) error {
			_, err := url.Parse(value)
			if err != nil {
				return fmt.Errorf("'MO_API_SERVER' needs to be a URL: %s", err.Error())
			}
			return nil
		},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_EVENT_SERVER",
		DefaultValue: utils.Pointer("wss://127.0.0.1:8080/ws-event"),
		Description:  utils.Pointer("URL of Event Server"),
		Validate: func(value string) error {
			_, err := url.Parse(value)
			if err != nil {
				return fmt.Errorf("'MO_EVENT_SERVER' needs to be a URL: %s", err.Error())
			}
			return nil
		},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_LOG_LEVEL",
		DefaultValue: utils.Pointer("info"),
		Description:  utils.Pointer(`A log level: "debug", "info", "warn" or "error"`),
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
		Description:  utils.Pointer("Optional comma separated list of components for which logs should be enabled. If none are defined all logs are collected."),
		Cobra: &interfaces.ConfigCobraFlags{
			Name: "log-filter",
		},
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_LOG_DIR",
		DefaultValue: utils.Pointer(defaultLogDir),
		Description:  utils.Pointer(`Path in which logs are stored in the filesystem`),
		ReadOnly:     true,
	})
	cmdConfig.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_DEBUG",
		DefaultValue: utils.Pointer("false"),
		Description:  utils.Pointer("Enable debug mode"),
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
