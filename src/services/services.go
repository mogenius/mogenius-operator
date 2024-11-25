package services

import (
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/logging"
)

var serviceLogger *slog.Logger
var config cfg.ConfigModule

func Setup(logManagerModule logging.LogManagerModule, configModule cfg.ConfigModule) {
	serviceLogger = logManagerModule.CreateLogger("services")
	config = configModule
}
