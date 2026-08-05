package core

import (
	"fmt"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/valkeyclient"
	"os"
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
		// Failed writes are reported on stderr, NOT via slog: this goroutine
		// consumes the slog record channel, so logging through slog here
		// feeds every failure back into the very channel it is draining.
		// During a Valkey outage that loop, combined with the write timeout
		// per record, kept the channel permanently exhausted (MOG-4518).
		// Errors are also sampled — one line per interval instead of one per
		// record — because an outage fails every record with the same error.
		var (
			lastErrLog time.Time
			suppressed int
		)
		const errLogInterval = 10 * time.Second

		for record := range self.logChannel {
			truncateLogPayload(record.Payload)
			err := self.valkey.StoreSortedListEntry(record, time.Now().UnixNano(), "logs", record.Component)
			if err == nil {
				continue
			}
			if time.Since(lastErrLog) >= errLogInterval {
				fmt.Fprintf(os.Stderr, "[ERROR] Failed to write log record to valkey (%d more suppressed): %v\n", suppressed, err)
				lastErrLog = time.Now()
				suppressed = 0
			} else {
				suppressed++
			}
		}
	}()
}
