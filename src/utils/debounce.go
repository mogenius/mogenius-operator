package utils

import (
	"fmt"
	"sync"
	"time"
)

type DebounceEntry struct {
	result any
	err    *error
	done   chan struct{}
	timer  *time.Timer
}

type Debounce struct {
	name     string
	cacheTTL time.Duration
	timer    time.Duration
	cache    map[string]*DebounceEntry
	mutex    sync.Mutex
}

func NewDebounce(name string, cacheTTL time.Duration, timer time.Duration) *Debounce {
	return &Debounce{
		name:     name,
		cacheTTL: cacheTTL,
		timer:    timer,
		cache:    make(map[string]*DebounceEntry),
	}
}

func (d *Debounce) CallFn(key string, fn func() (any, error)) (any, *error) {
	key = fmt.Sprintf("%s-%s", d.name, key)
	d.mutex.Lock()

	if entry, found := d.cache[key]; found {
		if entry.timer != nil {
			entry.timer.Reset(d.timer)
		}
		d.mutex.Unlock()
		<-entry.done
		return entry.result, entry.err
	}

	entry := &DebounceEntry{
		done: make(chan struct{}),
	}
	d.cache[key] = entry
	d.mutex.Unlock()

	entry.timer = time.AfterFunc(d.timer, func() {
		result, err := fn()
		entry.result = result
		entry.err = &err

		d.mutex.Lock()
		if entry.done != nil {
			close(entry.done)
			entry.done = nil
		}
		d.mutex.Unlock()
	})

	<-entry.done

	go func() {
		d.mutex.Lock()
		delete(d.cache, key)
		d.mutex.Unlock()
	}()
	return entry.result, entry.err
}
