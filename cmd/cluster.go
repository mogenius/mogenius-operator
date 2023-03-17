/*
Copyright © 2022 mogenius, Benedikt Iltisberger
*/
package cmd

import (
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/socketServer"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"

	"github.com/spf13/cobra"
)

// clusterCmd represents the cluster command
var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Run the application inside a container.",
	Long: `
	This cmd starts the application permanently into you cluster. 
	Please run cleanup if you want to remove it again.`,
	Run: func(cmd *cobra.Command, args []string) {
		showDebug, _ := cmd.Flags().GetBool("debug")
		customConfig, _ := cmd.Flags().GetString("config")

		clusterSecret, err := mokubernetes.CreateClusterSecretIfNotExist(true)
		if err != nil {
			logger.Log.Fatalf("Error retrieving cluster secret. Aborting: %s.", err.Error())
		}

		utils.InitConfigYaml(showDebug, &customConfig, clusterSecret, true)

		go structs.ConnectToEventQueue()
		go mokubernetes.WatchEvents()

		structs.ExecuteBashCommandSilent("Git setup ...", `git config --global user.email "git@mogenius.com"; git config --global user.name "mogenius git-user"; git config --global init.defaultBranch main; git config --global advice.addIgnoredFile false;`)

		socketServer.StartK8sManager(true)
	},
}

func init() {
	rootCmd.AddCommand(clusterCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// clusterCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// clusterCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	clusterCmd.Flags().BoolP("debug", "d", false, "Be verbose and show debug infos.")
	clusterCmd.Flags().StringP("config", "c", "config.yaml", "Use custom config.")
}
