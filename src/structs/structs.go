package structs

import (
	"log/slog"
	"mogenius-k8s-manager/src/interfaces"
)

var structsLogger *slog.Logger
var logManager interfaces.LogManagerModule
var config interfaces.ConfigModule

func Setup(logManagerModule interfaces.LogManagerModule, configModule interfaces.ConfigModule) {
	logManager = logManagerModule
	structsLogger = logManager.CreateLogger("structs")
	config = configModule
}
