package socketclient

import (
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/logging"
)

var socketClientLogger *slog.Logger
var config cfg.ConfigModule

func Setup(logManagerModule logging.LogManagerModule, configModule cfg.ConfigModule) {
	socketClientLogger = logManagerModule.CreateLogger("socket-client")
	config = configModule
}
