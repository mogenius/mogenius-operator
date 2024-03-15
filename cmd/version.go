/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"mogenius-k8s-manager/version"

	"github.com/fatih/color"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information and exit.",
	Long:  `Print version information and exit.`,
	Run: func(cmd *cobra.Command, args []string) {
		yellow := color.New(color.FgYellow).SprintFunc()
		log.Infof("CLI: \t\t%s\n", yellow(version.Ver))
		log.Infof("Container: \t%s\n", yellow(version.Ver))
		log.Infof("Branch: \t%s\n", yellow(version.Branch))
		log.Infof("Commit: \t%s\n", yellow(version.GitCommitHash))
		log.Infof("Timestamp: \t%s\n", yellow(version.BuildTimestamp))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
