package utils

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type DebounceEntry struct {
	result interface{}
	err    *error
	done   chan struct{}
}

type Debounce struct {
	name     string
	cacheTTL time.Duration
	cache    map[string]*DebounceEntry
	mutex    sync.Mutex
}

func NewDebounce(name string, cacheTTL time.Duration) *Debounce {
	return &Debounce{
		name:     name,
		cacheTTL: cacheTTL,
		cache:    make(map[string]*DebounceEntry),
	}
}

func (d *Debounce) CallFn(key string, fn func() (interface{}, error)) (interface{}, *error) {
	key = fmt.Sprintf("%s-%s", d.name, key)
	d.mutex.Lock()
	if entry, found := d.cache[key]; found {
		log.Infof("--- DEBOUNCED_CALL_FOR_KEY %s ---", key)
		d.mutex.Unlock()
		<-entry.done
		return entry.result, entry.err
	}

	entry := &DebounceEntry{done: make(chan struct{})}
	d.cache[key] = entry
	d.mutex.Unlock()

	go func() {
		result, err := fn()
		entry.result = result
		entry.err = &err
		close(entry.done)
	}()

	<-entry.done

	go func() {
		time.Sleep(d.cacheTTL)
		d.mutex.Lock()
		delete(d.cache, key)
		d.mutex.Unlock()
	}()
	return entry.result, entry.err
}
