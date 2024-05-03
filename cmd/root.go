/*
Copyright © 2022 mogenius, Benedikt Iltisberger
*/
package cmd

import (
	"mogenius-k8s-manager/utils"
	"os"

	cc "github.com/ivanpirog/coloredcobra"
	punqDtos "github.com/mogenius/punq/dtos"
	punq "github.com/mogenius/punq/kubernetes"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var resetConfig bool
var stage string
var debug bool = false
var customConfig string

var rootCmd = &cobra.Command{
	Use:   "mogenius-k8s-manager",
	Short: "Control your kubernetes cluster the easy way",
	Long: `
Use mogenius-k8s-manager to control your kubernetes cluster. 🚀`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if resetConfig {
			utils.DeleteCurrentConfig()
		}
		utils.InitConfigYaml(debug, customConfig, stage)
		punq.InitKubernetes(utils.CONFIG.Kubernetes.RunInCluster)

		if utils.ClusterProviderCached == punqDtos.UNKNOWN {
			foundProvider, err := punq.GuessClusterProvider(nil)
			if err != nil {
				log.Errorf("GuessClusterProvider ERR: %s", err.Error())
			}
			utils.ClusterProviderCached = foundProvider
			log.Infof("🎲 🎲 🎲 ClusterProvider: %s", string(foundProvider))
		}
	},
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
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&stage, "stage", "s", "", "Use different stage environment")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug information")
	rootCmd.PersistentFlags().BoolVarP(&resetConfig, "reset-config", "k", false, "Reset Config YAML File '~/.mogenius-k8s-manager/config.yaml'.")
	rootCmd.PersistentFlags().StringVarP(&customConfig, "config", "y", "", "Use config from custom location")
}
