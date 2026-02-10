package ai

import (
	"log/slog"
	"mogenius-operator/src/valkeyclient"
)

type toolHandler = func(map[string]any, valkeyclient.ValkeyClient, *slog.Logger) string

// toolDefinitions is the combined registry of all AI tool handlers.
var toolDefinitions = mergeToolMaps(
	kubernetesToolDefinitions,
	helmToolDefinitions,
)

func mergeToolMaps(maps ...map[string]toolHandler) map[string]toolHandler {
	result := make(map[string]toolHandler)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}