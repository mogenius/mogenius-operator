package db

import (
	"log/slog"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/shutdown"
)

var dbLogger *slog.Logger

func Setup(logManagerModule interfaces.LogManagerModule) {
	dbLogger = logManagerModule.CreateLogger("db")

	shutdown.Add(func() {
		dbLogger.Debug("Shutting down...")
		close()
		dbLogger.Debug("done")
	})
}
