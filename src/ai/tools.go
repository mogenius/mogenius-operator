package ai

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"mogenius-operator/src/store"
	"mogenius-operator/src/valkeyclient"
)

var toolDefinitions = map[string]func(map[string]any, valkeyclient.ValkeyClient, *slog.Logger) string{
	"get_kubernetes_resources":  getKubernetesResourcesTool,
	"list_kubernetes_resources": listKubernetesResourcesTool,
}

func listKubernetesResourcesTool(args map[string]any, valkeyClient valkeyclient.ValkeyClient, logger *slog.Logger) string {

	kind := args["kind"].(string)
	apiVersion := args["apiVersion"].(string)
	namespace, _ := args["namespace"].(string)

	logger.Info("Listing Kubernetes resources", "apiVersion", apiVersion, "kind", kind, "namespace", namespace)
	resources := store.GetResourceByKindAndNamespace(valkeyClient, apiVersion, kind, namespace, logger)

	resourceBytes, err := json.MarshalIndent(resources, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling resources: %v", err)
	}
	return string(resourceBytes)
}

func getKubernetesResourcesTool(args map[string]any, valkeyClient valkeyclient.ValkeyClient, logger *slog.Logger) string {

	kind := args["kind"].(string)
	apiVersion := args["apiVersion"].(string)
	name, _ := args["name"].(string)
	namespace, _ := args["namespace"].(string)

	logger.Info("Retrieving Kubernetes resources", "apiVersion", apiVersion, "kind", kind, "namespace", namespace, "name", name)
	resources, err := store.GetResource(valkeyClient, apiVersion, kind, namespace, name, logger)

	if err != nil {
		return fmt.Sprintf("Error retrieving resources: %v", err)
	}
	resourceBytes, err := json.MarshalIndent(resources, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling resources: %v", err)
	}
	return string(resourceBytes)
}
