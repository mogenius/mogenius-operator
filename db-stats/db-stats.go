package dbstats

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var dbStatsLogger *slog.Logger

func Setup(logManager interfaces.LogManager) {
	dbStatsLogger = logManager.CreateLogger("db-stats")
}
