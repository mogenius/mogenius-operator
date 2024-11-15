/*
Copyright © 2022 mogenius, Benedikt Iltisberger
*/
package cmd

import (
	"fmt"
	"mogenius-k8s-manager/src/kubernetes"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"
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

		utils.PrintLogo()
		versionModule := version.NewVersion(slogManager)
		versionModule.PrintVersionInfo()
		cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

		preRun()

		yellow := color.New(color.FgYellow).SprintFunc()
		if !punqUtils.ConfirmTask(fmt.Sprintf("Do you really want to install mogenius-k8s-manager to '%s' context?", yellow(kubernetes.CurrentContextName()))) {
			os.Exit(0)
		}

		kubernetes.Deploy()
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}
