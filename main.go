package main

import (
	_ "embed"
	"mogenius-k8s-manager/cmd"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
)

//go:embed config/config.yaml
var DefaultConfigFile string

func main() {
	utils.DefaultConfigFile = DefaultConfigFile
	logger.Init()
	utils.InitConfigYaml()
	cmd.Execute()
}
