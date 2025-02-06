package services

import (
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
)

var serviceLogger *slog.Logger
var config cfg.ConfigModule
var clientProvider k8sclient.K8sClientProvider
var dbstats kubernetes.BoltDbStats

func Setup(
	logManagerModule logging.LogManagerModule,
	configModule cfg.ConfigModule,
	clientProviderModule k8sclient.K8sClientProvider,
	dbstatsModule kubernetes.BoltDbStats,
) {
	serviceLogger = logManagerModule.CreateLogger("services")
	config = configModule
	clientProvider = clientProviderModule
	dbstats = dbstatsModule
}
