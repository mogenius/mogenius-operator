package kubernetes_test

import (
	"testing"
)

// compile time check
func TestWatcherAdheresToInterface(t *testing.T) {

	// ############
	// git-runner does not have a valid kubeconfig. this leads to errors in the test
	// ############

	// t.Parallel()
	// logManager := logging.NewMockSlogManager(t)
	// clientProvider := k8sclient.NewK8sClientProvider(logManager.CreateLogger("client-provider"))
	// testfunc := func(w kubernetes.WatcherModule) {}
	// testfunc(kubernetes.NewWatcher(logManager.CreateLogger("watcher"), clientProvider)) // this checks if the typesystem allows to call it
}
