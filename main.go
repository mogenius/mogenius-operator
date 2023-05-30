package main

import (
	"embed"
	_ "embed"
	"log"
	"mogenius-k8s-manager/cmd"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"net/http"
	_ "net/http/pprof"
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
	utils.PrintLogo()
	logger.Init()
	utils.DefaultConfigClusterFileDev = DefaultConfigClusterFileDev
	utils.DefaultConfigClusterFileProd = DefaultConfigClusterFileProd
	utils.DefaultConfigLocalFile = DefaultConfigLocalFile
	utils.YamlTemplatesFolder = YamlTemplatesFolder

	if utils.CONFIG.Misc.Debug {
		logger.Log.Warning("Starting serice for pprof in localhost:6060")
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	cmd.Execute()
}
