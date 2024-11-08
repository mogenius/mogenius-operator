package crds

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var crdLogger *slog.Logger

func Setup(logManagerModule interfaces.LogManagerModule) {
	crdLogger = logManagerModule.CreateLogger("crds")
}
