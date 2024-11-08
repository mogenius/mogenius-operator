package socketclient

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var socketClientLogger *slog.Logger

func Setup(logManagerModule interfaces.LogManagerModule) {
	socketClientLogger = logManagerModule.CreateLogger("socket-client")
}
