package crds

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var crdLogger *slog.Logger

func Setup(logManager interfaces.LogManagerModule) {
	crdLogger = logManager.CreateLogger("crds")
}
