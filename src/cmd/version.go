package cmd

import (
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"
	"mogenius-k8s-manager/src/watcher"

	"github.com/fatih/color"
)

func RunVersion(logManagerModule interfaces.LogManagerModule, configModule *config.Config, cmdLogger *slog.Logger) error {
	assert.Assert(configModule != nil)
	assert.Assert(cmdLogger != nil)

	configModule.Validate()

	watcherModule := watcher.NewWatcher()

	mokubernetes.Setup(logManagerModule, configModule, watcherModule)
	utils.Setup(logManagerModule, configModule)

	utils.PrintLogo()

	versionModule := version.NewVersion(logManagerModule)
	versionModule.PrintVersionInfo()
	cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

	yellow := color.New(color.FgYellow).SprintFunc()
	fmt.Printf("CLI: \t\t%s\n", yellow(version.Ver))
	fmt.Printf("Container: \t%s\n", yellow(version.Ver))
	fmt.Printf("Branch: \t%s\n", yellow(version.Branch))
	fmt.Printf("Commit: \t%s\n", yellow(version.GitCommitHash))
	fmt.Printf("Timestamp: \t%s\n", yellow(version.BuildTimestamp))

	return nil
}
