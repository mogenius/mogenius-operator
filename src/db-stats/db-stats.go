package dbstats

import (
	"log/slog"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/shutdown"
)

var dbStatsLogger *slog.Logger

func Setup(logManagerModule interfaces.LogManagerModule) {
	dbStatsLogger = logManagerModule.CreateLogger("db-stats")

	shutdown.Add(func() {
		dbStatsLogger.Debug("Shutting down...")
		close()
		dbStatsLogger.Debug("done")
	})
}
