package cmd

import (
	"log/slog"
	"mogenius-operator/src/config"
	mokubernetes "mogenius-operator/src/kubernetes"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/shutdown"
)

func RunSystem(logManagerModule logging.SlogManager, configModule *config.Config, cmdLogger *slog.Logger, valkeyLogChannel chan logging.LogLine) error {
	configModule.Validate()

	base := initializeBaseSystems(logManagerModule, configModule, cmdLogger)
	defer shutdown.ExecuteShutdownHandlers()

	base.versionModule.PrintVersionInfo()

	cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

	return nil
}
