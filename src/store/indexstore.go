package store

import (
	"fmt"
	"regexp"
	"strings"
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

func CreateKey(parts ...string) string {
	return strings.Join(parts, "___")
}

func CreateKeyPattern(groupVersion, kind, namespace, name *string) (*regexp.Regexp, error) {
	parts := make([]string, 4)

	if groupVersion != nil && *groupVersion != "" {
		parts[0] = regexp.QuoteMeta(*groupVersion)
	} else {
		parts[0] = `\S+`
	}

	if kind != nil && *kind != "" {
		parts[1] = regexp.QuoteMeta(*kind)
	} else {
		parts[1] = `\S+`
	}

	if namespace != nil && *namespace != "" {
		parts[2] = regexp.QuoteMeta(*namespace)
	} else {
		parts[2] = `\S+`
	}

	if name != nil && *name != "" {
		parts[3] = regexp.QuoteMeta(*name)
	} else {
		parts[3] = `\S+`
	}

	pattern := fmt.Sprintf(`^%s$`, strings.Join(parts, "___"))
	return regexp.Compile(pattern)
}
