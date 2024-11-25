package structs

import (
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/logging"
)

var structsLogger *slog.Logger
var logManager logging.LogManagerModule
var config cfg.ConfigModule

func Setup(logManagerModule logging.LogManagerModule, configModule cfg.ConfigModule) {
	logManager = logManagerModule
	structsLogger = logManager.CreateLogger("structs")
	config = configModule
}
