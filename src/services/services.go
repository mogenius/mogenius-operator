package services

import (
	"log/slog"
	"mogenius-k8s-manager/src/interfaces"
)

var serviceLogger *slog.Logger
var config interfaces.ConfigModule

func Setup(logManagerModule interfaces.LogManagerModule, configModule interfaces.ConfigModule) {
	serviceLogger = logManagerModule.CreateLogger("services")
	config = configModule
}
