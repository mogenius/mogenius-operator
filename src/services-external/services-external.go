package servicesExternal

import (
	"log/slog"
	"mogenius-k8s-manager/src/interfaces"
)

var config interfaces.ConfigModule
var esoLogger *slog.Logger

func Setup(logManagerModule interfaces.LogManagerModule, configModule interfaces.ConfigModule) {
	config = configModule
	esoLogger = logManagerModule.CreateLogger("eso")
}
