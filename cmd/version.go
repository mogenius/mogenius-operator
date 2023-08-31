/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"mogenius-k8s-manager/version"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information and exit.",
	Long:  `Print version information and exit.`,
	Run: func(cmd *cobra.Command, args []string) {
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
