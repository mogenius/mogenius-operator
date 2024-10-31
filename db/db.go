package db

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var dbLogger *slog.Logger

func Setup(logManager interfaces.LogManager) {
	dbLogger = logManager.CreateLogger("db")
}
