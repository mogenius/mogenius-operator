package cmd

import (
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"
	"mogenius-k8s-manager/src/watcher"

	"github.com/fatih/color"
)

func RunClean(logManagerModule interfaces.LogManagerModule, configModule *config.Config, cmdLogger *slog.Logger) error {
	configModule.Validate()

	utils.PrintLogo()

	watcherModule := watcher.NewWatcher()
	mokubernetes.Setup(logManagerModule, configModule, watcherModule)

	versionModule := version.NewVersion(logManagerModule)
	versionModule.PrintVersionInfo()
	cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

	yellow := color.New(color.FgYellow).SprintFunc()
	if !utils.ConfirmTask(fmt.Sprintf("Do you realy want to remove mogenius-k8s-manager from '%s' context?", yellow(mokubernetes.CurrentContextName()))) {
		return nil
	}

	mokubernetes.Remove()

	return nil
}
