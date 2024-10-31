package kubernetes

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var helmLogger *slog.Logger
var k8sLogger *slog.Logger

func Setup(logManager interfaces.LogManager) {
	k8sLogger = logManager.CreateLogger("kubernetes")
	helmLogger = logManager.CreateLogger("helm")

}
