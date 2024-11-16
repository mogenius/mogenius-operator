package cmd

import (
	"fmt"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/services"
	"mogenius-k8s-manager/src/version"

	"mogenius-k8s-manager/src/utils"

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
		cmdConfig.Validate()

		utils.PrintLogo()

		versionModule := version.NewVersion(slogManager)
		versionModule.PrintVersionInfo()
		cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

		preRun()

		services.SystemCheck()
	},
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Print information and exit",
	Long:  `Print information and exit`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmdConfig.AsEnvs())
	},
}

func init() {
	rootCmd.AddCommand(systemCmd)
	systemCmd.AddCommand(infoCmd)
	systemCmd.AddCommand(checkCmd)
}
