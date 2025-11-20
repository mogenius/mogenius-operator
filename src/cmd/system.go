package cmd

import (
	"log/slog"
	"mogenius-operator/src/config"
	mokubernetes "mogenius-operator/src/kubernetes"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/services"
	"mogenius-operator/src/shutdown"
)

func RunSystem(logManagerModule logging.SlogManager, configModule *config.Config, cmdLogger *slog.Logger, valkeyLogChannel chan logging.LogLine) error {
	configModule.Validate()

	systems := InitializeSystems(logManagerModule, configModule, cmdLogger, valkeyLogChannel)
	defer shutdown.ExecuteShutdownHandlers()

	systems.versionModule.PrintVersionInfo()

	cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

	services.SystemCheck()

	return nil
}
