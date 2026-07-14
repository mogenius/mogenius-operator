package cmd

import (
	"mogenius-operator/src/config"
	"testing"

	"github.com/stretchr/testify/assert"
)

// requiredEnvKeys are the config keys that installations must provide via the
// environment (Helm chart or .env) and are therefore allowed to have no
// DefaultValue. Config.Validate() exits the process for every declared key
// that is neither set nor has a default, so adding a key to this list is a
// breaking change for every existing installation: their charts do not set
// the new env var yet and the operator crash-loops on upgrade. New optional
// keys must declare a DefaultValue instead.
var requiredEnvKeys = map[string]bool{
	"MO_API_KEY":         true,
	"MO_API_SERVER":      true,
	"MO_CLUSTER_MFA_ID":  true,
	"MO_CLUSTER_NAME":    true,
	"MO_EVENT_SERVER":    true,
	"MO_VALKEY_ADDR":     true,
	"MO_VALKEY_PASSWORD": true,
}

func TestNewConfigKeysHaveDefaults(t *testing.T) {
	configModule := config.NewConfig()
	LoadConfigDeclarations(configModule)

	for _, usage := range configModule.GetUsage() {
		if !usage.Initialized {
			assert.True(t, requiredEnvKeys[usage.Key],
				"config key %q is declared without a DefaultValue but is not a known required key. "+
					"Existing installations do not set it, so Config.Validate() would crash-loop them on upgrade. "+
					"Add a DefaultValue to the declaration (or, for a deliberate breaking change, extend requiredEnvKeys).",
				usage.Key)
		}
	}
}
