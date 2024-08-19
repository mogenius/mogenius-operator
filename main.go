//lint:file-ignore ST1005 Error strings should not be capitalized is ignored throughout this file

package main

import (
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

	// DEFAULT LOGGING --- will be overwritten by utils when envvars and yamlconfig is loaded
	log.SetOutput(os.Stdout)
	log.SetLevel(log.TraceLevel)
	log.SetFormatter(&log.TextFormatter{
		ForceColors:      true,
		DisableTimestamp: false,
		DisableQuote:     true,
	})

	utils.PrintLogo()
	cmd.Execute()
}
