package cmd

import (
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/config"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/utils"
)

func RunCluster(logManagerModule logging.SlogManager, configModule *config.Config, cmdLogger *slog.Logger, valkeyLogChannel chan logging.LogLine) error {
	go func() {
		defer shutdown.SendShutdownSignal(true)
		configModule.Validate()

		systems := InitializeSystems(logManagerModule, configModule, cmdLogger, valkeyLogChannel)

		systems.versionModule.PrintVersionInfo()
		// due to the usage of TCX of ebpf we require at least version 6.6 of the linux kernel. otherwise we dont load the traffic collector.
		minimumKernelVersionFound, kernelErr := utils.ShouldLoadTrafficCollector("6.6")
		if kernelErr != nil {
			fmt.Printf("failed to determine the nodes kernel version", "error", kernelErr)
		}

		err := systems.core.Initialize()
		if err != nil {
			cmdLogger.Error("failed to initialize kubernetes resources", "error", err)
			return
		}

		cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

		systems.httpApi.Run()
		systems.socketApi.Run()
		systems.podStatsCollector.Run()
		if minimumKernelVersionFound {
			systems.trafficCollector.Run()
		}
		systems.valkeyLoggerService.Run()
		systems.dbstatsService.Run()
		systems.leaderElector.Run()

		// services have to be started before this otherwise watcher events will get missing
		err = mokubernetes.WatchStoreResources(systems.watcherModule, systems.eventConnectionClient)
		if err != nil {
			cmdLogger.Error("failed to start watcher", "error", err)
			return
		}

		cmdLogger.Info("SYSTEM STARTUP COMPLETE")
		select {}
	}()

	shutdown.Listen()

	return nil
}
