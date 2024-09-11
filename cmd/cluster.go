/*
Copyright © 2022 mogenius, Benedikt Iltisberger
*/
package cmd

import (
	"fmt"
	"mogenius-k8s-manager/db"
	dbstats "mogenius-k8s-manager/db-stats"
	api "mogenius-k8s-manager/http"
	iacmanager "mogenius-k8s-manager/iac-manager"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/migrations"
	"mogenius-k8s-manager/services"
	socketclient "mogenius-k8s-manager/socket-client"
	"mogenius-k8s-manager/store"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

// clusterCmd represents the cluster command
var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Run the application inside a container",
	Long: `
	This cmd starts the application permanently into you cluster. 
	Please run cleanup if you want to remove it again.`,
	Run: func(cmd *cobra.Command, args []string) {
		clusterSecret, err := mokubernetes.CreateOrUpdateClusterSecret(nil)
		if err != nil {
			log.Fatalf("Error retrieving cluster secret. Aborting: %s", err.Error())
		}

		utils.SetupClusterSecret(clusterSecret)

		log.Infof("Init DB ...")
		db.Init()
		store.Init()
		defer store.Defer()
		dbstats.Init()
		iacmanager.Init()

		migrations.ExecuteMigrations()

		// INIT MOUNTS
		if utils.CONFIG.Misc.AutoMountNfs {
			volumesToMount, err := mokubernetes.GetVolumeMountsForK8sManager()
			if err != nil && utils.CONFIG.Misc.Stage != utils.STAGE_LOCAL {
				log.Errorf("GetVolumeMountsForK8sManager ERROR: %s", err.Error())
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
				log.Infof("Seeding Commands ( 🪴 🪴 🪴 ): \"%s\".\n", userApps)
				if err != nil {
					log.Fatalf("Error installing default applications: %s", err.Error())
				}
			}
			services.DISABLEQUEUE = false
			services.ProcessQueue() // Process the queue maybe there are builds left to build
		}()

		mokubernetes.InitOrUpdateCrds()

		go api.InitApi()
		go structs.ConnectToEventQueue()
		go structs.ConnectToJobQueue()
		go mokubernetes.EventWatcher()
		go mokubernetes.ResourceWatcher()
		go mokubernetes.WatchPersistentVolumes()

		mokubernetes.CreateMogeniusContainerRegistryIngress()

		// Init Helm Config
		if err := mokubernetes.InitHelmConfig(); err != nil {
			log.Errorf("Error initializing Helm Config: %s", err.Error())
		}

		socketclient.StartK8sManager()
	},
}

func init() {
	rootCmd.AddCommand(clusterCmd)
}
