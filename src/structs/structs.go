package structs

import (
	"mogenius-operator/src/logging"
)

var logManager logging.SlogManager

func Setup(logManagerModule logging.SlogManager) {
	logManager = logManagerModule
}
