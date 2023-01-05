/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/socketServer"
	"mogenius-k8s-manager/utils"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

// testCmd represents the test command
var testServerCmd = &cobra.Command{
	Use:   "testserver",
	Short: "Print testServerCmd information and exit.",
	Long:  `Print testServerCmd information and exit.`,
	Run: func(cmd *cobra.Command, args []string) {
		gin.SetMode(gin.ReleaseMode)
		router := gin.Default()
		socketServer.Init(router)
		logger.Log.Noticef("Started WS server %s:%d 🚀", utils.CONFIG.ApiServer.WebsocketServer, utils.CONFIG.ApiServer.WebsocketPort)

		go socketServer.ReadInput()
		router.Run()
	},
}

func init() {
	rootCmd.AddCommand(testServerCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// testCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// testCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
