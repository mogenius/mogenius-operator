package ai

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"mogenius-operator/src/store"
	"mogenius-operator/src/valkeyclient"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// Function variables to avoid cyclic imports - set these from cmd package
var (
	K8sUpdateUnstructuredResource       func(apiVersion, plural string, namespaced bool, yamlData string) (*unstructured.Unstructured, error)
	K8sDeleteUnstructuredResource       func(apiVersion, plural, namespace, resourceName string) error
	K8sCreateUnstructuredResource       func(apiVersion, plural string, namespaced bool, yamlData string) (*unstructured.Unstructured, error)
	K8sGetUnstructuredResourceFromStore func(apiVersion, kind, namespace, resourceName string) (*unstructured.Unstructured, error)
)

var kubernetesToolDefinitions = map[string]func(map[string]any, *ToolContext, valkeyclient.ValkeyClient, *slog.Logger) string{
	"get_kubernetes_resources":   getKubernetesResourcesTool,
	"list_kubernetes_resources":  listKubernetesResourcesTool,
	"update_kubernetes_resource": updateKubernetesResourceTool,
	"delete_kubernetes_resource": deleteKubernetesResourceTool,
	"create_kubernetes_resource": createKubernetesResourceTool,
}

const maxListResults = 50 // Limit to prevent token overflow

// ResourceSummary is a compact representation of a resource for listing
type ResourceSummary struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace,omitempty"`
	Kind         string `json:"kind"`
	Status       string `json:"status,omitempty"`
	CreationTime string `json:"creationTime,omitempty"`
}

func listKubernetesResourcesTool(args map[string]any, tc *ToolContext, valkeyClient valkeyclient.ValkeyClient, logger *slog.Logger) string {

	kind := args["kind"].(string)
	apiVersion := args["apiVersion"].(string)
	namespace, _ := args["namespace"].(string)

	if namespace != "" && !tc.IsNamespaceAllowed(namespace) && tc.AllowedArgoCDApps == nil {
		return fmt.Sprintf("Error: access to namespace %q is not allowed", namespace)
	}

	logger.Info("Listing Kubernetes resources", "apiVersion", apiVersion, "kind", kind, "namespace", namespace)
	resources := store.GetResourceByKindAndNamespace(valkeyClient, apiVersion, kind, namespace, logger)

	if tc.hasRestrictions() {
		filtered := resources[:0]
		for _, res := range resources {
			meta := mergeAnnotationsAndLabels(res.GetAnnotations(), res.GetLabels())
			if tc.IsResourceAllowed(res.GetNamespace(), meta) {
				filtered = append(filtered, res)
			}
		}
		resources = filtered
	}

	totalCount := len(resources)
	if totalCount == 0 {
		return fmt.Sprintf("No %s resources found", kind)
	}

	// Limit results to prevent token overflow
	truncated := false
	if len(resources) > maxListResults {
		resources = resources[:maxListResults]
		truncated = true
	}

	// Create compact summaries instead of full YAML
	summaries := make([]ResourceSummary, len(resources))
	for i, res := range resources {
		summary := ResourceSummary{
			Name:         res.GetName(),
			Namespace:    res.GetNamespace(),
			Kind:         res.GetKind(),
			CreationTime: res.GetCreationTimestamp().String(),
			Status:       "Unknown",
		}

		// Try to extract status (common field)
		if status, found, _ := unstructured.NestedString(res.Object, "status", "phase"); found {
			summary.Status = status
		} else if conditions, found, _ := unstructured.NestedSlice(res.Object, "status", "conditions"); found && len(conditions) > 0 {
			if cond, ok := conditions[0].(map[string]interface{}); ok {
				if condType, ok := cond["type"].(string); ok {
					if condStatus, ok := cond["status"].(string); ok {
						summary.Status = fmt.Sprintf("%s=%s", condType, condStatus)
					}
				}
			}
		}
		summaries[i] = summary
	}

	result := map[string]interface{}{
		"kind":      kind,
		"count":     totalCount,
		"resources": summaries,
	}

	if truncated {
		result["truncated"] = true
		result["message"] = fmt.Sprintf("Showing %d of %d resources. Use get_kubernetes_resources with a specific name for full details.", maxListResults, totalCount)
	}

	resourceBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling resources: %v", err)
	}
	logger.Info("Tool result", "resultCount", len(summaries), "totalCount", totalCount, "truncated", truncated)
	return string(resourceBytes)
}

func getKubernetesResourcesTool(args map[string]any, tc *ToolContext, valkeyClient valkeyclient.ValkeyClient, logger *slog.Logger) string {

	kind := args["kind"].(string)
	apiVersion := args["apiVersion"].(string)
	name, _ := args["name"].(string)
	namespace, _ := args["namespace"].(string)

	// Early reject if namespace is definitely not allowed (no ArgoCD fallback needed)
	if !tc.IsNamespaceAllowed(namespace) && tc.AllowedArgoCDApps == nil {
		return fmt.Sprintf("Error: access to namespace %q is not allowed", namespace)
	}

	logger.Info("Retrieving Kubernetes resources", "apiVersion", apiVersion, "kind", kind, "namespace", namespace, "name", name)
	resources, err := store.GetResource(valkeyClient, apiVersion, kind, namespace, name, logger)

	if err != nil {
		logger.Error("Error retrieving resources", "error", err)
		return fmt.Sprintf("Error retrieving resources: %v", err)
	}
	if resources == nil {
		logger.Warn("No resources found", "apiVersion", apiVersion, "kind", kind, "namespace", namespace, "name", name)
		return "No resources found matching the criteria"
	}

	// Check resource-level ownership (namespace + helm release + ArgoCD app)
	meta := mergeAnnotationsAndLabels(resources.GetAnnotations(), resources.GetLabels())
	if !tc.IsResourceAllowed(resources.GetNamespace(), meta) {
		return fmt.Sprintf("Error: access to resource %q in namespace %q is not allowed", resources.GetName(), namespace)
	}

	resourceBytes, err := json.MarshalIndent(resources, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling resources: %v", err)
	}
	logger.Info("Tool result", "resultLength", len(resourceBytes))
	return string(resourceBytes)
}

func updateKubernetesResourceTool(args map[string]any, tc *ToolContext, valkeyClient valkeyclient.ValkeyClient, logger *slog.Logger) string {
	if !tc.IsEditor() && !tc.IsAdmin() {
		return "Error: only users with editor or admin roles can update resources"
	}

	yamlData, ok := args["yamlData"].(string)
	if !ok {
		return "Error: yamlData is required"
	}
	apiVersion, _ := args["apiVersion"].(string)
	plural, _ := args["plural"].(string)
	namespaced, _ := args["namespaced"].(bool)

	// Parse YAML to get resource metadata
	var updatedObj *unstructured.Unstructured
	err := yaml.Unmarshal([]byte(yamlData), &updatedObj)
	if err != nil {
		logger.Error("Failed to unmarshal YAML data", "error", err)
		return fmt.Sprintf("Error: failed to unmarshal YAML data: %v", err)
	}

	meta := mergeAnnotationsAndLabels(updatedObj.GetAnnotations(), updatedObj.GetLabels())
	if !tc.IsResourceAllowed(updatedObj.GetNamespace(), meta) {
		return fmt.Sprintf("Error: access to resource %q in namespace %q is not allowed", updatedObj.GetName(), updatedObj.GetNamespace())
	}

	// Get old object for comparison
	oldObj, _ := K8sGetUnstructuredResourceFromStore(apiVersion, updatedObj.GetKind(), updatedObj.GetNamespace(), updatedObj.GetName())

	logger.Info("Updating Kubernetes resource", "apiVersion", apiVersion, "kind", updatedObj.GetKind(), "namespace", updatedObj.GetNamespace(), "name", updatedObj.GetName())

	// Perform the update
	updatedRes, err := K8sUpdateUnstructuredResource(apiVersion, plural, namespaced, yamlData)
	if err != nil {
		logger.Error("Failed to update resource", "error", err)
		return fmt.Sprintf("Error updating resource: %v", err)
	}

	// Log the change
	if oldObj != nil {
		logger.Info("Resource updated", "old", oldObj.GetResourceVersion(), "new", updatedRes.GetResourceVersion())
	}

	resourceBytes, err := json.MarshalIndent(updatedRes, "", "  ")
	if err != nil {
		return fmt.Sprintf("Resource updated successfully but error marshaling result: %v", err)
	}
	return fmt.Sprintf("Resource updated successfully:\n%s", string(resourceBytes))
}

func deleteKubernetesResourceTool(args map[string]any, tc *ToolContext, valkeyClient valkeyclient.ValkeyClient, logger *slog.Logger) string {
	if !tc.IsEditor() && !tc.IsAdmin() {
		return "Error: only users with editor or admin roles can delete resources"
	}

	apiVersion, _ := args["apiVersion"].(string)
	plural, _ := args["plural"].(string)
	namespace, _ := args["namespace"].(string)
	name, _ := args["name"].(string)

	if name == "" {
		return "Error: name is required for delete operation"
	}

	if !tc.IsNamespaceAllowed(namespace) && tc.AllowedArgoCDApps == nil {
		return fmt.Sprintf("Error: access to namespace %q is not allowed", namespace)
	}

	logger.Info("Deleting Kubernetes resource", "apiVersion", apiVersion, "plural", plural, "namespace", namespace, "name", name)

	err := K8sDeleteUnstructuredResource(apiVersion, plural, namespace, name)
	if err != nil {
		logger.Error("Failed to delete resource", "error", err)
		return fmt.Sprintf("Error deleting resource: %v", err)
	}

	return fmt.Sprintf("Resource '%s' deleted successfully", name)
}

func createKubernetesResourceTool(args map[string]any, tc *ToolContext, valkeyClient valkeyclient.ValkeyClient, logger *slog.Logger) string {
	if !tc.IsEditor() && !tc.IsAdmin() {
		return "Error: only users with editor or admin roles can create resources"
	}

	yamlData, ok := args["yamlData"].(string)
	if !ok {
		return "Error: yamlData is required"
	}
	apiVersion, _ := args["apiVersion"].(string)
	plural, _ := args["plural"].(string)
	namespaced, _ := args["namespaced"].(bool)

	// Parse YAML to get resource metadata for logging
	var obj *unstructured.Unstructured
	err := yaml.Unmarshal([]byte(yamlData), &obj)
	if err != nil {
		logger.Error("Failed to unmarshal YAML data", "error", err)
		return fmt.Sprintf("Error: failed to unmarshal YAML data: %v", err)
	}

	if !tc.IsNamespaceAllowed(obj.GetNamespace()) && tc.AllowedArgoCDApps == nil {
		return fmt.Sprintf("Error: access to namespace %q is not allowed", obj.GetNamespace())
	}

	logger.Info("Creating Kubernetes resource", "apiVersion", apiVersion, "kind", obj.GetKind(), "namespace", obj.GetNamespace(), "name", obj.GetName())

	// Perform the create
	createdRes, err := K8sCreateUnstructuredResource(apiVersion, plural, namespaced, yamlData)
	if err != nil {
		logger.Error("Failed to create resource", "error", err)
		return fmt.Sprintf("Error creating resource: %v", err)
	}

	resourceBytes, err := json.MarshalIndent(createdRes, "", "  ")
	if err != nil {
		return fmt.Sprintf("Resource created successfully but error marshaling result: %v", err)
	}
	return fmt.Sprintf("Resource created successfully:\n%s", string(resourceBytes))
}
