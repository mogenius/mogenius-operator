package ai

import (
	"encoding/json"
	"mogenius-operator/src/utils"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFollowUpResourceLenientUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected FollowUpResource
	}{
		{
			name:  "plain string keeps the identification as resource name",
			input: `"ReplicaSet/homepage-5f8d9f9c5d (homepage namespace, 0 replicas)"`,
			expected: func() FollowUpResource {
				var f FollowUpResource
				f.ResourceName = "ReplicaSet/homepage-5f8d9f9c5d (homepage namespace, 0 replicas)"
				return f
			}(),
		},
		{
			name:  "object with name alias maps to resourceName",
			input: `{"kind":"Job","apiVersion":"batch/v1","namespace":"harbor","name":"harbor-jobservice-init"}`,
			expected: func() FollowUpResource {
				var f FollowUpResource
				f.Kind = "Job"
				f.ApiVersion = "batch/v1"
				f.Namespace = "harbor"
				f.ResourceName = "harbor-jobservice-init"
				return f
			}(),
		},
		{
			name:  "canonical object parses unchanged",
			input: `{"kind":"Deployment","plural":"deployments","apiVersion":"apps/v1","namespaced":true,"namespace":"mogenius","resourceName":"mogenius-studio"}`,
			expected: func() FollowUpResource {
				var f FollowUpResource
				f.Kind = "Deployment"
				f.Plural = "deployments"
				f.ApiVersion = "apps/v1"
				f.Namespaced = true
				f.Namespace = "mogenius"
				f.ResourceName = "mogenius-studio"
				return f
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got FollowUpResource
			err := json.Unmarshal([]byte(tt.input), &got)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestParseSubmittedAnalysisAcceptsStringFollowUps replays the shape that made
// run cleaner-run-1784009096 fail four times in a row (followUpResources as
// free-text strings): the analysis must survive parsing instead of being
// discarded.
func TestParseSubmittedAnalysisAcceptsStringFollowUps(t *testing.T) {
	input := json.RawMessage(`{
		"errorMessage": "Cluster contains 100+ obsolete ReplicaSets",
		"analysis": {
			"problemDescription": "Old ReplicaSets accumulate.",
			"possibleCauses": ["default revisionHistoryLimit"],
			"proposedSolutions": [{"solutionDescription": "clean up", "steps": ["1. delete"]}],
			"needsFollowUp": true,
			"followUpResources": [
				"ReplicaSet/homepage-5f8d9f9c5d (homepage namespace, 0 replicas)",
				{"kind": "Job", "apiVersion": "batch/v1", "namespace": "harbor", "name": "harbor-jobservice-init"}
			],
			"proposedOperation": "DeleteResource",
			"targetResource": {"kind": "Job", "plural": "jobs", "apiVersion": "batch/v1", "namespaced": true, "namespace": "harbor", "resourceName": "harbor-jobservice-init"}
		}
	}`)

	responses, err := parseSubmittedAnalysis(input)
	assert.NoError(t, err)
	assert.Len(t, responses, 1)
	response := responses[0]
	assert.Equal(t, "Cluster contains 100+ obsolete ReplicaSets", response.ErrorMessage)
	assert.Len(t, response.Analysis.FollowUpResources, 2)
	assert.Equal(t, "ReplicaSet/homepage-5f8d9f9c5d (homepage namespace, 0 replicas)", response.Analysis.FollowUpResources[0].ResourceName)
	assert.Equal(t, "harbor-jobservice-init", response.Analysis.FollowUpResources[1].ResourceName)
	assert.Equal(t, "harbor-jobservice-init", response.Analysis.TargetResource.ResourceName)
}

// TestParseSubmittedAnalysisMultipleFindings: each entry in findings becomes
// its own AiResponse, order preserved.
func TestParseSubmittedAnalysisMultipleFindings(t *testing.T) {
	input := json.RawMessage(`{
		"findings": [
			{"errorMessage": "first", "analysis": {"problemDescription": "a", "proposedOperation": "UpdateResource"}},
			{"errorMessage": "second", "analysis": {"problemDescription": "b", "proposedOperation": "DeleteResource"}},
			{"errorMessage": "third", "analysis": {"problemDescription": "c"}}
		]
	}`)

	responses, err := parseSubmittedAnalysis(input)
	assert.NoError(t, err)
	assert.Len(t, responses, 3)
	assert.Equal(t, "first", responses[0].ErrorMessage)
	assert.Equal(t, "UpdateResource", responses[0].Analysis.ProposedOperation)
	assert.Equal(t, "second", responses[1].ErrorMessage)
	assert.Equal(t, "c", responses[2].Analysis.ProblemDescription)
}

func TestParseSubmittedAnalysisRejectsFindingWithoutProblemDescription(t *testing.T) {
	input := json.RawMessage(`{
		"findings": [
			{"errorMessage": "ok", "analysis": {"problemDescription": "a"}},
			{"errorMessage": "broken", "analysis": {}}
		]
	}`)
	_, err := parseSubmittedAnalysis(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "finding 2")
}

// TestParseFindingsNoTruncation: the number of findings per submission is not
// capped — the run's tool-call budget is the only bound.
func TestParseFindingsNoTruncation(t *testing.T) {
	const count = 25
	findings := make([]string, 0, count)
	for range count {
		findings = append(findings, `{"errorMessage": "x", "analysis": {"problemDescription": "y"}}`)
	}
	input := []byte(`{"findings": [` + strings.Join(findings, ",") + `]}`)
	responses, err := parseFindings(input)
	assert.NoError(t, err)
	assert.Len(t, responses, count)
}

func TestParseSubmittedAnalysisRejectsEmptyProblemDescription(t *testing.T) {
	_, err := parseSubmittedAnalysis(json.RawMessage(`{"errorMessage": "x", "analysis": {}}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "problemDescription")
}

func TestParseSubmittedAnalysisRejectsInvalidJSON(t *testing.T) {
	_, err := parseSubmittedAnalysis(json.RawMessage(`{"analysis": "not an object"}`))
	assert.Error(t, err)
}

func TestParseAiResponseExtractsFromProse(t *testing.T) {
	text := "Based on my analysis:\n```json\n{\"errorMessage\": \"x\", \"analysis\": {\"problemDescription\": \"y\", \"followUpResources\": [\"a string hint\"]}}\n```"
	responses, _, err := parseAiResponse(text)
	assert.NoError(t, err)
	assert.Len(t, responses, 1)
	assert.Equal(t, "y", responses[0].Analysis.ProblemDescription)
	assert.Equal(t, "a string hint", responses[0].Analysis.FollowUpResources[0].ResourceName)
}

func TestDescribeToolCall(t *testing.T) {
	assert.Equal(t,
		"list_kubernetes_resources (kind: Pod, namespace: harbor)",
		describeToolCall("list_kubernetes_resources", map[string]any{"kind": "Pod", "namespace": "harbor", "apiVersion": "v1"}))
	assert.Equal(t, "helm_list_releases", describeToolCall("helm_list_releases", map[string]any{}))
}

func TestStripServerManagedFields(t *testing.T) {
	yamlIn := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: harbor-core
  namespace: harbor
  resourceVersion: "123456789"
  uid: a1b2c3d4-e5f6-7890-abcd-ef1234567890
  creationTimestamp: "2024-03-15T10:23:45Z"
  generation: 16
  annotations:
    deployment.kubernetes.io/revision: "16"
    meta.helm.sh/release-name: harbor
spec:
  replicas: 1
status:
  readyReplicas: 1
`
	obj, err := parseTargetYaml(yamlIn)
	assert.NoError(t, err)
	stripServerManagedFields(obj)

	assert.Equal(t, "harbor-core", obj.GetName())
	assert.Equal(t, "", obj.GetResourceVersion())
	assert.Empty(t, obj.GetUID())
	creationTimestamp := obj.GetCreationTimestamp()
	assert.True(t, creationTimestamp.IsZero())
	assert.Zero(t, obj.GetGeneration())
	_, hasStatus := obj.Object["status"]
	assert.False(t, hasStatus)
	assert.Equal(t, map[string]string{"meta.helm.sh/release-name": "harbor"}, obj.GetAnnotations())
	replicas, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	assert.Equal(t, int64(1), replicas)
}

func TestParseFindingsEmptySubmissionMeansNothingToReport(t *testing.T) {
	responses, err := parseFindings([]byte(`{"findings": []}`))
	assert.NoError(t, err)
	assert.Empty(t, responses)

	responses, err = parseFindings([]byte(`{}`))
	assert.NoError(t, err)
	assert.Empty(t, responses)
}

func TestFindingRejectionReason(t *testing.T) {
	ai := &aiManager{}

	// No structured proposal at all → advice-only.
	adviceOnly := &AiResponse{ErrorMessage: "advice", Analysis: Analysis{ProblemDescription: "p"}}
	assert.Contains(t, ai.findingRejectionReason(adviceOnly), "proposedOperation")

	// "Other" is not applicable either.
	other := &AiResponse{Analysis: Analysis{ProposedOperation: ProposedOperationOther}}
	assert.Contains(t, ai.findingRejectionReason(other), "proposedOperation")

	// Delete without a target name.
	del := &AiResponse{Analysis: Analysis{ProposedOperation: ProposedOperationDelete}}
	assert.Contains(t, ai.findingRejectionReason(del), "resourceName")

	// Update without a manifest.
	upd := &AiResponse{Analysis: Analysis{ProposedOperation: ProposedOperationUpdate}}
	assert.Contains(t, ai.findingRejectionReason(upd), "targetResourceYaml")

	// Create with a manifest is actionable without a store lookup.
	create := &AiResponse{Analysis: Analysis{ProposedOperation: ProposedOperationCreate, TargetResourceYaml: "kind: ConfigMap"}}
	assert.Equal(t, "", ai.findingRejectionReason(create))

	assert.Equal(t, "empty finding", ai.findingRejectionReason(nil))
}

func TestAgeString(t *testing.T) {
	assert.Equal(t, "", ageString(time.Time{}))
	assert.Equal(t, "2d", ageString(time.Now().Add(-49*time.Hour)))
	assert.Equal(t, "3h", ageString(time.Now().Add(-190*time.Minute)))
	assert.Equal(t, "5m", ageString(time.Now().Add(-5*time.Minute)))
}

func TestIsResourceExcluded(t *testing.T) {
	var nilCtx *ToolContext
	assert.False(t, nilCtx.IsResourceExcluded("v1", "Pod", "default", "x"))

	tc := &ToolContext{ExcludeResources: map[string]bool{
		aiResourceKey("apps/v1", "ReplicaSet", "mogenius", "cert-manager-cainjector-5cd89979d6"): true,
	}}
	assert.True(t, tc.IsResourceExcluded("apps/v1", "ReplicaSet", "mogenius", "cert-manager-cainjector-5cd89979d6"))
	assert.False(t, tc.IsResourceExcluded("apps/v1", "ReplicaSet", "mogenius", "other"))
	assert.False(t, tc.IsResourceExcluded("v1", "Pod", "mogenius", "cert-manager-cainjector-5cd89979d6"))

	empty := &ToolContext{}
	assert.False(t, empty.IsResourceExcluded("v1", "Pod", "default", "x"))
}

func TestDeleteTargetsBulk(t *testing.T) {
	ai := &aiManager{}
	analysis := Analysis{
		ProposedOperation: ProposedOperationDelete,
		TargetResource: utils.WorkloadSingleRequest{
			ResourceDescriptor: utils.ResourceDescriptor{ApiVersion: "batch/v1", Kind: "Job", Plural: "jobs", Namespaced: true},
			Namespace:          "ci",
			ResourceName:       "job-1",
		},
		AdditionalTargets: []utils.WorkloadSingleRequest{
			// Missing descriptor fields default from the primary target.
			{Namespace: "ci", ResourceName: "job-2"},
			{Namespace: "ci", ResourceName: "job-3"},
			// Duplicate of the primary — must be deduped.
			{ResourceDescriptor: utils.ResourceDescriptor{ApiVersion: "batch/v1", Kind: "Job"}, Namespace: "ci", ResourceName: "job-1"},
			// No name — dropped.
			{Namespace: "ci"},
		},
	}
	targets := ai.deleteTargets(analysis)
	assert.Len(t, targets, 3)
	for _, tg := range targets {
		assert.Equal(t, "batch/v1", tg.ApiVersion)
		assert.Equal(t, "jobs", tg.Plural)
		assert.Equal(t, "Job", tg.Kind)
		assert.True(t, tg.Namespaced)
	}
	names := []string{targets[0].ResourceName, targets[1].ResourceName, targets[2].ResourceName}
	assert.ElementsMatch(t, []string{"job-1", "job-2", "job-3"}, names)
}
