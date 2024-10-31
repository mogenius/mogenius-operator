package structs

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var structsLogger *slog.Logger

func Setup(logManager interfaces.LogManager) {
	structsLogger = logManager.CreateLogger("structs")
}
