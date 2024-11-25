package kubernetes_test

import (
	"mogenius-k8s-manager/src/kubernetes"
	"testing"
)

// compile time check
func TestWatcherAdheresToInterface(t *testing.T) {
	t.Parallel()
	testfunc := func(w kubernetes.WatcherModule) {}
	testfunc(kubernetes.NewWatcher()) // this checks if the typesystem allows to call it
}
