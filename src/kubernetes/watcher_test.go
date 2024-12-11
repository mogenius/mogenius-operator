package kubernetes_test

import (
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"testing"
)

// compile time check
func TestWatcherAdheresToInterface(t *testing.T) {
	t.Parallel()
	logManager := logging.NewMockSlogManager(t)
	clientProvider := k8sclient.NewK8sClientProvider(logManager.CreateLogger("client-provider"))
	testfunc := func(w kubernetes.WatcherModule) {}
	testfunc(kubernetes.NewWatcher(clientProvider)) // this checks if the typesystem allows to call it
}
