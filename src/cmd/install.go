package cmd

import (
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"
	"os"

	"github.com/fatih/color"
)

func RunInstall(logManagerModule interfaces.LogManagerModule, configModule *config.Config, cmdLogger *slog.Logger) error {
	configModule.Validate()

	utils.Setup(logManagerModule, configModule)

	utils.PrintLogo()

	versionModule := version.NewVersion(logManagerModule)
	versionModule.PrintVersionInfo()
	cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

	yellow := color.New(color.FgYellow).SprintFunc()
	if !utils.ConfirmTask(fmt.Sprintf("Do you really want to install mogenius-k8s-manager to '%s' context?", yellow(mokubernetes.CurrentContextName()))) {
		os.Exit(0)
	}

	mokubernetes.Deploy()

	return nil
}
