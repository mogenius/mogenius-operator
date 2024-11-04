package services

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var serviceLogger *slog.Logger

func Setup(logManager interfaces.LogManager) {
	serviceLogger = logManager.CreateLogger("services")
}
