/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	socketserver "mogenius-k8s-manager/socket-server"
	"mogenius-k8s-manager/utils"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	punqStructs "github.com/mogenius/punq/structs"
)

var testServerCmd = &cobra.Command{
	Use:   "testserver",
	Short: "Print testServerCmd information and exit.",
	Long:  `Print testServerCmd information and exit.`,
	Run: func(cmd *cobra.Command, args []string) {
		// SETUP SECRET
		clusterSecret, err := mokubernetes.CreateClusterSecretIfNotExist()
		if err != nil {
			logger.Log.Fatalf("Error retrieving cluster secret. Aborting: %s.", err.Error())
		}

		utils.SetupClusterSecret(clusterSecret)
		utils.CONFIG.Misc.Stage = utils.STAGE_LOCAL

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

		if !utils.CONFIG.Misc.Debug {
			gin.SetMode(gin.ReleaseMode)
		}
		router := gin.Default()
		router.POST("path/to/send/data", func(c *gin.Context) {
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
		})
		router.POST("build-notification-test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "OK",
			})

			data, err := ioutil.ReadAll(c.Request.Body)
			if err != nil {
				fmt.Println(err.Error())
			}
			punqStructs.PrettyPrintJSON(data)

		})
		socketserver.Init(router)
		logger.Log.Noticef("Started WS server %s 🚀", utils.CONFIG.ApiServer.Ws_Server)

		go socketserver.ReadInput()
		router.Run()
	},
}

func init() {
	rootCmd.AddCommand(testServerCmd)
}
