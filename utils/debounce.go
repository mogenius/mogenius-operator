package utils

import "sync"

type DebounceEntry struct {
	result interface{}
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

func (d *Debounce) CallFn(key string, fn func() interface{}) interface{} {
	d.mutex.Lock()
	if entry, found := d.cache[key]; found {
		d.mutex.Unlock()
		<-entry.done
		return entry.result
	}
	result := fn()

	entry := &DebounceEntry{done: make(chan struct{})}
	d.cache[key] = entry
	d.mutex.Unlock()

	entry.result = result

	close(entry.done)
	delete(d.cache, key)
	return result
}
