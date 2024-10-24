/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"mogenius-k8s-manager/version"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information and exit",
	Long:  `Print version information and exit`,
	Run: func(cmd *cobra.Command, args []string) {
		yellow := color.New(color.FgYellow).SprintFunc()
		CmdLogger.Infof("CLI: \t\t%s\n", yellow(version.Ver))
		CmdLogger.Infof("Container: \t%s\n", yellow(version.Ver))
		CmdLogger.Infof("Branch: \t%s\n", yellow(version.Branch))
		CmdLogger.Infof("Commit: \t%s\n", yellow(version.GitCommitHash))
		CmdLogger.Infof("Timestamp: \t%s\n", yellow(version.BuildTimestamp))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
