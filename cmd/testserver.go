/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"context"
	"fmt"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	socketserver "mogenius-k8s-manager/socket-server"
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
		showDebug, _ := cmd.Flags().GetBool("debug")
		customConfig, _ := cmd.Flags().GetString("config")

		clusterSecret, err := mokubernetes.CreateClusterSecretIfNotExist(false)
		if err != nil {
			logger.Log.Fatalf("Error retrieving cluster secret. Aborting: %s.", err.Error())
		}

		// nfsServiceIp, err := mokubernetes.CreateNfsServiceIfNotExist(false)
		// if err != nil {
		// 	logger.Log.Fatalf("Error retrieving nfs service IP. Aborting: %s.", err.Error())
		// }

		utils.InitConfigYaml(showDebug, &customConfig, clusterSecret, false)
		// mokubernetes.CheckIfDeploymentUpdateIsRequiredForNfs(nfsServiceIp, false)

		if !utils.CONFIG.Misc.Debug {
			gin.SetMode(gin.ReleaseMode)
		}
		router := gin.Default()
		router.POST("path/to/send/data", func(c *gin.Context) {
			// c.JSON(http.StatusOK, gin.H{
			// 	"message": "OK",
			// })

			ctx := context.Background()
			cancelCtx, _ := context.WithCancel(ctx)

			reader := bufio.NewScanner(c.Request.Body)
			for {
				select {
				case <-cancelCtx.Done():
					fmt.Println("done")
					return
				default:
					for reader.Scan() {
						lastBytes := reader.Bytes()
						fmt.Println(string(lastBytes))
					}
				}
			}
			// data, err := ioutil.ReadAll(c.Request.Body)
			// if err != nil {
			// 	fmt.Println(err.Error())
			// }
			// fmt.Println(c.Request.Header)
			// fmt.Println(string(data))

		})
		socketserver.Init(router)
		logger.Log.Noticef("Started WS server %s 🚀", utils.CONFIG.ApiServer.Ws_Server)

		go socketserver.ReadInput()
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

	testServerCmd.Flags().BoolP("debug", "d", false, "Be verbose and show debug infos.")
	testServerCmd.Flags().StringP("config", "c", "config.yaml", "Use custom config.")
}
