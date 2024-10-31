package logging_test

import (
	"mogenius-k8s-manager/interfaces"
	"mogenius-k8s-manager/logging"
	"testing"
)

// compile time check
func TestWatcherAdheresToInterface(t *testing.T) {
	slogManager := logging.NewSlogManager()
	testfunc := func(w interfaces.LogManager) {}
	testfunc(&slogManager) // this checks if the typesystem allows to call it
}
