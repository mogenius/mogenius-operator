//lint:file-ignore ST1005 Error strings should not be capitalized is ignored throughout this file

package main

import (
	"embed"
	"mogenius-k8s-manager/cmd"
	"mogenius-k8s-manager/db"
	dbstats "mogenius-k8s-manager/db-stats"
	"mogenius-k8s-manager/utils"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
)

//go:embed config/config-local.yaml
var DefaultConfigLocalFile string

//go:embed config/config-cluster-pre-dev.yaml
var DefaultConfigClusterFilePreDev string

//go:embed config/config-cluster-dev.yaml
var DefaultConfigClusterFileDev string

//go:embed config/config-cluster-prod.yaml
var DefaultConfigClusterFileProd string

//go:embed yaml-templates
var YamlTemplatesFolder embed.FS

func main() {
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Warning("Shutting down bbolt server...")
		db.Close()
		dbstats.Close()
		log.Warning("DB shutdown complete")
		os.Exit(0)
	}()

	// DEFAULT LOGGING
	log.SetOutput(os.Stdout)
	log.SetLevel(log.TraceLevel)
	log.AddHook(&utils.SecretRedactionHook{})
	log.SetFormatter(&log.TextFormatter{
		ForceColors:      true,
		DisableTimestamp: true,
		DisableQuote:     true,
	})

	utils.PrintLogo()

	utils.DefaultConfigClusterFilePreDev = DefaultConfigClusterFilePreDev
	utils.DefaultConfigClusterFileDev = DefaultConfigClusterFileDev
	utils.DefaultConfigClusterFileProd = DefaultConfigClusterFileProd
	utils.DefaultConfigLocalFile = DefaultConfigLocalFile
	utils.YamlTemplatesFolder = YamlTemplatesFolder

	cmd.Execute()
}
