package api

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var httpLogger *slog.Logger
var config interfaces.ConfigModule

func Setup(logManagerModule interfaces.LogManagerModule, configModule interfaces.ConfigModule) {
	httpLogger = logManagerModule.CreateLogger("http")
	config = configModule
}
