package controllers

import (
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/logging"
)

var controllerLogger *slog.Logger

func Setup(logManagerModule logging.LogManagerModule, configModule cfg.ConfigModule) {
	controllerLogger = logManagerModule.CreateLogger("controllers")
	config = configModule
}
