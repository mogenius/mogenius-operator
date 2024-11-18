package kubernetes

import (
	"log/slog"
	"mogenius-k8s-manager/src/interfaces"
)

var config interfaces.ConfigModule
var k8sLogger *slog.Logger
var watcher interfaces.WatcherModule

func Setup(
	logManagerModule interfaces.LogManagerModule,
	configModule interfaces.ConfigModule,
	watcherModule interfaces.WatcherModule,
) {
	k8sLogger = logManagerModule.CreateLogger("kubernetes")
	config = configModule
	watcher = watcherModule
}

func Start() error {
	err := WatchStoreResources(watcher)
	if err != nil {
		return err
	}

	return nil
}
