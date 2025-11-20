package secrets

import (
	"mogenius-operator/src/assert"
	"mogenius-operator/src/collections"
	"mogenius-operator/src/config"
	"strings"
	"sync"
)

var (
	secrets           collections.HashSet[string] = collections.NewHashSet[string]()
	secretsLock       sync.RWMutex                = sync.RWMutex{}
	configSecrets     collections.HashSet[string] = collections.NewHashSet[string]()
	configSecretsLock sync.RWMutex                = sync.RWMutex{}
)

const REDACTED = "***[REDACTED]***"

func AddSecret(secret string) {
	if secret == "" {
		return
	}

	secretsLock.Lock()
	defer secretsLock.Unlock()

	secrets.Insert(secret)
}

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
	defer configSecretsLock.Unlock()

	configSecrets = newConfigSecrets
}

func SecretArray() []string {
	secretsLock.RLock()
	secretVals := secrets.Slice()
	secretsLock.RUnlock()

	configSecretsLock.RLock()
	configVals := configSecrets.Slice()
	configSecretsLock.RUnlock()

	data := collections.NewHashSet[string]()

	for _, secret := range secretVals {
		assert.Assert(secret != "", "there should never be an empty string as a secret")
		data.Insert(secret)
	}

	for _, secret := range configVals {
		assert.Assert(secret != "", "there should never be an empty string as a config value")
		data.Insert(secret)
	}

	return data.Slice()
}

func EraseSecrets(data string) string {
	for _, b := range SecretArray() {
		data = strings.ReplaceAll(data, b, REDACTED)
	}
	return data
}
