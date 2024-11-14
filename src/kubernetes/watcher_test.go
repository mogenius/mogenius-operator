package kubernetes_test

import (
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/kubernetes"
	"testing"
)

// compile time check
func TestWatcherAdheresToInterface(t *testing.T) {
	t.Parallel()
	watcher := kubernetes.NewWatcher()
	testfunc := func(w interfaces.WatcherModule) {}
	testfunc(&watcher) // this checks if the typesystem allows to call it
}
