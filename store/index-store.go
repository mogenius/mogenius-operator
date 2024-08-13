package store

import (
	"sync"
)

type ReverseIndexStore struct {
	index map[string]map[string]struct{}
	mu    sync.RWMutex
}

func NewReverseIndexStore() *ReverseIndexStore {
	return &ReverseIndexStore{
		index: make(map[string]map[string]struct{}),
	}
}

func (s *ReverseIndexStore) AddCompositeKey(compositeKey string, parts ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, part := range parts {
		if s.index[part] == nil {
			s.index[part] = make(map[string]struct{})
		}
		s.index[part][compositeKey] = struct{}{}
	}
}

func (s *ReverseIndexStore) GetCompositeKeys(part string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	compositeKeysMap, exists := s.index[part]
	if !exists {
		return make([]string, 0)
	}

	compositeKeys := make([]string, 0, len(compositeKeysMap))
	for key := range compositeKeysMap {
		compositeKeys = append(compositeKeys, key)
	}

	return compositeKeys
}

func (s *ReverseIndexStore) DeleteCompositeKey(compositeKey string, parts ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, part := range parts {
		if compositeKeysMap, exists := s.index[part]; exists {
			delete(compositeKeysMap, compositeKey)
			if len(compositeKeysMap) == 0 {
				delete(s.index, part)
			}
		}
	}
}

func (s *ReverseIndexStore) DeletePartKey(part string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.index, part)
}
