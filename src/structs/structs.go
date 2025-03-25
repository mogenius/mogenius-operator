package structs

import (
	"log/slog"
	"mogenius-k8s-manager/src/logging"
)

var structsLogger *slog.Logger
var logManager logging.SlogManager

func Setup(logManagerModule logging.SlogManager) {
	logManager = logManagerModule
	structsLogger = logManager.CreateLogger("structs")
}
