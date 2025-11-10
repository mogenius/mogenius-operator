package cmd

import (
	"log/slog"
	"mogenius-operator/src/config"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/networkmonitor"
	"mogenius-operator/src/shutdown"
)

type nodeMetricsArgs struct {
	NetworkDevicePollRate uint64 `help:"" default:"1000"`
	MetricsRate           uint64 `help:"" default:"2000"`
}

func RunNodeMetrics(args *nodeMetricsArgs, logManagerModule logging.SlogManager, configModule *config.Config, cmdLogger *slog.Logger, valkeyLogChannel chan logging.LogLine) {
	go func() {
		for range valkeyLogChannel {
			// throw away
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

		systems.networkmonitor.Snoopy().SetArgs(networkmonitor.SnoopyArgs{
			MetricsRate:           args.MetricsRate,
			NetworkDevicePollRate: args.NetworkDevicePollRate,
		})
		systems.nodeMetricsCollector.Run()

		select {}
	}()
	shutdown.Listen()
}
