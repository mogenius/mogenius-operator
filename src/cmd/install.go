package cmd

import (
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/config"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/shell"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/utils"
	"os"
)

func RunInstall(logManagerModule logging.LogManagerModule, configModule *config.Config, cmdLogger *slog.Logger) error {
	configModule.Validate()

	systems := InitializeSystems(logManagerModule, configModule, cmdLogger)
	defer shutdown.ExecuteShutdownHandlers()

	systems.versionModule.PrintVersionInfo()

	cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

	if !utils.ConfirmTask(fmt.Sprintf("Do you really want to install mogenius-k8s-manager to '%s' context?", shell.Colorize(mokubernetes.CurrentContextName(), shell.Yellow))) {
		os.Exit(0)
	}

	mokubernetes.Deploy()

	return nil
}
