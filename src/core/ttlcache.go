package core

import (
	"sync"
	"time"
)

// ttlCache memoizes fn for ttl. On expiry the first caller recomputes;
// other callers block on the mutex and then return the freshly-computed
// value, which collapses bursts of concurrent requests into one fn call
// (no thundering herd). Suitable for read-mostly, idempotent lookups
// behind WS/HTTP handlers where staleness up to ttl is acceptable.
type ttlCache[T any] struct {
	mu     sync.Mutex
	ttl    time.Duration
	fn     func() T
	value  T
	expiry time.Time
}

func newTTLCache[T any](ttl time.Duration, fn func() T) *ttlCache[T] {
	return &ttlCache[T]{ttl: ttl, fn: fn}
}

func (c *ttlCache[T]) Get() T {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.expiry.IsZero() && time.Now().Before(c.expiry) {
		return c.value
	}
	c.value = c.fn()
	c.expiry = time.Now().Add(c.ttl)
	return c.value
}
