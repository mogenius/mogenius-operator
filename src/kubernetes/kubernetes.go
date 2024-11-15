package kubernetes

import (
	"log/slog"
	"mogenius-k8s-manager/src/interfaces"
)

var config interfaces.ConfigModule
var k8sLogger *slog.Logger

func Setup(logManagerModule interfaces.LogManagerModule, configModule interfaces.ConfigModule) {
	k8sLogger = logManagerModule.CreateLogger("kubernetes")
	config = configModule
}
