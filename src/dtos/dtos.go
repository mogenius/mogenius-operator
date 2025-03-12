package dtos

import (
	"log/slog"
	"mogenius-k8s-manager/src/logging"
)

var dtosLogger *slog.Logger

func Setup(logManagerModule logging.SlogManager) {
	dtosLogger = logManagerModule.CreateLogger("dtos")
}
