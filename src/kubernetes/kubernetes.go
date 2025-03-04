package kubernetes

import (
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/redisstore"
	"mogenius-k8s-manager/src/utils"
)

var config cfg.ConfigModule
var k8sLogger *slog.Logger
var watcher WatcherModule
var clientProvider k8sclient.K8sClientProvider
var redisStore redisstore.RedisStore

func Setup(
	logManagerModule logging.LogManagerModule,
	configModule cfg.ConfigModule,
	watcherModule WatcherModule,
	clientProviderModule k8sclient.K8sClientProvider,
	redisStoreModule redisstore.RedisStore,
) error {
	k8sLogger = logManagerModule.CreateLogger("kubernetes")
	config = configModule
	watcher = watcherModule
	clientProvider = clientProviderModule
	redisStore = redisStoreModule

	if utils.ClusterProviderCached == utils.UNKNOWN {
		foundProvider, err := GuessClusterProvider()
		if err != nil {
			k8sLogger.Error("GuessClusterProvider", "error", err)
		}
		utils.ClusterProviderCached = foundProvider
		k8sLogger.Debug("🎲 🎲 🎲 ClusterProvider", "foundProvider", string(foundProvider))
	}

	return nil
}

func Start() error {
	err := WatchStoreResources(watcher)
	if err != nil {
		return err
	}

	return nil
}
