package services

import (
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
)

var serviceLogger *slog.Logger
var config cfg.ConfigModule
var dbstats kubernetes.BoltDbStats

func Setup(logManagerModule logging.LogManagerModule, configModule cfg.ConfigModule, dbstatsModule kubernetes.BoltDbStats) {
	serviceLogger = logManagerModule.CreateLogger("services")
	config = configModule
	dbstats = dbstatsModule
}
