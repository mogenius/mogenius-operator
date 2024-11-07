package controllers

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var controllerLogger *slog.Logger

func Setup(logManager interfaces.LogManagerModule) {
	controllerLogger = logManager.CreateLogger("controllers")
}
