package crds

import (
	"log/slog"
	"mogenius-k8s-manager/src/logging"
)

var crdLogger *slog.Logger

func Setup(logManagerModule logging.LogManagerModule) {
	crdLogger = logManagerModule.CreateLogger("crds")
}
