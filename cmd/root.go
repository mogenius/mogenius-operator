/*
Copyright © 2022 mogenius, Benedikt Iltisberger
*/
package cmd

import (
	"log/slog"
	"mogenius-k8s-manager/logging"
	"mogenius-k8s-manager/utils"

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

var rootConfig rootCmdConfig

var slogManager logging.SlogManager = logging.NewSlogManager()
var cmdLogger *slog.Logger
var klogLogger *slog.Logger

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
		panic(1)
	}
}

func init() {
	slogManager = logging.NewSlogManager()
	cmdLogger = slogManager.CreateLogger("cmd")
	klogLogger = slogManager.CreateLogger("klog")
	klog.SetSlogLogger(klogLogger)
	rootCmd.PersistentFlags().StringVarP(&rootConfig.stage, "stage", "s", "", "Use different stage environment")
	rootCmd.PersistentFlags().BoolVarP(&rootConfig.debug, "debug", "d", false, "Enable debug information")
	rootCmd.PersistentFlags().BoolVarP(&rootConfig.resetConfig, "reset-config", "k", false, "Reset Config YAML File '~/.mogenius-k8s-manager/config.yaml'.")
	rootCmd.PersistentFlags().StringVarP(&rootConfig.customConfig, "config", "y", "", "Use config from custom location")
}
