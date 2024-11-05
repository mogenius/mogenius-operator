package logging_test

import (
	"mogenius-k8s-manager/interfaces"
	"mogenius-k8s-manager/logging"
	"testing"
)

// compile time check
func TestSlogerManagerAdheresToLogManagerInterface(t *testing.T) {
	slogManager := logging.NewSlogManager("logs")
	testfunc := func(w interfaces.LogManagerModule) {}
	testfunc(slogManager) // this checks if the typesystem allows to call it
}
