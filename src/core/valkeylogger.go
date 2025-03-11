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
			err := self.store.AddToBucket(10000, record, "logs", record.Component)
			if err != nil {
				fmt.Printf("Failed to log record: %v\n", err)
			}
		}
	}()

	return self
}
