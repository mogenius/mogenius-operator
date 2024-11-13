package crds

import (
	"log/slog"
	"mogenius-k8s-manager/src/interfaces"
)

var crdLogger *slog.Logger

func Setup(logManagerModule interfaces.LogManagerModule) {
	crdLogger = logManagerModule.CreateLogger("crds")
}
