package logging

import (
	"mogenius-k8s-manager/src/config"
	"slices"
	"sync"
)

var (
	secrets           []string     = []string{}
	secretsLock       sync.RWMutex = sync.RWMutex{}
	configSecrets     []string     = []string{}
	configSecretsLock sync.RWMutex = sync.RWMutex{}
)

const REDACTED = "***[REDACTED]***"

func AddSecret(secret string) {
	if secret == "" {
		return
	}

	secretsLock.Lock()
	defer secretsLock.Unlock()

	if !slices.Contains(secrets, secret) {
		secrets = append(secrets, secret)
	}
}

func UpdateConfigSecrets(configVariables []config.ConfigVariable) {
	newConfigSecrets := []string{}
	for _, cv := range configVariables {
		if cv.IsSecret && cv.Value != "" && !slices.Contains(newConfigSecrets, cv.Value) {
			newConfigSecrets = append(newConfigSecrets, cv.Value)
		}
	}

	configSecretsLock.Lock()
	defer configSecretsLock.Unlock()

	configSecrets = newConfigSecrets
}

func SecretArray() []string {
	var data []string

	secretsLock.RLock()
	for _, secret := range secrets {
		if secret != "" {
			data = append(data, secret)
		}
	}
	secretsLock.RUnlock()

	configSecretsLock.RLock()
	for _, secret := range configSecrets {
		if secret != "" {
			data = append(data, secret)
		}
	}
	configSecretsLock.RUnlock()

	return data
}
