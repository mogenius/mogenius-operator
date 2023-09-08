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
		clusterSecret, err := mokubernetes.CreateClusterSecretIfNotExist(true)
		if err != nil {
			logger.Log.Fatalf("Error retrieving cluster secret. Aborting: %s.", err.Error())
		}

		utils.SetupClusterSecret(clusterSecret)

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

		socketclient.StartK8sManager()
	},
}

func init() {
	rootCmd.AddCommand(clusterCmd)
}
