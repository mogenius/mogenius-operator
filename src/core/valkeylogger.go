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

// logPayloadValueLimit caps individual string values in a log record's
// payload before it is persisted to Valkey. The watcher in particular
// embeds full Kubernetes objects as JSON-in-JSON ("resourceJson":...),
// which produced 9+ MiB streams in production. 1 KiB per value keeps
// log lines diagnostically useful without letting any single payload
// blow up the stream's memory footprint.
const logPayloadValueLimit = 1024

func truncateLogPayload(payload map[string]any) {
	for k, v := range payload {
		s, ok := v.(string)
		if !ok || len(s) <= logPayloadValueLimit {
			continue
		}
		payload[k] = s[:logPayloadValueLimit] + "...[truncated]"
	}
}

func (self *valkeyLogger) Run() {
	go func() {
		for record := range self.logChannel {
			truncateLogPayload(record.Payload)
			err := self.valkey.StoreSortedListEntry(record, time.Now().UnixNano(), "logs", record.Component)
			if err != nil {
				slog.Error("Failed to log record to valkey", "error", err)
			}
		}
	}()
}
