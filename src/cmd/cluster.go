package cmd

import (
	"log/slog"
	"mogenius-operator/src/config"
	mokubernetes "mogenius-operator/src/kubernetes"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/shutdown"
)

func RunCluster(logManagerModule logging.SlogManager, configModule *config.Config, cmdLogger *slog.Logger, valkeyLogChannel chan logging.LogLine) {
	// f, perr := os.Create("cpu-cluster-" + strconv.FormatInt(time.Now().Unix(), 10) + ".pprof")
	// assert.Assert(perr == nil, perr)
	// pprof.StartCPUProfile(f)
	// shutdown.Add(pprof.StopCPUProfile)

	go func() {
		defer shutdown.SendShutdownSignal(true)
		configModule.Validate()

		systems := InitializeSystems(logManagerModule, configModule, cmdLogger, valkeyLogChannel)

		systems.versionModule.PrintVersionInfo()

		err := systems.core.Initialize()
		if err != nil {
			cmdLogger.Error("failed to initialize kubernetes resources", "error", err)
			return
		}

		cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

		systems.httpApi.Run()
		systems.socketApi.Run()
		systems.podStatsCollector.Run()
		systems.nodeMetricsCollector.Orchestrate()
		systems.valkeyLoggerService.Run()
		systems.dbstatsService.Run()
		systems.reconciler.Run()
		systems.leaderElector.Run()

		// services have to be started before this otherwise watcher events will get missing
		err = mokubernetes.WatchStoreResources(systems.watcherModule, systems.eventConnectionClient)
		if err != nil {
			cmdLogger.Error("failed to start watcher", "error", err)
			return
		}

		cmdLogger.Info("SYSTEM STARTUP COMPLETE")

		// connect socket after everything is ready
		systems.core.InitializeWebsocketEventServer()
		systems.core.InitializeWebsocketApiServer()

		select {}
	}()

	shutdown.Listen()
}
