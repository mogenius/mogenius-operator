package watcher_test

import (
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/watcher"
	"testing"
)

// compile time check
func TestWatcherAdheresToInterface(t *testing.T) {
	t.Parallel()
	w := watcher.NewWatcher()
	testfunc := func(w interfaces.WatcherModule) {}
	testfunc(&w) // this checks if the typesystem allows to call it
}
