package structs

import (
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
)

var structsLogger *slog.Logger
var logManager interfaces.LogManagerModule
var config cfg.ConfigModule

func Setup(logManagerModule interfaces.LogManagerModule, configModule cfg.ConfigModule) {
	logManager = logManagerModule
	structsLogger = logManager.CreateLogger("structs")
	config = configModule
}
