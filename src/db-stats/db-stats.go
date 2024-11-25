package dbstats

import (
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/shutdown"
)

var dbStatsLogger *slog.Logger
var config cfg.ConfigModule

func Setup(logManagerModule interfaces.LogManagerModule, configModule cfg.ConfigModule) {
	dbStatsLogger = logManagerModule.CreateLogger("db-stats")
	config = configModule

	shutdown.Add(func() {
		dbStatsLogger.Debug("Shutting down...")
		close()
		dbStatsLogger.Debug("done")
	})
}
