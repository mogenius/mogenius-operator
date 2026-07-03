package utils

import (
	"sync"
	"time"
)

// TTLCache memoizes fn for ttl. On expiry the first caller recomputes;
// other callers block on the mutex and then return the freshly-computed
// value, which collapses bursts of concurrent requests into one fn call
// (no thundering herd). Suitable for read-mostly, idempotent lookups
// behind WS/HTTP handlers where staleness up to ttl is acceptable.
type TTLCache[T any] struct {
	mu     sync.Mutex
	ttl    time.Duration
	fn     func() T
	value  T
	expiry time.Time
}

func NewTTLCache[T any](ttl time.Duration, fn func() T) *TTLCache[T] {
	return &TTLCache[T]{ttl: ttl, fn: fn}
}

func (c *TTLCache[T]) Get() T {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.expiry.IsZero() && time.Now().Before(c.expiry) {
		return c.value
	}
	c.value = c.fn()
	c.expiry = time.Now().Add(c.ttl)
	return c.value
}
