package api

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var httpLogger *slog.Logger

func Setup(logManager interfaces.LogManagerModule) {
	httpLogger = logManager.CreateLogger("http")
}
