package controllers

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var controllerLogger *slog.Logger

func Setup(logManagerModule interfaces.LogManagerModule) {
	controllerLogger = logManagerModule.CreateLogger("controllers")
}
