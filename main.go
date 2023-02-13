package main

import (
	_ "embed"
	"mogenius-k8s-manager/cmd"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
)

//go:embed config/config-local.yaml
var DefaultConfigLocalFile string

//go:embed config/config-cluster.yaml
var DefaultConfigClusterFile string

func main() {
	utils.PrintLogo()
	logger.Init()
	utils.DefaultConfigClusterFile = DefaultConfigClusterFile
	utils.DefaultConfigLocalFile = DefaultConfigLocalFile
	cmd.Execute()
}
