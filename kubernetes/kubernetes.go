package kubernetes

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var config interfaces.ConfigModule
var helmLogger *slog.Logger
var k8sLogger *slog.Logger

func Setup(logManagerModule interfaces.LogManagerModule, configModule interfaces.ConfigModule) {
	k8sLogger = logManagerModule.CreateLogger("kubernetes")
	helmLogger = logManagerModule.CreateLogger("helm")
	config = configModule
}
