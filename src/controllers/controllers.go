package controllers

import (
	"log/slog"
	"mogenius-k8s-manager/src/logging"
)

var controllerLogger *slog.Logger

func Setup(logManagerModule logging.LogManagerModule) {
	controllerLogger = logManagerModule.CreateLogger("controllers")
}
