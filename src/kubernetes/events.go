package kubernetes

import (
	"time"
)

const RETRYTIMEOUT time.Duration = 3
const CONCURRENTCONNECTIONS = 1

var EventChannels = make(map[string]chan string)
