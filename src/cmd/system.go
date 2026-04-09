package cmd

import (
	"log/slog"
	"mogenius-operator/src/config"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/shutdown"
)

func RunSystem(logManagerModule logging.SlogManager, configModule *config.Config, cmdLogger *slog.Logger, valkeyLogChannel chan logging.LogLine) error {
	configModule.Validate()

	base := initializeBaseSystems(logManagerModule, configModule, cmdLogger)
	defer shutdown.ExecuteShutdownHandlers()

	base.versionModule.PrintVersionInfo()

	return nil
}
