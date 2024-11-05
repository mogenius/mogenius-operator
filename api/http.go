package api

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var httpLogger *slog.Logger
var config interfaces.ConfigModule

func Setup(logManager interfaces.LogManagerModule, configModule interfaces.ConfigModule) {
	httpLogger = logManager.CreateLogger("http")
	config = configModule
}
