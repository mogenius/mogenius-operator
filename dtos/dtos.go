package dtos

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var dtosLogger *slog.Logger

func Setup(logManager interfaces.LogManager) {
	dtosLogger = logManager.CreateLogger("dtos")
}
