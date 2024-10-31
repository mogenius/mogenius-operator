/*
Copyright © 2022 mogenius, Benedikt Iltisberger
*/
package cmd

import (
	"fmt"
	"mogenius-k8s-manager/api"
	"mogenius-k8s-manager/controllers"
	"mogenius-k8s-manager/crds"
	"mogenius-k8s-manager/db"
	dbstats "mogenius-k8s-manager/db-stats"
	"mogenius-k8s-manager/dtos"
	iacmanager "mogenius-k8s-manager/iac-manager"
	"mogenius-k8s-manager/kubernetes"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/migrations"
	"mogenius-k8s-manager/services"
	socketclient "mogenius-k8s-manager/socket-client"
	"mogenius-k8s-manager/store"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"mogenius-k8s-manager/xterm"

	"github.com/spf13/cobra"
)

// clusterCmd represents the cluster command
var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Run the application inside a container",
	Long: `
	This cmd starts the application permanently into your cluster. 
	Please run cleanup if you want to remove it again.`,
	Run: func(cmd *cobra.Command, args []string) {
		mokubernetes.Setup(&slogManager)
		controllers.Setup(&slogManager)
		crds.Setup(&slogManager)
		db.Setup(&slogManager)
		dbstats.Setup(&slogManager)
		dtos.Setup(&slogManager)
		api.Setup(&slogManager)
		iacmanager.Setup(&slogManager)
		migrations.Setup(&slogManager)
		services.Setup(&slogManager)
		socketclient.Setup(&slogManager)
		store.Setup(&slogManager)
		structs.Setup(&slogManager)
		utils.Setup(&slogManager)
		xterm.Setup(&slogManager)

		preRun()

		clusterSecret, err := mokubernetes.CreateOrUpdateClusterSecret(nil)
		if err != nil {
			cmdLogger.Error("Error retrieving cluster secret. Aborting.", "error", err)
			panic(1)
		}
		clusterConfigmap, err := mokubernetes.CreateOrUpdateClusterConfigmap(nil)
		if err != nil {
			cmdLogger.Error("Error retrieving cluster configmap. Aborting.", "error", err.Error())
			panic(1)
		}

		utils.SetupClusterSecret(clusterSecret)
		utils.SetupClusterConfigmap(clusterConfigmap)

		if utils.CONFIG.Misc.Debug {
			utils.PrintSettings()
		}

		cmdLogger.Info("Init DB ...")
		db.Start()
		store.Start()
		defer store.Defer()
		dbstats.Start()
		iacmanager.Start()

		migrations.ExecuteMigrations()

		// INIT MOUNTS
		if utils.CONFIG.Misc.AutoMountNfs {
			volumesToMount, err := mokubernetes.GetVolumeMountsForK8sManager()
			if err != nil && utils.CONFIG.Misc.Stage != utils.STAGE_LOCAL {
				cmdLogger.Error("GetVolumeMountsForK8sManager", "error", err)
			}
			for _, vol := range volumesToMount {
				mokubernetes.Mount(vol.Namespace, vol.VolumeName, nil)
			}
		}

		go func() {
			services.DISABLEQUEUE = true
			basicApps, userApps := services.InstallDefaultApplications()
			if basicApps != "" || userApps != "" {
				err := utils.ExecuteShellCommandSilent("Installing default applications ...", fmt.Sprintf("%s\n%s", basicApps, userApps))
				cmdLogger.Info("Seeding Commands ( 🪴 🪴 🪴 )", "userApps", userApps)
				if err != nil {
					cmdLogger.Error("Error installing default applications", "error", err)
					panic(1)
				}
			}
			services.DISABLEQUEUE = false
			services.ProcessQueue() // Process the queue maybe there are builds left to build
		}()

		mokubernetes.InitOrUpdateCrds()

		go api.InitApi()
		go structs.ConnectToEventQueue()
		go structs.ConnectToJobQueue()
		watcherManager := kubernetes.NewWatcher()
		go kubernetes.WatchAllResources(&watcherManager)

		mokubernetes.CreateMogeniusContainerRegistryIngress()

		// Init Helm Config
		if err := mokubernetes.InitHelmConfig(); err != nil {
			cmdLogger.Error("Error initializing Helm Config", "error", err)
		}

		// Init Network Policy Configmap
		if err := mokubernetes.InitNetworkPolicyConfigMap(); err != nil {
			cmdLogger.Error("Error initializing Network Policy Configmap", "error", err)
		}

		socketclient.StartK8sManager()
	},
}

func init() {
	rootCmd.AddCommand(clusterCmd)
}
