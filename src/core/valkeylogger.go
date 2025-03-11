package core

import (
	"fmt"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/valkeystore"
)

type ValkeyLogger interface{}

type valkeyLogger struct {
	store      valkeystore.ValkeyStore
	logChannel chan logging.LogLine
}

func NewValkeyLogger(store valkeystore.ValkeyStore, logChannel chan logging.LogLine) ValkeyLogger {
	self := &valkeyLogger{}

	self.store = store
	self.logChannel = logChannel

	go func() {
		for {
			record := <-self.logChannel
			fmt.Printf("Received record: %s\n", record.ToJson())
		}
	}()

	return self
}
