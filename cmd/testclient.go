/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"mogenius-k8s-manager/builder"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	socketclient "mogenius-k8s-manager/socket-client"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"

	"github.com/spf13/cobra"
)

var testClientCmd = &cobra.Command{
	Use:   "testclient",
	Short: "Print testServerCmd information and exit.",
	Long:  `Print testServerCmd information and exit.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.InitConfigYaml(true, "", "local")
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

		socketclient.StartK8sManager(false)
	},
}

func init() {
	rootCmd.AddCommand(testClientCmd)
}
