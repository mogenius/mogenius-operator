/*
Copyright © 2022 mogenius, Benedikt Iltisberger
*/
package cmd

import (
	"fmt"
	"mogenius-k8s-manager/builder"
	"mogenius-k8s-manager/db"
	dbstats "mogenius-k8s-manager/db-stats"
	api "mogenius-k8s-manager/http"
	"mogenius-k8s-manager/kubernetes"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/migrations"
	"mogenius-k8s-manager/services"
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
		clusterSecret, err := mokubernetes.CreateClusterSecretIfNotExist()
		if err != nil {
			logger.Log.Fatalf("Error retrieving cluster secret. Aborting: %s.", err.Error())
		}

		utils.SetupClusterSecret(clusterSecret)

		logger.Log.Noticef("Init DB ...")
		db.Init()
		dbstats.Init()

		migrations.ExecuteMigrations()

		// INIT MOUNTS
		if utils.CONFIG.Misc.AutoMountNfs {
			volumesToMount, err := mokubernetes.GetVolumeMountsForK8sManager()
			if err != nil && utils.CONFIG.Misc.Stage != utils.STAGE_LOCAL {
				logger.Log.Errorf("GetVolumeMountsForK8sManager ERROR: %s", err.Error())
			}
			for _, vol := range volumesToMount {
				mokubernetes.Mount(vol.Namespace, vol.VolumeName, nil)
			}
		}

		go func() {
			builder.DISABLEQUEUE = true
			basicApps, userApps := services.InstallDefaultApplications()
			if basicApps != "" || userApps != "" {
				err := utils.ExecuteShellCommandSilent("Installing default applications ...", fmt.Sprintf("%s\n%s", basicApps, userApps))
				fmt.Printf("Seeding Commands (🪴🪴🪴): \"%s\".\n", userApps)
				if err != nil {
					logger.Log.Fatalf("Error installing default applications: %s", err.Error())
				}
			}
			builder.DISABLEQUEUE = false
		}()

		go api.InitApi()
		go structs.ConnectToEventQueue()
		go structs.ConnectToJobQueue()
		go mokubernetes.NewEventWatcher()

		punq.ExecuteShellCommandSilent("Git setup (1/4)", fmt.Sprintf(`git config --global user.email "%s"`, utils.CONFIG.Git.GitUserEmail))
		punq.ExecuteShellCommandSilent("Git setup (2/4)", fmt.Sprintf(`git config --global user.name "%s"`, utils.CONFIG.Git.GitUserName))
		punq.ExecuteShellCommandSilent("Git setup (3/4)", fmt.Sprintf(`git config --global init.defaultBranch %s`, utils.CONFIG.Git.GitDefaultBranch))
		punq.ExecuteShellCommandSilent("Git setup (4/4)", fmt.Sprintf(`git config --global advice.addIgnoredFile %s`, utils.CONFIG.Git.GitAddIgnoredFile))

		kubernetes.CreateMogeniusContainerRegistryTlsSecret()
		kubernetes.CreateMogeniusContainerRegistryIngress()

		socketclient.StartK8sManager()
	},
}

func init() {
	rootCmd.AddCommand(clusterCmd)
}
