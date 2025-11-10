package services

import (
	"log/slog"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/logging"
)

var serviceLogger *slog.Logger
var config cfg.ConfigModule
var clientProvider k8sclient.K8sClientProvider

func Setup(
	logManagerModule logging.SlogManager,
	configModule cfg.ConfigModule,
	clientProviderModule k8sclient.K8sClientProvider,
) {
	serviceLogger = logManagerModule.CreateLogger("services")
	config = configModule
	clientProvider = clientProviderModule
}
