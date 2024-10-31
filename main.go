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
)

func main() {
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		db.Close()
		dbstats.Close()
		panic("shutting down due to received signal")
	}()

	utils.PrintLogo()
	cmd.Execute()
}
