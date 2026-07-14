package ai

import (
	"encoding/json"
	"fmt"
	"mogenius-operator/src/utils"
	"strconv"

	"github.com/anthropics/anthropic-sdk-go"
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

// maxFindingsPerRun caps how many findings one run may submit — each finding
// becomes its own review task, so this bounds task spam from a single run.
const maxFindingsPerRun = 5

// aiSubmission is the wire shape of a submit_analysis call: a findings array,
// or (legacy / text fallback) a single top-level errorMessage+analysis pair.
type aiSubmission struct {
	Findings []AiResponse `json:"findings"`
	AiResponse
}

// parseFindings converts raw submission JSON into one AiResponse per finding.
// Accepts both the findings-array shape and the legacy single-analysis shape.
func parseFindings(data []byte) ([]*AiResponse, error) {
	var submission aiSubmission
	if err := json.Unmarshal(data, &submission); err != nil {
		return nil, fmt.Errorf("error unmarshaling AI response: %w", err)
	}

	findings := submission.Findings
	if len(findings) == 0 {
		findings = []AiResponse{submission.AiResponse}
	}
	if len(findings) > maxFindingsPerRun {
		findings = findings[:maxFindingsPerRun]
	}

	responses := make([]*AiResponse, 0, len(findings))
	for i := range findings {
		if findings[i].Analysis.ProblemDescription == "" {
			return nil, fmt.Errorf("finding %d: analysis.problemDescription must not be empty", i+1)
		}
		responses = append(responses, &findings[i])
	}
	return responses, nil
}

// parseAiResponse extracts and unmarshals the final analysis from a free-text
// model response. Shared by all providers; removedText is the prose the model
// wrapped around the JSON (logged for diagnosis).
func parseAiResponse(responseText string) (responses []*AiResponse, removedText string, err error) {
	responseText = cleanJSONResponse(responseText)
	responseBytes, removedText, err := extractJSONRobust(responseText)
	if err != nil {
		return nil, removedText, fmt.Errorf("error extracting JSON from AI response: %w", err)
	}
	responses, err = parseFindings(responseBytes)
	if err != nil {
		return nil, removedText, err
	}
	return responses, removedText, nil
}

// maxAnalysisRepairs bounds the in-conversation repair turns when the model's
// final analysis does not match the required schema. A repair turn feeds the
// parse error back into the running conversation (a few hundred tokens)
// instead of failing the task and re-running the whole exploration.
const maxAnalysisRepairs = 3

const submitAnalysisToolName = "submit_analysis"

// submitAnalysisInstruction is appended to the system prompt of providers that
// expose the submit_analysis tool.
const submitAnalysisInstruction = "\n\nWhen your investigation is complete, submit your final analysis by calling the " + submitAnalysisToolName + " tool with the complete result. Do not print the final JSON as text — always submit it through the tool. When you identified several distinct issues, report each as its own entry in findings (most impactful first) — every finding is reviewed and applied separately, so it must be self-contained."

var workloadRefSchema = map[string]any{
	"type":        "object",
	"description": "Exact identification of one Kubernetes resource.",
	"properties": map[string]any{
		"kind":         map[string]any{"type": "string", "description": "Resource kind (e.g., Pod, Deployment)."},
		"plural":       map[string]any{"type": "string", "description": "Lowercase plural of the kind (e.g., pods, deployments)."},
		"apiVersion":   map[string]any{"type": "string", "description": "API version (e.g., v1, apps/v1)."},
		"namespaced":   map[string]any{"type": "boolean", "description": "Whether the resource is namespaced."},
		"namespace":    map[string]any{"type": "string", "description": "Namespace (empty for cluster-scoped resources)."},
		"resourceName": map[string]any{"type": "string", "description": "Name of the resource."},
	},
	"required": []string{"kind", "apiVersion", "resourceName"},
}

// findingSchema is the schema of one finding: a headline plus the full
// analysis. Multiple distinct findings go into the findings array — each one
// becomes its own review card for the user.
var findingSchema = map[string]any{
	"type":     "object",
	"required": []string{"errorMessage", "analysis"},
	"properties": map[string]any{
		"errorMessage": map[string]any{"type": "string", "description": "One-line headline of the finding."},
		"analysis": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"problemDescription": map[string]any{"type": "string", "description": "2-4 crisp sentences: what is wrong, what the proposed change does, any risk."},
				"possibleCauses":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"proposedSolutions": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"solutionDescription": map[string]any{"type": "string"},
							"steps":               map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						},
						"required": []string{"solutionDescription", "steps"},
					},
				},
				"additionalInformation": map[string]any{"type": "string"},
				"needsFollowUp":         map[string]any{"type": "boolean"},
				"followUpResources": map[string]any{
					"type":        "array",
					"description": "Resources that deserve a follow-up analysis, each as a structured reference.",
					"items":       workloadRefSchema,
				},
				"currentResourceYaml": map[string]any{"type": "string", "description": "Current manifest of the target resource, exactly as retrieved from the cluster."},
				"targetResourceYaml":  map[string]any{"type": "string", "description": "Complete proposed manifest (required for UpdateResource and CreateResource; omit for DeleteResource). Base it on the manifest you retrieved from the cluster and change ONLY what the fix requires — never invent values, and never include server-managed fields (metadata.resourceVersion, uid, creationTimestamp, generation, managedFields, status)."},
				"targetResource":      workloadRefSchema,
				"proposedOperation":   map[string]any{"type": "string", "enum": []string{ProposedOperationUpdate, ProposedOperationDelete, ProposedOperationCreate, ProposedOperationOther}},
			},
			"required": []string{"problemDescription", "possibleCauses", "proposedSolutions"},
		},
	},
}

// submitAnalysisAnthropicTool carries the response schema, so the final
// analysis arrives as validated tool input instead of JSON scraped out of
// free text. Each distinct finding becomes its own review task.
var submitAnalysisAnthropicTool = anthropicTool(
	submitAnalysisToolName,
	"Submit your final analysis. Call this exactly once as your last action, after finishing the investigation. Report each distinct issue as its own entry in findings (most impactful first, at most "+strconv.Itoa(maxFindingsPerRun)+") — every finding is reviewed separately.",
	map[string]any{
		"findings": map[string]any{
			"type":        "array",
			"description": "One entry per distinct issue, ordered by impact.",
			"minItems":    1,
			"maxItems":    maxFindingsPerRun,
			"items":       findingSchema,
		},
	},
	[]string{"findings"},
)

// parseSubmittedAnalysis converts submit_analysis tool input into one
// AiResponse per finding. Kept separate from parseAiResponse: tool input is
// guaranteed JSON, so no text extraction is involved.
func parseSubmittedAnalysis(input json.RawMessage) ([]*AiResponse, error) {
	responses, err := parseFindings(input)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling submitted analysis: %w", err)
	}
	return responses, nil
}

// analysisRejectionResult builds the is_error tool result that feeds a schema
// violation back to the model for an in-conversation repair turn.
func analysisRejectionResult(toolUseID string, parseErr error) anthropic.ContentBlockParamUnion {
	return anthropic.NewToolResultBlock(
		toolUseID,
		fmt.Sprintf("Submission rejected: %s. Fix the arguments to match the %s tool schema exactly and call it again.", parseErr.Error(), submitAnalysisToolName),
		true,
	)
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
