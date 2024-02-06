//lint:file-ignore ST1005 Error strings should not be capitalized is ignored throughout this file

package main

import (
	"embed"
	"mogenius-k8s-manager/cmd"
	"mogenius-k8s-manager/db"
	dbstats "mogenius-k8s-manager/db-stats"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
)

//go:embed config/config-local.yaml
var DefaultConfigLocalFile string

//go:embed config/config-cluster-dev.yaml
var DefaultConfigClusterFileDev string

//go:embed config/config-cluster-prod.yaml
var DefaultConfigClusterFileProd string

//go:embed yaml-templates
var YamlTemplatesFolder embed.FS

func main() {
	go func() {
		quit := make(chan os.Signal)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		logger.Log.Warning("Shutting down bbolt server...")
		db.Close()
		dbstats.Close()
	}()

	utils.PrintLogo()
	logger.Init()
	utils.DefaultConfigClusterFileDev = DefaultConfigClusterFileDev
	utils.DefaultConfigClusterFileProd = DefaultConfigClusterFileProd
	utils.DefaultConfigLocalFile = DefaultConfigLocalFile
	utils.YamlTemplatesFolder = YamlTemplatesFolder

	cmd.Execute()
}
