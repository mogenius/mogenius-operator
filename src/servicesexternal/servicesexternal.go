package servicesexternal

import (
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/logging"
)

var config cfg.ConfigModule
var esoLogger *slog.Logger

func Setup(logManagerModule logging.LogManagerModule, configModule cfg.ConfigModule) {
	config = configModule
	esoLogger = logManagerModule.CreateLogger("eso")
}
