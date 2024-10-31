package api

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var httpLogger *slog.Logger

func Setup(logManager interfaces.LogManager) {
	httpLogger = logManager.CreateLogger("http")
}
