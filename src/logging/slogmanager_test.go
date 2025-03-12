package logging_test

import (
	"log/slog"
	"mogenius-k8s-manager/src/logging"
	"testing"
)

// compile time check
func TestSlogManagerAdheresToLogManagerInterface(t *testing.T) {
	t.Parallel()
	testfunc := func(w logging.SlogManager) {}
	testfunc(logging.NewSlogManager(slog.LevelInfo, []slog.Handler{})) // this checks if the typesystem allows to call it
}

// compile time check
func TestMockSlogManagerAdheresToLogManagerInterface(t *testing.T) {
	t.Parallel()
	testfunc := func(w logging.SlogManager) {}
	testfunc(logging.NewMockSlogManager(t)) // this checks if the typesystem allows to call it
}
