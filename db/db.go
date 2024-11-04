package db

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
	"mogenius-k8s-manager/shutdown"
)

var dbLogger *slog.Logger

func Setup(logManager interfaces.LogManager) {
	dbLogger = logManager.CreateLogger("db")

	shutdown.Add(func() {
		dbLogger.Debug("Shutting down...")
		close()
		dbLogger.Debug("done")
	})
}
