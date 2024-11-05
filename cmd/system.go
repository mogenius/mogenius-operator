package cmd

import (
	"mogenius-k8s-manager/services"

	"github.com/mogenius/punq/utils"

	"github.com/spf13/cobra"
)

var systemCmd = &cobra.Command{
	Use:   "system",
	Short: "All general system commands",
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check the system for all required components and offer healing",
	Run: func(cmd *cobra.Command, args []string) {
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
		services.SystemCheck()
	},
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Print information and exit",
	Long:  `Print information and exit`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.PrintSettings()
	},
}

func init() {
	rootCmd.AddCommand(systemCmd)
	systemCmd.AddCommand(infoCmd)
	systemCmd.AddCommand(checkCmd)
}
