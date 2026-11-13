package cmd

import (
	"fmt"
	"mogenius-operator/src/bootstrap"
	"mogenius-operator/src/helm"
	"mogenius-operator/src/version"

	"github.com/alecthomas/kong"
)

var CLI struct {
	// Commands
	Cluster     struct{}        `cmd:"" help:"start the operator"`
	Nodemetrics nodeMetricsArgs `cmd:"" help:"start the node metrics collector"`
	Config      struct{}        `cmd:"" help:"print application config in ENV format"`
	System      struct{}        `cmd:"" help:"check the system for all required components and offer healing"`
	Version     struct{}        `cmd:"" help:"print version information" default:"1"`
	Exec        execArgs        `cmd:"" help:"open an interactive shell inside a container"`
	Logs        logArgs         `cmd:"" help:"retrieve streaming logs of a container"`
}

func Run() error {
	configModule := bootstrap.NewConfig()
	logs := bootstrap.NewLogging(configModule)
	slogManager := logs.Manager
	cmdLogger := logs.CmdLogger

	//===============================================================
	//========================= Parse Args ==========================
	//===============================================================
	ctx := kong.Parse(
		&CLI,
		kong.Name("mogenius-operator"),
		kong.Description("kubernetes operator for https://mogenius.com"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: false,
			Summary: true,
			Tree:    true,
		}),
	)

	//===============================================================
	//=================== Setup ENVs for Helm SDK ===================
	//===============================================================
	helm.InitEnvs(configModule)

	//===============================================================
	//======================= Execute Command =======================
	//===============================================================
	switch ctx.Command() {
	case "cluster":
		RunCluster(slogManager, configModule, cmdLogger, logs.RecordChannel)
		return nil
	case "nodemetrics":
		RunNodeMetrics(&CLI.Nodemetrics, configModule, logs)
		return nil
	case "system":
		err := RunSystem(slogManager, configModule, cmdLogger, logs.RecordChannel)
		if err != nil {
			return err
		}
		return nil
	case "version":
		versionModule := version.NewVersion()
		versionModule.PrintVersionInfo()
		return nil
	case "config":
		fmt.Println(configModule.AsEnvs())
		return nil
	case "exec <command>":
		err := RunExec(&CLI.Exec, cmdLogger, configModule)
		if err != nil {
			return err
		}
		return nil
	case "logs":
		err := RunLogs(&CLI.Logs, cmdLogger, configModule)
		if err != nil {
			return err
		}
		return nil
	default:
		return ctx.PrintUsage(true)
	}
}
