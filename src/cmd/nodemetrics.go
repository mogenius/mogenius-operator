package cmd

import (
	"log/slog"
	"mogenius-k8s-manager/src/config"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/shutdown"
)

func RunNodeMetrics(logManagerModule logging.SlogManager, configModule *config.Config, cmdLogger *slog.Logger) {
	go func() {
		defer shutdown.SendShutdownSignal(true)
		configModule.Validate()

		systems := InitializeSystems(
			logManagerModule,
			configModule,
			cmdLogger,
			make(chan logging.LogLine), // logging to valkey is disabled -> this channel wont send anything
		)

		systems.versionModule.PrintVersionInfo()

		err := systems.core.Initialize()
		if err != nil {
			cmdLogger.Error("failed to initialize kubernetes resources", "error", err)
			return
		}

		cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

		systems.nodeMetricsCollector.Run()

		cmdLogger.Info("SYSTEM STARTUP COMPLETE")
		select {}
	}()
	shutdown.Listen()
}
