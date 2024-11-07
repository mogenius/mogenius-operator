package dbstats

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
	"mogenius-k8s-manager/shutdown"
)

var dbStatsLogger *slog.Logger

func Setup(logManager interfaces.LogManagerModule) {
	dbStatsLogger = logManager.CreateLogger("db-stats")

	shutdown.Add(func() {
		dbStatsLogger.Debug("Shutting down...")
		close()
		dbStatsLogger.Debug("done")
	})
}
