/*
Copyright © 2022 mogenius, Benedikt Iltisberger
*/
package cmd

import (
	"fmt"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/utils"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	punqUtils "github.com/mogenius/punq/utils"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the application into your cluster without auto-removal",
	Long: `
	This cmd installs the application permanently into you cluster. 
	Please run cleanup if you want to remove it again.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmdConfig.Validate()

		err := slogManager.SetLogLevel(cmdConfig.Get("MO_LOG_LEVEL"))
		if err != nil {
			panic(err)
		}
		logFilter := cmdConfig.Get("MO_LOG_FILTER")
		err = slogManager.SetLogFilter(logFilter)
		if err != nil {
			panic(err)
		}
		utils.PrintLogo()
		preRun()
		yellow := color.New(color.FgYellow).SprintFunc()
		if !punqUtils.ConfirmTask(fmt.Sprintf("Do you realy want to install mogenius-k8s-manager to '%s' context?", yellow(kubernetes.CurrentContextName()))) {
			os.Exit(0)
		}

		kubernetes.Deploy()
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}
