package watcher_test

import (
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/watcher"
	"testing"
)

// compile time check
func TestWatcherAdheresToInterface(t *testing.T) {
	t.Parallel()
	testfunc := func(w interfaces.WatcherModule) {}
	testfunc(watcher.NewWatcher()) // this checks if the typesystem allows to call it
}
