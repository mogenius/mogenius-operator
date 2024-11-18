package cmd

import (
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/shell"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"
	"os"
)

func RunInstall(logManagerModule interfaces.LogManagerModule, configModule *config.Config, cmdLogger *slog.Logger) error {
	versionModule := version.NewVersion()
	versionModule.PrintVersionInfo()

	configModule.Validate()

	utils.Setup(logManagerModule, configModule)

	cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

	if !utils.ConfirmTask(fmt.Sprintf("Do you really want to install mogenius-k8s-manager to '%s' context?", shell.Colorize(mokubernetes.CurrentContextName(), shell.Yellow))) {
		os.Exit(0)
	}

	mokubernetes.Deploy()

	return nil
}
