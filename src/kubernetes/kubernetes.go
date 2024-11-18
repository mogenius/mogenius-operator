package kubernetes

import (
	"log/slog"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/utils"
)

var config interfaces.ConfigModule
var k8sLogger *slog.Logger
var watcher interfaces.WatcherModule

func Setup(
	logManagerModule interfaces.LogManagerModule,
	configModule interfaces.ConfigModule,
	watcherModule interfaces.WatcherModule,
) {
	k8sLogger = logManagerModule.CreateLogger("kubernetes")
	config = configModule
	watcher = watcherModule

	if utils.ClusterProviderCached == utils.UNKNOWN {
		foundProvider, err := GuessClusterProvider()
		if err != nil {
			k8sLogger.Error("GuessClusterProvider", "error", err)
		}
		utils.ClusterProviderCached = foundProvider
		k8sLogger.Debug("🎲 🎲 🎲 ClusterProvider", "foundProvider", string(foundProvider))
	}
}

func Start() error {
	err := WatchStoreResources(watcher)
	if err != nil {
		return err
	}

	return nil
}
