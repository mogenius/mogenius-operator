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
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all components from your cluster",
	Long: `
	This cmd removes all remaining parts of the daemonset, configs, etc. from your cluster. 
	This can be used if something went wrong during automatic cleanup.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmdConfig.Validate()

		versionModule := version.NewVersion(slogManager)
		versionModule.PrintVersionInfo()
		cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

		preRun()

		yellow := color.New(color.FgYellow).SprintFunc()
		if !utils.ConfirmTask(fmt.Sprintf("Do you realy want to remove mogenius-k8s-manager from '%s' context?", yellow(kubernetes.CurrentContextName()))) {
			os.Exit(0)
		}

		kubernetes.Remove()
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}
