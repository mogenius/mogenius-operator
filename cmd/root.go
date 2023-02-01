/*
Copyright © 2022 mogenius, Benedikt Iltisberger
*/
package cmd

import (
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"os"

	cc "github.com/ivanpirog/coloredcobra"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mogenius-k8s-manager",
	Short: "Control your kubernetes cluster the easy way.",
	Long: `
Use mogenius-k8s-manager to control your kubernetes cluster. 🚀`,
	Run: func(cmd *cobra.Command, args []string) {
		reset, _ := cmd.Flags().GetBool("reset")
		if reset {
			logger.Log.Notice("Resetting config yaml to dafault values.")
			utils.WriteDefaultConfig()
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cc.Init(&cc.Config{
		RootCmd:  rootCmd,
		Headings: cc.HiCyan + cc.Bold + cc.Underline,
		Commands: cc.HiYellow + cc.Bold,
		Example:  cc.Italic,
		ExecName: cc.Bold,
		Flags:    cc.Bold,
	})

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.mogenius-k8s-manager.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.

	rootCmd.Flags().BoolP("reset", "r", false, "Reset Config YAML File '~/.mogenius-k8s-manager/config.yaml'.")
}
