/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/socketServer"
	"time"

	"github.com/spf13/cobra"
)

// testCmd represents the test command
var testClientCmd = &cobra.Command{
	Use:   "testclient",
	Short: "Print testServerCmd information and exit.",
	Long:  `Print testServerCmd information and exit.`,
	Run: func(cmd *cobra.Command, args []string) {
		for {
			socketServer.StartClient()
			logger.Log.Error("Connection closed. Reconnecting after waiting 1 second ...")
			time.Sleep(1 * time.Second)
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
}
