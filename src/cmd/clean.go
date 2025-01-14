package cmd

import (
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/kubernetes"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/shell"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"
)

func RunClean(logManagerModule logging.LogManagerModule, configModule *config.Config, cmdLogger *slog.Logger) error {
	versionModule := version.NewVersion()
	versionModule.PrintVersionInfo()

	configModule.Validate()

	clientProvider := k8sclient.NewK8sClientProvider(logManagerModule.CreateLogger("client-provider"))
	watcherModule := kubernetes.NewWatcher(logManagerModule.CreateLogger("watcher"), clientProvider)
	err := mokubernetes.Setup(logManagerModule, configModule, watcherModule, clientProvider)
	assert.Assert(err == nil, err)

	cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

	if !utils.ConfirmTask(fmt.Sprintf(
		"Do you realy want to remove mogenius-k8s-manager from '%s' context?",
		shell.Colorize(mokubernetes.CurrentContextName(), shell.Yellow),
	)) {
		return nil
	}

	mokubernetes.Remove()

	return nil
}
