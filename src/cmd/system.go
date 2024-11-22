package cmd

import (
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/services"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"
	"mogenius-k8s-manager/src/watcher"
)

func RunSystem(logManagerModule interfaces.LogManagerModule, configModule *config.Config, cmdLogger *slog.Logger) error {
	versionModule := version.NewVersion()
	versionModule.PrintVersionInfo()

	configModule.Validate()

	watcherModule := watcher.NewWatcher()

	err := mokubernetes.Setup(logManagerModule, configModule, watcherModule)
	assert.Assert(err == nil, err)
	services.Setup(logManagerModule, configModule)
	utils.Setup(logManagerModule, configModule)

	cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

	services.SystemCheck()

	return nil
}
