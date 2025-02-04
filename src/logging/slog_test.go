package logging_test

import (
	"mogenius-k8s-manager/src/logging"
	"testing"
)

// compile time check
func TestSlogManagerAdheresToLogManagerInterface(t *testing.T) {
	t.Parallel()
	testfunc := func(w logging.LogManagerModule) {}
	logDir := t.TempDir()
	testfunc(logging.NewSlogManager(&logDir)) // this checks if the typesystem allows to call it
}

// compile time check
func TestMockSlogManagerAdheresToLogManagerInterface(t *testing.T) {
	t.Parallel()
	testfunc := func(w logging.LogManagerModule) {}
	testfunc(logging.NewMockSlogManager(t)) // this checks if the typesystem allows to call it
}
