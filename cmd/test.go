/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "test.",
	Long:  `test.`,
	Run: func(cmd *cobra.Command, args []string) {
		// nothing
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
