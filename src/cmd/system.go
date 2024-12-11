package cmd

import (
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/kubernetes"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/services"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"
)

func RunSystem(logManagerModule logging.LogManagerModule, configModule *config.Config, cmdLogger *slog.Logger) error {
	versionModule := version.NewVersion()
	versionModule.PrintVersionInfo()

	configModule.Validate()

	clientProvider := k8sclient.NewK8sClientProvider(logManagerModule.CreateLogger("client-provider"))
	watcherModule := kubernetes.NewWatcher(clientProvider)
	dbstatsModule, err := kubernetes.NewBoltDbStatsModule(configModule, logManagerModule.CreateLogger("db-stats"))
	assert.Assert(err == nil, err)

	err = mokubernetes.Setup(logManagerModule, configModule, watcherModule, clientProvider)
	assert.Assert(err == nil, err)
	services.Setup(logManagerModule, configModule, clientProvider, dbstatsModule)
	utils.Setup(logManagerModule, configModule)

	cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

	services.SystemCheck()

	return nil
}
