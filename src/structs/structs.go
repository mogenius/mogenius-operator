package structs

import (
	"mogenius-k8s-manager/src/logging"
)

var logManager logging.SlogManager

func Setup(logManagerModule logging.SlogManager) {
	logManager = logManagerModule
}
