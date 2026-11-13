// Command mogenius-nodemetrics is the node-metrics DaemonSet of the
// mogenius operator as a binary of its own. It links only what node-metrics
// needs (Kubernetes client, Valkey, /proc and snoopy readers) instead of the
// whole operator, which cuts the binary from ~134 MB to ~35 MB and with it
// the memory every node pays for mapped code and package initialization.
package main

import (
	"mogenius-operator/src/bootstrap"
	"mogenius-operator/src/nodemetrics"
	"mogenius-operator/src/version"
	"os"

	"github.com/alecthomas/kong"

	// Derive GOMEMLIMIT from the container's cgroup memory limit, like the
	// operator binary does (see src/main.go).
	_ "github.com/KimMachineGun/automemlimit"
)

var cli struct {
	MetricsRate uint64 `help:"interval in milliseconds at which snoopy reads its eBPF counters" default:"2000"`
	Version     bool   `help:"print version information and exit"`
}

func main() {
	configModule := bootstrap.NewConfig()
	logs := bootstrap.NewLogging(configModule)

	kong.Parse(
		&cli,
		kong.Name("mogenius-nodemetrics"),
		kong.Description("node metrics collector of the mogenius operator"),
		kong.UsageOnError(),
	)
	if cli.Version {
		version.NewVersion().PrintVersionInfo()
		os.Exit(0)
	}

	nodemetrics.Run(configModule, logs, cli.MetricsRate)
	os.Exit(0)
}
