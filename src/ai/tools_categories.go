package ai

import (
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/ollama/ollama/api"
	"github.com/openai/openai-go/v3"
)

// ---------------------------------------------------------------------------
// LLM-driven tool category activation
// ---------------------------------------------------------------------------

// ActiveToolCategories tracks which tool categories are active for a session.
// Categories are sticky: once activated they stay on for the entire session.
type ActiveToolCategories struct {
	KubernetesRead  bool
	KubernetesWrite bool
	HelmRead        bool
	HelmWrite       bool
}

// NewActiveToolCategories returns categories with only KubernetesRead enabled.
func NewActiveToolCategories() *ActiveToolCategories {
	return &ActiveToolCategories{KubernetesRead: true}
}

const activateToolCategoriesName = "activate_tool_categories"

const activateToolCategoriesDesc = `Activate additional tool categories for this chat session. By default only Kubernetes read tools (get/list resources) are available. Call this FIRST when you need additional capabilities.

Available categories (comma-separated):
- KubernetesWrite: create, update, delete Kubernetes resources
- HelmRead: search charts, list repos, get release details/status/history/workloads
- HelmWrite: add/remove/update repos, install/upgrade/uninstall charts, rollback releases

Example: "HelmRead,HelmWrite". Categories stay active once enabled.`

// ActivateFromToolCall processes an activate_tool_categories tool call and
// returns a human-readable result string.
func (atc *ActiveToolCategories) ActivateFromToolCall(args map[string]any) string {
	raw, _ := args["categories"].(string)
	var activated []string
	for _, cat := range strings.Split(raw, ",") {
		switch strings.TrimSpace(cat) {
		case "KubernetesWrite":
			if !atc.KubernetesWrite {
				atc.KubernetesWrite = true
				activated = append(activated, "KubernetesWrite")
			}
		case "HelmRead":
			if !atc.HelmRead {
				atc.HelmRead = true
				activated = append(activated, "HelmRead")
			}
		case "HelmWrite":
			if !atc.HelmWrite {
				atc.HelmWrite = true
				atc.HelmRead = true // HelmWrite implies HelmRead
				activated = append(activated, "HelmWrite")
			}
		}
	}
	if len(activated) == 0 {
		return "No new categories activated. Already active or unknown category names."
	}
	return fmt.Sprintf("Activated: %s. These tools are now available.", strings.Join(activated, ", "))
}

// --- Meta-tool definitions per provider ---

var activateToolCategoriesOpenAi = openai.ChatCompletionToolUnionParam{
	OfFunction: &openai.ChatCompletionFunctionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        activateToolCategoriesName,
			Description: openai.String(activateToolCategoriesDesc),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"categories": map[string]any{
						"type":        "string",
						"description": "Comma-separated list of categories to activate: KubernetesWrite, HelmRead, HelmWrite",
					},
				},
				"required": []string{"categories"},
			},
		},
	},
}

var activateToolCategoriesAnthropic = anthropic.ToolParam{
	Name:        activateToolCategoriesName,
	Description: anthropic.String(activateToolCategoriesDesc),
	InputSchema: anthropic.ToolInputSchemaParam{
		Type: "object",
		Properties: map[string]any{
			"categories": map[string]any{
				"type":        "string",
				"description": "Comma-separated list of categories to activate: KubernetesWrite, HelmRead, HelmWrite",
			},
		},
		Required: []string{"categories"},
	},
}

var activateToolCategoriesOllama = ollamaTool(
	activateToolCategoriesName,
	activateToolCategoriesDesc,
	map[string]api.ToolProperty{
		"categories": {
			Type:        []string{"string"},
			Description: "Comma-separated list of categories to activate: KubernetesWrite, HelmRead, HelmWrite",
		},
	},
	[]string{"categories"},
)

// ---------------------------------------------------------------------------
// Category mapping & filtering
// ---------------------------------------------------------------------------

// toolCategory represents which category a tool belongs to.
type toolCategory int

const (
	categoryKubernetesRead toolCategory = iota
	categoryKubernetesWrite
	categoryHelmRead
	categoryHelmWrite
)

// toolCategoryMap maps tool names to their category.
var toolCategoryMap = map[string]toolCategory{
	// Kubernetes Read
	"get_kubernetes_resources":  categoryKubernetesRead,
	"list_kubernetes_resources": categoryKubernetesRead,
	"check_kubernetes_resource": categoryKubernetesRead,
	"get_pod_logs":              categoryKubernetesRead,
	"get_pod_events":            categoryKubernetesRead,
	// Kubernetes Write
	"update_kubernetes_resource": categoryKubernetesWrite,
	"delete_kubernetes_resource": categoryKubernetesWrite,
	"create_kubernetes_resource": categoryKubernetesWrite,
	// Helm Read
	"helm_chart_search":          categoryHelmRead,
	"helm_chart_show":            categoryHelmRead,
	"helm_chart_versions":        categoryHelmRead,
	"helm_repo_list":             categoryHelmRead,
	"helm_release_list":          categoryHelmRead,
	"helm_release_get":           categoryHelmRead,
	"helm_release_history":       categoryHelmRead,
	"helm_release_status":        categoryHelmRead,
	"helm_release_get_workloads": categoryHelmRead,
	// Helm Write
	"helm_repo_add":          categoryHelmWrite,
	"helm_repo_patch":        categoryHelmWrite,
	"helm_repo_update":       categoryHelmWrite,
	"helm_repo_remove":       categoryHelmWrite,
	"helm_chart_install":     categoryHelmWrite,
	"helm_oci_install":       categoryHelmWrite,
	"helm_release_upgrade":   categoryHelmWrite,
	"helm_release_uninstall": categoryHelmWrite,
	"helm_release_rollback":  categoryHelmWrite,
	"helm_release_link":      categoryHelmWrite,
}

// shouldIncludeTool returns true when the tool should be sent to the model
// given the current active categories. Unknown tools (e.g. MCP) are always
// included.
func shouldIncludeTool(name string, cat *ActiveToolCategories) bool {
	tc, known := toolCategoryMap[name]
	if !known {
		return true // MCP or unknown → always include
	}
	switch tc {
	case categoryKubernetesRead:
		return cat.KubernetesRead
	case categoryKubernetesWrite:
		return cat.KubernetesWrite
	case categoryHelmRead:
		return cat.HelmRead
	case categoryHelmWrite:
		return cat.HelmWrite
	}
	return true
}

func filterOpenAiToolsByCategory(tools []openai.ChatCompletionToolUnionParam, cat *ActiveToolCategories) []openai.ChatCompletionToolUnionParam {
	filtered := make([]openai.ChatCompletionToolUnionParam, 0, len(tools))
	for _, t := range tools {
		name := ""
		if t.OfFunction != nil {
			name = t.OfFunction.Function.Name
		}
		if shouldIncludeTool(name, cat) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func filterAnthropicToolsByCategory(tools []anthropic.ToolParam, cat *ActiveToolCategories) []anthropic.ToolParam {
	filtered := make([]anthropic.ToolParam, 0, len(tools))
	for _, t := range tools {
		if shouldIncludeTool(t.Name, cat) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func filterOllamaToolsByCategory(tools []api.Tool, cat *ActiveToolCategories) []api.Tool {
	filtered := make([]api.Tool, 0, len(tools))
	for _, t := range tools {
		if shouldIncludeTool(t.Function.Name, cat) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
