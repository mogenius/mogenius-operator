package core

import (
	"log/slog"
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
		for record := range self.logChannel {
			err := self.valkey.StoreSortedListEntry(record, time.Now().UnixNano(), "logs", record.Component)
			if err != nil {
				slog.Error("Failed to log record to valkey", "error", err)
			}
		}
	}()
}
