/*
Copyright © 2022 mogenius, Benedikt Iltisberger
*/
package cmd

import (
	"fmt"
	"mogenius-k8s-manager/builder"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	socketclient "mogenius-k8s-manager/socket-client"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"

	punq "github.com/mogenius/punq/structs"

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

		builder.Init()

		// INIT MOUNTS
		if utils.CONFIG.Misc.AutoMountNfs {
			volumesToMount, err := utils.GetVolumeMountsForK8sManager()
			if err != nil && utils.CONFIG.Misc.Stage != "local" {
				logger.Log.Errorf("GetVolumeMountsForK8sManager ERROR: %s", err.Error())
			}
			for _, vol := range volumesToMount {
				mokubernetes.Mount(vol.Namespace.Name, vol.VolumeName, nil)
			}
		}

		go structs.ConnectToEventQueue()
		go structs.ConnectToJobQueue()
		go mokubernetes.WatchEvents()

		punq.ExecuteBashCommandSilent("Git setup (1/4) ...", fmt.Sprintf(`git config --global user.email "%s"`, utils.CONFIG.Git.GitUserEmail))
		punq.ExecuteBashCommandSilent("Git setup (2/4) ...", fmt.Sprintf(`git config --global user.name "%s"`, utils.CONFIG.Git.GitUserName))
		punq.ExecuteBashCommandSilent("Git setup (3/4) ...", fmt.Sprintf(`git config --global init.defaultBranch %s`, utils.CONFIG.Git.GitDefaultBranch))
		punq.ExecuteBashCommandSilent("Git setup (4/4) ...", fmt.Sprintf(`git config --global advice.addIgnoredFile %s`, utils.CONFIG.Git.GitAddIgnoredFile))

		socketclient.StartK8sManager(true)
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
