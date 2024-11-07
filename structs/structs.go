package structs

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var structsLogger *slog.Logger
var logManager interfaces.LogManagerModule

func Setup(interfaceLogManager interfaces.LogManagerModule) {
	logManager = interfaceLogManager
	structsLogger = logManager.CreateLogger("structs")
}
