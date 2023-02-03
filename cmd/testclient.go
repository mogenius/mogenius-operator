/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"log"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/socketServer"
	"mogenius-k8s-manager/utils"
	"os"
	"os/signal"
	"time"

	mokubernetes "mogenius-k8s-manager/kubernetes"

	"github.com/spf13/cobra"
)

// testCmd represents the test command
var testClientCmd = &cobra.Command{
	Use:   "testclient",
	Short: "Print testServerCmd information and exit.",
	Long:  `Print testServerCmd information and exit.`,
	Run: func(cmd *cobra.Command, args []string) {
		showDebug, _ := cmd.Flags().GetBool("debug")
		customConfig, _ := cmd.Flags().GetString("config")
		clusterName, _ := cmd.Flags().GetString("clustername")
		utils.InitConfigYaml(showDebug, &customConfig, &clusterName)

		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)

		maxGoroutines := utils.CONFIG.Misc.ConcurrentConnections
		connectionCounter := 0
		guard := make(chan struct{}, maxGoroutines)

		fmt.Println(utils.FillWith("", 60, "#"))
		fmt.Printf("###   CURRENT CONTEXT: %s   ###\n", utils.FillWith(mokubernetes.CurrentContextName(), 31, " "))
		fmt.Println(utils.FillWith("", 60, "#"))

		for {
			select {
			case <-interrupt:
				log.Fatal("CTRL + C pressed. Terminating.")
			case <-time.After(time.Second):
			}

			guard <- struct{}{} // would block if guard channel is already filled
			go func() {
				connectionCounter++
				socketServer.StartClient(connectionCounter)
				logger.Log.Error("Connection closed. Reconnecting after waiting 1 second ...")
				time.Sleep(1 * time.Second)
				<-guard
			}()

		}
	},
}

func init() {
	rootCmd.AddCommand(testClientCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// testCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// testCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	testClientCmd.Flags().BoolP("debug", "d", false, "Be verbose and show debug infos.")
	testClientCmd.Flags().StringP("config", "c", "config.yaml", "Use custom config.")
	testClientCmd.Flags().StringP("clustername", "o", "", "Override clustername.")
}
