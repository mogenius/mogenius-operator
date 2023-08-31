/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	punqUtils "github.com/mogenius/punq/utils"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "test.",
	Long:  `test.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(punqUtils.InitConfigMapYaml())
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
