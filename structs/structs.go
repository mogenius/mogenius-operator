package structs

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var structsLogger *slog.Logger
var logManager interfaces.LogManagerModule

func Setup(logManagerModule interfaces.LogManagerModule) {
	logManager = logManagerModule
	structsLogger = logManager.CreateLogger("structs")
}
