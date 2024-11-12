package socketclient

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var socketClientLogger *slog.Logger
var config interfaces.ConfigModule

func Setup(logManagerModule interfaces.LogManagerModule, configModule interfaces.ConfigModule) {
	socketClientLogger = logManagerModule.CreateLogger("socket-client")
	config = configModule
}
