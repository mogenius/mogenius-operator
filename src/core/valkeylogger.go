package core

import (
	"fmt"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/valkeyclient"
	"time"
)

type ValkeyLogger interface {
	Run()
}

type valkeyLogger struct {
	valkey     valkeyclient.ValkeyClient
	logChannel chan logging.LogLine
}

func NewValkeyLogger(valkey valkeyclient.ValkeyClient, logChannel chan logging.LogLine) ValkeyLogger {
	self := &valkeyLogger{}

	self.valkey = valkey
	self.logChannel = logChannel

	return self
}

func (self *valkeyLogger) Run() {
	go func() {
		for {
			record := <-self.logChannel
			// err := self.valkey.AddToBucket(10000, record, "logs", record.Component)
			err := self.valkey.StoreSortedListEntry(record, time.Now(), "logs", record.Component)
			if err != nil {
				fmt.Printf("Failed to log record: %v\n", err)
			}
		}
	}()
}
