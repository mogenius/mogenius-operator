package servicesExternal

import (
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
)

var config cfg.ConfigModule
var esoLogger *slog.Logger

func Setup(logManagerModule interfaces.LogManagerModule, configModule cfg.ConfigModule) {
	config = configModule
	esoLogger = logManagerModule.CreateLogger("eso")
}
