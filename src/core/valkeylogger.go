package core

import (
	"fmt"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/valkeyclient"
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
			err := self.valkey.StoreSortedListEntry(record, time.Now().UnixNano(), "logs", record.Component)
			if err != nil {
				fmt.Printf("Failed to log record: %v\n", err)
			}
		}
	}()
}
