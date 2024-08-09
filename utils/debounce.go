package utils

import (
	log "github.com/sirupsen/logrus"
	"reflect"
	"sync"
)

type DebounceEntry struct {
	result interface{}
	err    *error
	done   chan struct{}
}

type Debounce struct {
	cache map[string]*DebounceEntry
	mutex sync.Mutex
}

func NewDebounce() *Debounce {
	return &Debounce{
		cache: make(map[string]*DebounceEntry),
	}
}

func (d *Debounce) CallFn(key string, fn func() interface{}) (interface{}, *error) {
	d.mutex.Lock()
	if entry, found := d.cache[key]; found {
		log.Infof("DEBOUNCED_CALL_FOR_KEY %s", key)
		d.mutex.Unlock()
		<-entry.done
		return entry.result, entry.err
	}

	entry := &DebounceEntry{err: nil, done: make(chan struct{})}
	d.cache[key] = entry
	d.mutex.Unlock()

	go func() {
		fnValue := reflect.ValueOf(fn)
		results := fnValue.Call(nil)

		if len(results) > 0 {
			entry.result = results[0].Interface()
			if len(results) > 1 && !results[1].IsNil() {
				err := results[1].Interface().(error)
				entry.err = &err
			}
		}

		entry.result = fn()
		close(entry.done)
	}()

	<-entry.done

	defer func() {
		d.mutex.Lock()
		delete(d.cache, key)
		d.mutex.Unlock()
	}()

	return entry.result, entry.err
}
