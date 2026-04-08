package ai

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/openai/openai-go/v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// fieldsToStrip are metadata/status fields that waste tokens without adding
// value for LLM reasoning. managedFields alone is often 50-80% of a resource.
var fieldsToStrip = [][]string{
	{"metadata", "managedFields"},
	{"metadata", "annotations", "kubectl.kubernetes.io/last-applied-configuration"},
	{"metadata", "uid"},
	{"metadata", "resourceVersion"},
	{"metadata", "generation"},
	{"metadata", "selfLink"},
}

// stripVerboseFields removes known noisy fields in place.
func stripVerboseFields(obj map[string]interface{}) {
	for _, path := range fieldsToStrip {
		unstructured.RemoveNestedField(obj, path...)
	}
}

// compactResourceText converts a K8s Unstructured object into a compact,
// describe-like text representation optimized for LLM consumption.
// Produces ~90% fewer tokens than json.MarshalIndent of the full object.
func compactResourceText(obj *unstructured.Unstructured) string {
	data := obj.DeepCopy().Object
	stripVerboseFields(data)

	var b strings.Builder

	// Header line
	kind, _, _ := unstructured.NestedString(data, "kind")
	name := obj.GetName()
	ns := obj.GetNamespace()
	apiVersion, _, _ := unstructured.NestedString(data, "apiVersion")
	fmt.Fprintf(&b, "%s/%s", kind, name)
	if ns != "" {
		fmt.Fprintf(&b, " ns=%s", ns)
	}
	if apiVersion != "" {
		fmt.Fprintf(&b, " apiVersion=%s", apiVersion)
	}
	if ts := obj.GetCreationTimestamp(); !ts.IsZero() {
		fmt.Fprintf(&b, " created=%s", ts.Format("2006-01-02T15:04:05Z"))
	}
	b.WriteString("\n")

	// Labels
	if labels := obj.GetLabels(); len(labels) > 0 {
		fmt.Fprintf(&b, "labels: %s\n", flatKV(labels))
	}

	// Annotations (already stripped of last-applied-configuration)
	if meta, ok := data["metadata"].(map[string]interface{}); ok {
		if anns, ok := meta["annotations"].(map[string]interface{}); ok && len(anns) > 0 {
			fmt.Fprintf(&b, "annotations: %s\n", flatKVAny(anns))
		}
	}

	// Owner references
	if owners, found, _ := unstructured.NestedSlice(data, "metadata", "ownerReferences"); found && len(owners) > 0 {
		var parts []string
		for _, o := range owners {
			if om, ok := o.(map[string]interface{}); ok {
				parts = append(parts, fmt.Sprintf("%s/%s", om["kind"], om["name"]))
			}
		}
		fmt.Fprintf(&b, "owners: %s\n", strings.Join(parts, ", "))
	}

	// Spec
	if spec, ok := data["spec"].(map[string]interface{}); ok {
		b.WriteString("spec:\n")
		writeCompactMap(&b, spec, "  ")
	}

	// Status
	if status, ok := data["status"].(map[string]interface{}); ok {
		b.WriteString("status:\n")
		writeCompactMap(&b, status, "  ")
	}

	// Data (ConfigMaps/Secrets)
	if d, ok := data["data"].(map[string]interface{}); ok {
		b.WriteString("data:\n")
		writeCompactMap(&b, d, "  ")
	}

	// StringData (Secrets)
	if d, ok := data["stringData"].(map[string]interface{}); ok {
		b.WriteString("stringData:\n")
		writeCompactMap(&b, d, "  ")
	}

	return b.String()
}

// writeCompactMap recursively writes a map in compact describe-like format.
func writeCompactMap(b *strings.Builder, m map[string]interface{}, indent string) {
	keys := sortedMapKeys(m)
	for _, k := range keys {
		v := m[k]
		switch val := v.(type) {
		case nil:
			// skip
		case map[string]interface{}:
			if len(val) == 0 {
				continue
			}
			if isSimpleMap(val) && len(val) <= 4 {
				fmt.Fprintf(b, "%s%s: %s\n", indent, k, flatKVAny(val))
			} else {
				fmt.Fprintf(b, "%s%s:\n", indent, k)
				writeCompactMap(b, val, indent+"  ")
			}
		case []interface{}:
			if len(val) == 0 {
				continue
			}
			writeCompactSlice(b, k, val, indent)
		case string:
			s := val
			if len(s) > 200 {
				s = s[:200] + fmt.Sprintf("...(%d chars)", len(val))
			}
			if strings.ContainsAny(s, "\n\r") {
				lines := strings.Split(s, "\n")
				fmt.Fprintf(b, "%s%s: %s ...(%d lines)\n", indent, k, lines[0], len(lines))
			} else {
				fmt.Fprintf(b, "%s%s: %s\n", indent, k, s)
			}
		default:
			fmt.Fprintf(b, "%s%s: %v\n", indent, k, v)
		}
	}
}

// writeCompactSlice writes a slice in compact format.
func writeCompactSlice(b *strings.Builder, key string, items []interface{}, indent string) {
	if isSimpleSlice(items) {
		parts := make([]string, len(items))
		for i, item := range items {
			parts[i] = fmt.Sprintf("%v", item)
		}
		fmt.Fprintf(b, "%s%s: [%s]\n", indent, key, strings.Join(parts, ", "))
		return
	}

	fmt.Fprintf(b, "%s%s:\n", indent, key)
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			if isSimpleMap(m) && len(m) <= 5 {
				fmt.Fprintf(b, "%s  - %s\n", indent, flatKVAny(m))
			} else {
				b.WriteString(indent + "  -\n")
				writeCompactMap(b, m, indent+"    ")
			}
		} else {
			fmt.Fprintf(b, "%s  - %v\n", indent, item)
		}
	}
}

// isSimpleMap returns true if all values are scalar (no nested maps/slices).
func isSimpleMap(m map[string]interface{}) bool {
	for _, v := range m {
		switch v.(type) {
		case map[string]interface{}, []interface{}:
			return false
		}
	}
	return true
}

// isSimpleSlice returns true if all items are scalars.
func isSimpleSlice(items []interface{}) bool {
	for _, item := range items {
		switch item.(type) {
		case map[string]interface{}, []interface{}:
			return false
		}
	}
	return true
}

// flatKV formats a string map as "k1=v1 k2=v2".
func flatKV(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + m[k]
	}
	return strings.Join(parts, " ")
}

// flatKVAny formats an interface{} map as "k1=v1 k2=v2".
func flatKVAny(m map[string]interface{}) string {
	keys := sortedMapKeys(m)
	parts := make([]string, len(keys))
	for i, k := range keys {
		v := m[k]
		if s, ok := v.(string); ok && len(s) > 80 {
			v = s[:80] + "..."
		}
		parts[i] = fmt.Sprintf("%s=%v", k, v)
	}
	return strings.Join(parts, " ")
}

// sortedMapKeys returns the keys of a map sorted alphabetically.
func sortedMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// truncateResult caps a string at maxChars, appending a note if truncated.
func truncateResult(s string, maxChars int) string {
	if maxChars <= 0 || len(s) <= maxChars {
		return s
	}
	return s[:maxChars] + fmt.Sprintf("\n...truncated (%d total chars, showing first %d)", len(s), maxChars)
}

// compactAnthropicToolResults replaces all tool_result contents in messages
// with a short marker. Call this AFTER the model has processed the results
// (i.e. after receiving the API response) and BEFORE the next API call.
// This prevents old tool results from accumulating tokens across iterations.
func compactAnthropicToolResults(messages []anthropic.MessageParam) {
	for i := range messages {
		if messages[i].Role != anthropic.MessageParamRoleUser {
			continue
		}
		for j, block := range messages[i].Content {
			if block.OfToolResult != nil {
				messages[i].Content[j] = anthropic.NewToolResultBlock(
					block.OfToolResult.ToolUseID,
					"[processed]",
					false,
				)
			}
		}
	}
}

// compactOpenAiToolMessages replaces all tool message contents in the message
// list with a short marker. Same purpose as compactAnthropicToolResults.
func compactOpenAiToolMessages(messages []openai.ChatCompletionMessageParamUnion) {
	for i := range messages {
		if messages[i].OfTool != nil {
			messages[i] = openai.ToolMessage("[processed]", messages[i].OfTool.ToolCallID)
		}
	}
}
