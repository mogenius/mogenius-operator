/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information and exit",
	Long:  `Print version information and exit`,
	Run: func(cmd *cobra.Command, args []string) {
		cmdConfig.Validate()

		utils.PrintLogo()

		versionModule := version.NewVersion(slogManager)
		versionModule.PrintVersionInfo()
		cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

		preRun()

		yellow := color.New(color.FgYellow).SprintFunc()
		fmt.Printf("CLI: \t\t%s\n", yellow(version.Ver))
		fmt.Printf("Container: \t%s\n", yellow(version.Ver))
		fmt.Printf("Branch: \t%s\n", yellow(version.Branch))
		fmt.Printf("Commit: \t%s\n", yellow(version.GitCommitHash))
		fmt.Printf("Timestamp: \t%s\n", yellow(version.BuildTimestamp))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
