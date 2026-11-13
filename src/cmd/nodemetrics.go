package cmd

import (
	"mogenius-operator/src/bootstrap"
	"mogenius-operator/src/config"
	"mogenius-operator/src/nodemetrics"
)

type nodeMetricsArgs struct {
	MetricsRate uint64 `help:"" default:"2000"`
}

// RunNodeMetrics keeps the `nodemetrics` subcommand of the operator binary
// working (local development spawns it, see core.NodeMetricsOrchestrator).
// The DaemonSet runs the slim mogenius-nodemetrics binary instead.
func RunNodeMetrics(args *nodeMetricsArgs, configModule *config.Config, logs bootstrap.Logging) {
	nodemetrics.Run(configModule, logs, args.MetricsRate)
}
