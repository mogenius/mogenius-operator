package kubernetes

import (
	"log/slog"
	"mogenius-k8s-manager/interfaces"
)

var config interfaces.ConfigModule
var helmLogger *slog.Logger
var k8sLogger *slog.Logger

func Setup(logManager interfaces.LogManagerModule, iConfig interfaces.ConfigModule) {
	k8sLogger = logManager.CreateLogger("kubernetes")
	helmLogger = logManager.CreateLogger("helm")
	config = iConfig
}
