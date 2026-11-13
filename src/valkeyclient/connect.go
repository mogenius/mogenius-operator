package valkeyclient

import (
	"fmt"
	"log/slog"
	"time"
)

// ConnectWithRetry connects the client, retrying with exponential backoff
// (2s, doubling up to 30s) until maxWait has passed. Valkey usually becomes
// reachable a little after the pods that use it start.
func ConnectWithRetry(client ValkeyClient, logger *slog.Logger, maxWait time.Duration) error {
	const (
		initialBackoff = 2 * time.Second
		maxBackoff     = 30 * time.Second
	)
	deadline := time.Now().Add(maxWait)
	backoff := initialBackoff
	for {
		err := client.Connect()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("connect to valkey, gave up after %s: %w", maxWait, err)
		}
		logger.Warn("failed to connect to valkey, retrying", "error", err, "backoff", backoff)
		time.Sleep(backoff)
		backoff = min(backoff*2, maxBackoff)
	}
}
