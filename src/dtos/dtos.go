package dtos

import (
	"log/slog"
	"mogenius-k8s-manager/src/interfaces"
)

var logManager interfaces.LogManagerModule
var dtosLogger *slog.Logger

func Setup(logManagerModule interfaces.LogManagerModule) {
	logManager = logManagerModule
	dtosLogger = logManagerModule.CreateLogger("dtos")
}
