package cmd

import (
	"log/slog"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/services"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"
)

func RunSystem(logManagerModule interfaces.LogManagerModule, configModule *config.Config, cmdLogger *slog.Logger) error {
	configModule.Validate()

	mokubernetes.Setup(logManagerModule, configModule)
	services.Setup(logManagerModule, configModule)
	utils.Setup(logManagerModule, configModule)

	utils.PrintLogo()

	versionModule := version.NewVersion(logManagerModule)
	versionModule.PrintVersionInfo()
	cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

	services.SystemCheck()

	return nil
}
