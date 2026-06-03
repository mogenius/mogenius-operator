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
	// ExecuteShutdownHandlers runs the hooks in a goroutine and returns a
	// channel that closes when they finish. Block on it so handlers actually
	// complete instead of being killed when the process exits.
	defer func() { <-shutdown.ExecuteShutdownHandlers() }()

	base.versionModule.PrintVersionInfo()

	return nil
}
