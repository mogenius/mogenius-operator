package main

import (
	_ "embed"
	"mogenius-k8s-manager/cmd"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
)

//go:embed .env/production.env
var defaultEnvFile string

func main() {
	utils.DefaultEnvFile = defaultEnvFile
	logger.Init()
	utils.LoadDotEnv()
	cmd.Execute()
}
