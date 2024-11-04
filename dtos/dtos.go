package dtos

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var logManager interfaces.LogManager
var dtosLogger *slog.Logger

func Setup(interfaceLogManager interfaces.LogManager) {
	logManager = interfaceLogManager
	dtosLogger = interfaceLogManager.CreateLogger("dtos")
}
