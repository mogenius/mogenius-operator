package dtos

import (
	"log/slog"
	"mogenius-k8s-manager/src/logging"
)

var logManager logging.LogManagerModule
var dtosLogger *slog.Logger

func Setup(logManagerModule logging.LogManagerModule) {
	logManager = logManagerModule
	dtosLogger = logManagerModule.CreateLogger("dtos")
}
