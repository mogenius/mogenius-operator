package socketclient

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var socketClientLogger *slog.Logger

func Setup(logManager interfaces.LogManager) {
	socketClientLogger = logManager.CreateLogger("socket-client")
}
