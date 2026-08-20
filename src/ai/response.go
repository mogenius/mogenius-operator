package ai

import (
	"encoding/json"
	"mogenius-operator/src/utils"
)

// FollowUpResource is a WorkloadSingleRequest that tolerates the format drift
// LLMs produce for follow-up hints: instead of failing the whole analysis when
// the model emits a plain string ("ReplicaSet/foo (bar namespace)") or uses
// "name" instead of "resourceName", it keeps whatever identification the model
// gave. Follow-up resources are informational (never executed), so lossy
// parsing beats discarding an otherwise complete analysis.
type FollowUpResource struct {
	utils.WorkloadSingleRequest
}

func (f *FollowUpResource) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		f.ResourceName = s
		return nil
	}
	var obj struct {
		utils.WorkloadSingleRequest
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	if obj.ResourceName == "" {
		obj.ResourceName = obj.Name
	}
	f.WorkloadSingleRequest = obj.WorkloadSingleRequest
	return nil
}

// describeToolCall renders a compact human-readable activity line for the UI,
// e.g. "list_kubernetes_resources (kind: Pod, namespace: harbor)".
func describeToolCall(name string, args map[string]any) string {
	details := ""
	for _, key := range []string{"kind", "name", "release", "namespace"} {
		if v, ok := args[key].(string); ok && v != "" {
			if details != "" {
				details += ", "
			}
			details += key + ": " + v
		}
	}
	if details == "" {
		return name
	}
	return name + " (" + details + ")"
}
