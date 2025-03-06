package structs

import (
	"log/slog"
	"mogenius-k8s-manager/src/logging"
)

var structsLogger *slog.Logger
var logManager logging.LogManagerModule

func Setup(logManagerModule logging.LogManagerModule) {
	logManager = logManagerModule
	structsLogger = logManager.CreateLogger("structs")
}
