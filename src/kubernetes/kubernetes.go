package kubernetes

import (
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeyclient"
	"mogenius-k8s-manager/src/websocket"
)

var config cfg.ConfigModule
var k8sLogger *slog.Logger
var watcher WatcherModule
var clientProvider k8sclient.K8sClientProvider
var valkeyClient valkeyclient.ValkeyClient

func Setup(
	logManagerModule logging.SlogManager,
	configModule cfg.ConfigModule,
	watcherModule WatcherModule,
	clientProviderModule k8sclient.K8sClientProvider,
	valkey valkeyclient.ValkeyClient,
) error {
	k8sLogger = logManagerModule.CreateLogger("kubernetes")
	config = configModule
	watcher = watcherModule
	clientProvider = clientProviderModule
	valkeyClient = valkey

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

func Start(eventClient websocket.WebsocketClient) error {
	err := WatchStoreResources(watcher, eventClient)
	if err != nil {
		return err
	}

	return nil
}
