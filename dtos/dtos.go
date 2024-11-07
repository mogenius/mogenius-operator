package dtos

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var logManager interfaces.LogManagerModule
var dtosLogger *slog.Logger

func Setup(interfaceLogManager interfaces.LogManagerModule) {
	logManager = interfaceLogManager
	dtosLogger = interfaceLogManager.CreateLogger("dtos")
}
