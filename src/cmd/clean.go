package cmd

import (
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/config"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/shell"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/utils"
)

func RunClean(logManagerModule logging.SlogManager, configModule *config.Config, cmdLogger *slog.Logger, valkeyLogChannel chan logging.LogLine) error {
	configModule.Validate()

	_ = InitializeSystems(logManagerModule, configModule, cmdLogger, valkeyLogChannel)
	defer shutdown.ExecuteShutdownHandlers()

	cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

	if !utils.ConfirmTask(fmt.Sprintf(
		"Do you realy want to remove mogenius-k8s-manager from '%s' context?",
		shell.Colorize(mokubernetes.CurrentContextName(), shell.Yellow),
	)) {
		return nil
	}

	mokubernetes.Remove()

	return nil
}
