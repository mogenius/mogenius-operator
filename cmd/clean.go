/*
Copyright © 2022 mogenius, Benedikt Iltisberger
*/
package cmd

import (
	"fmt"
	"mogenius-k8s-manager/kubernetes"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	punqUtils "github.com/mogenius/punq/utils"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all components from your cluster",
	Long: `
	This cmd removes all remaining parts of the daemonset, configs, etc. from your cluster. 
	This can be used if something went wrong during automatic cleanup.`,
	Run: func(cmd *cobra.Command, args []string) {
		preRun()
		yellow := color.New(color.FgYellow).SprintFunc()
		if !punqUtils.ConfirmTask(fmt.Sprintf("Do you realy want to remove mogenius-k8s-manager from '%s' context?", yellow(kubernetes.CurrentContextName()))) {
			os.Exit(0)
		}

		kubernetes.Remove()
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}
