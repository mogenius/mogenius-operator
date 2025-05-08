package cmd

import (
	"log/slog"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/networkmonitor"
	"mogenius-k8s-manager/src/shutdown"
)

type nodeMetricsArgs struct {
	NetworkDevicePollRate uint64 `help:"" default:"1000"`
	MetricsRate           uint64 `help:"" default:"2000"`
}

func RunNodeMetrics(args *nodeMetricsArgs, logManagerModule logging.SlogManager, configModule *config.Config, cmdLogger *slog.Logger, valkeyLogChannel chan logging.LogLine) {
	go func() {
		for {
			select {
			case <-valkeyLogChannel:
			}
		}
	}()
	go func() {
		defer shutdown.SendShutdownSignal(true)
		configModule.Validate()

		systems := InitializeSystems(
			logManagerModule,
			configModule,
			cmdLogger,
			make(chan logging.LogLine), // logging to valkey is disabled -> this channel wont send anything
		)

		systems.core.InitializeClusterSecret()
		systems.core.InitializeValkey()

		systems.networkmonitor.SetSnoopyArgs(networkmonitor.SnoopyArgs{
			MetricsRate:           args.MetricsRate,
			NetworkDevicePollRate: args.NetworkDevicePollRate,
		})
		systems.nodeMetricsCollector.Run()

		select {}
	}()
	shutdown.Listen()
}
