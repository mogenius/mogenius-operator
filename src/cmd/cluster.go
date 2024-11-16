/*
Copyright © 2022 mogenius, Benedikt Iltisberger
*/
package cmd

import (
	"fmt"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/controllers"
	"mogenius-k8s-manager/src/crds"
	"mogenius-k8s-manager/src/db"
	dbstats "mogenius-k8s-manager/src/db-stats"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/helm"
	"mogenius-k8s-manager/src/httpService"
	iacmanager "mogenius-k8s-manager/src/iac-manager"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/migrations"
	"mogenius-k8s-manager/src/services"
	servicesExternal "mogenius-k8s-manager/src/services-external"
	"mogenius-k8s-manager/src/shutdown"
	socketclient "mogenius-k8s-manager/src/socket-client"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"
	"mogenius-k8s-manager/src/watcher"
	"mogenius-k8s-manager/src/xterm"
	"strconv"

	"github.com/spf13/cobra"
)

// the IAC Manager is not readily implemented and development has been put on hold
const IAC_MANAGER_ENABLED bool = false

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

			utils.PrintLogo()
			versionModule := version.NewVersion(slogManager)
			helm.Setup(slogManager, cmdConfig)
			mokubernetes.Setup(slogManager, cmdConfig)
			controllers.Setup(slogManager)
			crds.Setup(slogManager)
			db.Setup(slogManager, cmdConfig)
			dbstats.Setup(slogManager, cmdConfig)
			dtos.Setup(slogManager)
			if IAC_MANAGER_ENABLED {
				iacmanager.Setup(slogManager, cmdConfig, watcher.NewWatcher())
			}
			migrations.Setup(slogManager)
			services.Setup(slogManager, cmdConfig)
			servicesExternal.Setup(slogManager, cmdConfig)
			socketclient.Setup(slogManager, cmdConfig)
			store.Setup(slogManager)
			structs.Setup(slogManager, cmdConfig)
			utils.Setup(slogManager, cmdConfig)
			xterm.Setup(slogManager, cmdConfig)
			httpApi := httpService.NewHttpApi(slogManager, cmdConfig)

			preRun()

			versionModule.PrintVersionInfo()
			cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

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

			db.Start()
			store.Start()
			defer store.Defer()
			dbstats.Start()
			if IAC_MANAGER_ENABLED {
				iacmanager.Start()
			}
			go httpApi.Run(":1337")

			migrations.ExecuteMigrations()

			// INIT MOUNTS
			autoMountNfs, err := strconv.ParseBool(cmdConfig.Get("MO_AUTO_MOUNT_NFS"))
			assert.Assert(err == nil)
			if autoMountNfs {
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

			go structs.ConnectToEventQueue()
			go structs.ConnectToJobQueue()

			mokubernetes.CreateMogeniusContainerRegistryIngress()

			// Init Helm Config
			go func() {
				if err := helm.InitHelmConfig(); err != nil {
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
