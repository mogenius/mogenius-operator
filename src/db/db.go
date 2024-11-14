package db

import (
	"log/slog"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/shutdown"
)

var dbLogger *slog.Logger
var config interfaces.ConfigModule

func Setup(logManagerModule interfaces.LogManagerModule, configModule interfaces.ConfigModule) {
	dbLogger = logManagerModule.CreateLogger("db")
	config = configModule

	shutdown.Add(func() {
		dbLogger.Debug("Shutting down...")
		close()
		dbLogger.Debug("done")
	})
}
