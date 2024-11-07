package structs

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var structsLogger *slog.Logger
var logManager interfaces.LogManager

func Setup(interfaceLogManager interfaces.LogManager) {
	logManager = interfaceLogManager
	structsLogger = logManager.CreateLogger("structs")
}
