package cmd

import (
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/containerenumerator"
	"mogenius-operator/src/core"
	"mogenius-operator/src/cpumonitor"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/networkmonitor"
	"mogenius-operator/src/rammonitor"
	"mogenius-operator/src/shutdown"
	"mogenius-operator/src/store"
)

type nodeMetricsArgs struct {
	MetricsRate uint64 `help:"" default:"2000"`
}

// nodeMetricsSystems holds the services needed for the nodemetrics subcommand.
// It skips WebSocket clients, AI, ArgoCD, Helm, xterm, HTTP/Socket API, reconciler,
// pod-stats collector, and other cluster-mode-only subsystems.
type nodeMetricsSystems struct {
	core                 core.Core
	networkmonitor       networkmonitor.NetworkMonitor
	nodeMetricsCollector core.NodeMetricsCollector
}

// initializeNodeMetricsSystems layers the metrics-specific services on top of the shared base.
func initializeNodeMetricsSystems(
	base baseSystems,
	logManagerModule logging.SlogManager,
	configModule *config.Config,
) nodeMetricsSystems {
	assert.Assert(logManagerModule != nil)
	assert.Assert(configModule != nil)
	assert.Assert(base.valkeyClient != nil)
	assert.Assert(base.clientProvider != nil)
	assert.Assert(base.logger != nil)

	containerEnumerator := containerenumerator.NewContainerEnumerator(logManagerModule.CreateLogger("container-enumerator"), configModule, base.clientProvider)
	cpuMonitor := cpumonitor.NewCpuMonitor(logManagerModule.CreateLogger("cpu-monitor"), configModule, base.clientProvider, containerEnumerator)
	ramMonitor := rammonitor.NewRamMonitor(logManagerModule.CreateLogger("ram-monitor"), configModule, base.clientProvider, containerEnumerator)
	networkMonitor := networkmonitor.NewNetworkMonitor(logManagerModule.CreateLogger("network-monitor"), configModule, containerEnumerator, configModule.Get("MO_HOST_PROC_PATH"))

	ownerCacheService := store.NewOwnerCacheService(logManagerModule.CreateLogger("owner-cache"), configModule)
	dbstatsService := core.NewValkeyStatsModule(logManagerModule.CreateLogger("db-stats"), configModule, base.valkeyClient, ownerCacheService)

	moKubernetes := core.NewMoKubernetes(logManagerModule.CreateLogger("mokubernetes"), configModule, base.clientProvider)
	moKubernetes.Link(dbstatsService)

	// Websocket clients are not used by InitializeClusterSecret or InitializeValkey.
	mocore := core.NewCore(logManagerModule.CreateLogger("core"), configModule, base.clientProvider, base.valkeyClient, nil, nil)
	mocore.Link(moKubernetes)

	leaderElector := core.NewLeaderElector(logManagerModule.CreateLogger("leader-elector"), configModule, base.clientProvider)

	nodeMetricsCollector := core.NewNodeMetricsCollector(
		logManagerModule.CreateLogger("traffic-collector"),
		configModule,
		base.clientProvider,
		cpuMonitor,
		ramMonitor,
		networkMonitor,
	)
	nodeMetricsCollector.Link(dbstatsService, leaderElector)

	return nodeMetricsSystems{
		core:                 mocore,
		networkmonitor:       networkMonitor,
		nodeMetricsCollector: nodeMetricsCollector,
	}
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

		base := initializeBaseSystems(logManagerModule, configModule, cmdLogger)
		systems := initializeNodeMetricsSystems(base, logManagerModule, configModule)

		systems.core.InitializeClusterSecret()
		systems.core.InitializeValkey()

		systems.networkmonitor.Snoopy().SetArgs(networkmonitor.SnoopyArgs{
			MetricsRate: args.MetricsRate,
		})
		systems.nodeMetricsCollector.Run()

		select {}
	}()
	shutdown.Listen()
}
