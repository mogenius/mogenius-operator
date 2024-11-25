package services

import (
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
)

var serviceLogger *slog.Logger
var config cfg.ConfigModule

func Setup(logManagerModule interfaces.LogManagerModule, configModule cfg.ConfigModule) {
	serviceLogger = logManagerModule.CreateLogger("services")
	config = configModule
}
