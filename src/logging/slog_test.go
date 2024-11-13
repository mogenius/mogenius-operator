package logging_test

import (
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/logging"
	"testing"
)

// compile time check
func TestSlogerManagerAdheresToLogManagerInterface(t *testing.T) {
	slogManager := logging.NewSlogManager("logs")
	testfunc := func(w interfaces.LogManagerModule) {}
	testfunc(slogManager) // this checks if the typesystem allows to call it
}
