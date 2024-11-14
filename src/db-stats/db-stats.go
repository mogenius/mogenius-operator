package dbstats

import (
	"log/slog"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/shutdown"
)

var dbStatsLogger *slog.Logger
var config interfaces.ConfigModule

func Setup(logManagerModule interfaces.LogManagerModule, configModule interfaces.ConfigModule) {
	dbStatsLogger = logManagerModule.CreateLogger("db-stats")
	config = configModule

	shutdown.Add(func() {
		dbStatsLogger.Debug("Shutting down...")
		close()
		dbStatsLogger.Debug("done")
	})
}
