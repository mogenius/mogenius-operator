/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
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
