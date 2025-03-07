package cmd

import (
	"log/slog"
	"mogenius-k8s-manager/src/config"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/services"
	"mogenius-k8s-manager/src/shutdown"
)

func RunSystem(logManagerModule logging.LogManagerModule, configModule *config.Config, cmdLogger *slog.Logger) error {
	configModule.Validate()

	systems := InitializeSystems(logManagerModule, configModule, cmdLogger)
	defer shutdown.ExecuteShutdownHandlers()

	systems.versionModule.PrintVersionInfo()

	cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

	services.SystemCheck()

	return nil
}
