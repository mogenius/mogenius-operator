package kubernetes

import (
	"log/slog"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
)

var config cfg.ConfigModule
var k8sLogger *slog.Logger
var clientProvider k8sclient.K8sClientProvider
var valkeyClient valkeyclient.ValkeyClient

func Setup(
	logManagerModule logging.SlogManager,
	configModule cfg.ConfigModule,
	clientProviderModule k8sclient.K8sClientProvider,
	valkey valkeyclient.ValkeyClient,
) error {
	k8sLogger = logManagerModule.CreateLogger("kubernetes")
	config = configModule
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
