/*
Copyright © 2022 mogenius, Benedikt Iltisberger
*/
package cmd

import (
	"fmt"
	"mogenius-k8s-manager/src/api"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/controllers"
	"mogenius-k8s-manager/src/crds"
	"mogenius-k8s-manager/src/db"
	dbstats "mogenius-k8s-manager/src/db-stats"
	"mogenius-k8s-manager/src/dtos"
	iacmanager "mogenius-k8s-manager/src/iac-manager"
	"mogenius-k8s-manager/src/kubernetes"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/migrations"
	"mogenius-k8s-manager/src/services"
	servicesExternal "mogenius-k8s-manager/src/services-external"
	"mogenius-k8s-manager/src/shutdown"
	socketclient "mogenius-k8s-manager/src/socket-client"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/xterm"
	"strconv"

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
		go func() {
			cmdConfig.Validate()

			logLevel := cmdConfig.Get("MO_LOG_LEVEL")
			err := slogManager.SetLogLevel(logLevel)
			if err != nil {
				panic(err)
			}
			logFilter := cmdConfig.Get("MO_LOG_FILTER")
			err = slogManager.SetLogFilter(logFilter)
			if err != nil {
				panic(err)
			}

			utils.PrintLogo()

			mokubernetes.Setup(slogManager, cmdConfig)
			controllers.Setup(slogManager)
			crds.Setup(slogManager)
			db.Setup(slogManager, cmdConfig)
			dbstats.Setup(slogManager)
			dtos.Setup(slogManager)
			api.Setup(slogManager, cmdConfig)
			iacmanager.Setup(slogManager, cmdConfig)
			migrations.Setup(slogManager)
			services.Setup(slogManager, cmdConfig)
			servicesExternal.Setup(cmdConfig)
			socketclient.Setup(slogManager, cmdConfig)
			store.Setup(slogManager)
			structs.Setup(slogManager, cmdConfig)
			utils.Setup(slogManager, cmdConfig)
			xterm.Setup(slogManager)

			preRun()

			clusterSecret, err := mokubernetes.CreateOrUpdateClusterSecret(nil)
			if err != nil {
				cmdLogger.Error("Error retrieving cluster secret. Aborting.", "error", err)
				shutdown.SendShutdownSignal(true)
				select {}
			}
			clusterConfigmap, err := mokubernetes.CreateAndUpdateClusterConfigmap()
			if err != nil {
				cmdLogger.Error("Error retrieving cluster configmap. Aborting.", "error", err.Error())
				shutdown.SendShutdownSignal(true)
				select {}
			}
			err = mokubernetes.CreateOrUpdateResourceTemplateConfigmap()
			if err != nil {
				cmdLogger.Error("Error creating resource template configmap", "error", err)
				shutdown.SendShutdownSignal(true)
				select {}
			}

			utils.SetupClusterSecret(clusterSecret)
			utils.SetupClusterConfigmap(clusterConfigmap)

			moDebug, err := strconv.ParseBool(cmdConfig.Get("MO_DEBUG"))
			assert.Assert(err == nil)
			if moDebug {
				utils.PrintSettings()
			}

			db.Start()
			store.Start()
			defer store.Defer()
			dbstats.Start()
			iacmanager.Start()

			migrations.ExecuteMigrations()

			// INIT MOUNTS
			if utils.CONFIG.Misc.AutoMountNfs {
				volumesToMount, err := mokubernetes.GetVolumeMountsForK8sManager()
				if err != nil && cmdConfig.Get("MO_STAGE") != utils.STAGE_LOCAL {
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
						shutdown.SendShutdownSignal(true)
						select {}
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
			go func() {
				if err := mokubernetes.InitHelmConfig(); err != nil {
					cmdLogger.Error("Error initializing Helm Config", "error", err)
				} else {
					cmdLogger.Info("Helm Config initialized")
				}
			}()

			// Init Network Policy Configmap
			go func() {
				if err := mokubernetes.InitNetworkPolicyConfigMap(); err != nil {
					cmdLogger.Error("Error initializing Network Policy Configmap", "error", err)
				} else {
					cmdLogger.Info("Network Policy Configmap initialized")
				}
			}()

			socketclient.StartK8sManager()
		}()

		shutdown.Listen()
	},
}

func init() {
	rootCmd.AddCommand(clusterCmd)
}
