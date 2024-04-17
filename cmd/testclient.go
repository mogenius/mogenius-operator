/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"mogenius-k8s-manager/db"
	dbstats "mogenius-k8s-manager/db-stats"
	api "mogenius-k8s-manager/http"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/migrations"
	socketclient "mogenius-k8s-manager/socket-client"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var testClientCmd = &cobra.Command{
	Use:   "testclient",
	Short: "Print testServerCmd information and exit.",
	Long:  `Print testServerCmd information and exit.`,
	Run: func(cmd *cobra.Command, args []string) {
		db.Init()
		dbstats.Init()

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

		mokubernetes.InitOrUpdateCrds()

		go api.InitApi()
		go structs.ConnectToEventQueue()
		go structs.ConnectToJobQueue()

		go mokubernetes.EventWatcher()

		socketclient.StartK8sManager()
	},
}

func init() {
	rootCmd.AddCommand(testClientCmd)
}
