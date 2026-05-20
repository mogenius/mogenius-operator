package secrets

import (
	"mogenius-operator/src/assert"
	"mogenius-operator/src/collections"
	"mogenius-operator/src/config"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"
)

var (
	configSecrets     collections.HashSet[string] = collections.NewHashSet[string]()
	configSecretsLock sync.RWMutex                = sync.RWMutex{}

	// Cached pattern matching all known secret values. Rebuilt on
	// UpdateConfigSecrets, read lock-free from EraseSecrets so logging
	// doesn't pay regex-compile or slice-walk cost per log line.
	secretPattern atomic.Pointer[regexp.Regexp]
)

const REDACTED = "***[REDACTED]***"

func UpdateConfigSecrets(configVariables []config.ConfigVariable) {
	newConfigSecrets := collections.NewHashSet[string]()
	for _, cv := range configVariables {
		if !cv.IsSecret {
			continue
		}
		if cv.Value == "" {
			continue
		}
		newConfigSecrets.Insert(cv.Value)
	}

	configSecretsLock.Lock()
	configSecrets = newConfigSecrets
	values := newConfigSecrets.Slice()
	configSecretsLock.Unlock()

	secretPattern.Store(buildSecretPattern(values))
}

// buildSecretPattern compiles a single alternation regex from all secret
// values. Longest-first ordering avoids a shorter secret partially
// matching inside a longer one. Returns nil when no secrets are set;
// callers must handle that case.
func buildSecretPattern(values []string) *regexp.Regexp {
	if len(values) == 0 {
		return nil
	}
	sorted := make([]string, len(values))
	copy(sorted, values)
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })
	quoted := make([]string, 0, len(sorted))
	for _, v := range sorted {
		assert.Assert(v != "", "there should never be an empty string as a config value")
		quoted = append(quoted, regexp.QuoteMeta(v))
	}
	pattern := "(?:" + joinAlternation(quoted) + ")"
	return regexp.MustCompile(pattern)
}

func joinAlternation(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	n := len(parts) - 1
	for _, p := range parts {
		n += len(p)
	}
	buf := make([]byte, 0, n)
	for i, p := range parts {
		if i > 0 {
			buf = append(buf, '|')
		}
		buf = append(buf, p...)
	}
	return string(buf)
}

func SecretArray() []string {
	configSecretsLock.RLock()
	defer configSecretsLock.RUnlock()
	return configSecrets.Slice()
}

func EraseSecrets(data string) string {
	pat := secretPattern.Load()
	if pat == nil {
		return data
	}
	return pat.ReplaceAllString(data, REDACTED)
}
