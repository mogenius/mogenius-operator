package ai

import (
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateAgentSpec(t *testing.T) {
	validSpec := v1alpha1.AgentSpec{
		Enabled: true,
		Scope:   v1alpha1.AgentScope{Namespaces: []string{"default"}},
		Triggers: v1alpha1.AgentTriggers{
			Cron: "*/5 * * * *",
			Events: []v1alpha1.AgentEventFilter{
				{Kind: "Pod", Contains: map[string]string{".status.phase": "Failed"}},
			},
		},
	}

	tests := []struct {
		name    string
		mutate  func(spec *v1alpha1.AgentSpec)
		wantErr string
	}{
		{name: "valid spec", mutate: func(spec *v1alpha1.AgentSpec) {}},
		{
			name:    "empty scope",
			mutate:  func(spec *v1alpha1.AgentSpec) { spec.Scope = v1alpha1.AgentScope{} },
			wantErr: "scope",
		},
		{
			name:    "blank namespace entry",
			mutate:  func(spec *v1alpha1.AgentSpec) { spec.Scope.Namespaces = []string{" "} },
			wantErr: "empty namespace",
		},
		{
			name:    "invalid cron",
			mutate:  func(spec *v1alpha1.AgentSpec) { spec.Triggers.Cron = "not a cron" },
			wantErr: "cron",
		},
		{
			name: "event filter without kind",
			mutate: func(spec *v1alpha1.AgentSpec) {
				spec.Triggers.Events = []v1alpha1.AgentEventFilter{{Contains: map[string]string{"a": "b"}}}
			},
			wantErr: "missing a kind",
		},
		{
			name: "event filter without contains",
			mutate: func(spec *v1alpha1.AgentSpec) {
				spec.Triggers.Events = []v1alpha1.AgentEventFilter{{Kind: "Pod"}}
			},
			wantErr: "contains",
		},
		{
			name:   "workspace ref only is a valid scope",
			mutate: func(spec *v1alpha1.AgentSpec) { spec.Scope = v1alpha1.AgentScope{WorkspaceRef: "team-a"} },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := *validSpec.DeepCopy()
			tt.mutate(&spec)
			err := ValidateAgentSpec(spec)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestAgentFilterToAiFilter(t *testing.T) {
	agent := &v1alpha1.Agent{ObjectMeta: metav1.ObjectMeta{Name: "my-agent"}}
	forDuration := metav1.Duration{Duration: 5 * time.Minute}
	filter := v1alpha1.AgentEventFilter{
		Kind:     "Pod",
		Contains: map[string]string{".status.phase": "Failed"},
		Excludes: map[string]string{".status.phase": "Succeeded"},
		Prompt:   "analyze this",
		For:      &forDuration,
	}

	converted := agentFilterToAiFilter(agent, filter)

	assert.Equal(t, "my-agent/Pod", converted.Id, "id falls back to agent/kind")
	assert.Equal(t, converted.Id, converted.Name, "name falls back to id")
	assert.Equal(t, "Pod", converted.Kind)
	assert.Equal(t, filter.Contains, converted.Contains)
	assert.Equal(t, filter.Excludes, converted.Excludes)
	assert.Equal(t, "analyze this", converted.Prompt)
	assert.True(t, converted.IsActive)
	assert.NotNil(t, converted.For)
	assert.Equal(t, 5*time.Minute, *converted.For)
}

// Locks in the security-critical invariants of the agent ToolContext: the role
// is explicitly "viewer" (an empty role passes IsEditor/IsAdmin) and namespace
// restrictions are enforced.
func TestNewToolContextFromAgent(t *testing.T) {
	agent := &v1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{Name: "my-agent"},
		Spec: v1alpha1.AgentSpec{
			Scope: v1alpha1.AgentScope{WorkspaceRef: "team-a"},
		},
	}

	tc := newToolContextFromAgent(agent, []string{"prod", "staging"})

	assert.Equal(t, "viewer", tc.Role)
	assert.False(t, tc.IsEditor(), "agent context must never pass editor checks")
	assert.False(t, tc.IsAdmin(), "agent context must never pass admin checks")
	assert.True(t, tc.hasRestrictions())
	assert.True(t, tc.IsNamespaceAllowed("prod"))
	assert.True(t, tc.IsNamespaceAllowed("staging"))
	assert.False(t, tc.IsNamespaceAllowed("kube-system"))
	assert.NotNil(t, tc.User)
	assert.Equal(t, "agent:my-agent@system", tc.User.Email, "tool calls must be attributable")
	assert.Equal(t, "team-a", tc.Workspace)
}

func TestUpdateTaskStateWhitelist(t *testing.T) {
	ai := &aiManager{}

	// States owned by the pipeline / approval flow must be rejected before any
	// storage access (ai has no valkey client here — reaching it would panic).
	for _, state := range []AiTaskState{
		AI_TASK_STATE_PROPOSED,
		AI_TASK_STATE_EXECUTING,
		AI_TASK_STATE_EXECUTED,
		AI_TASK_STATE_EXECUTION_FAILED,
		AI_TASK_STATE_REJECTED,
		AI_TASK_STATE_IN_PROGRESS,
		AI_TASK_STATE_COMPLETED,
		AI_TASK_STATE_FAILED,
	} {
		err := ai.UpdateTaskState("some-task", state)
		assert.ErrorContains(t, err, "cannot be set directly", "state %q must not be settable via the generic handler", state)
	}
}

func TestFinalizeTaskOutcome(t *testing.T) {
	ai := &aiManager{}

	t.Run("no response stays completed", func(t *testing.T) {
		task := &AiTask{}
		ai.finalizeTaskOutcome(task)
		assert.Equal(t, AI_TASK_STATE_COMPLETED, task.State)
	})

	t.Run("analysis without operation stays completed", func(t *testing.T) {
		task := &AiTask{Response: &AiResponse{Analysis: Analysis{ProposedOperation: ProposedOperationOther}}}
		ai.finalizeTaskOutcome(task)
		assert.Equal(t, AI_TASK_STATE_COMPLETED, task.State)
	})

	t.Run("create with yaml becomes proposed", func(t *testing.T) {
		task := &AiTask{Response: &AiResponse{Analysis: Analysis{
			ProposedOperation:  ProposedOperationCreate,
			TargetResourceYaml: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: foo\n",
		}}}
		ai.finalizeTaskOutcome(task)
		assert.Equal(t, AI_TASK_STATE_PROPOSED, task.State)
	})

	t.Run("create clears model-provided current yaml", func(t *testing.T) {
		task := &AiTask{Response: &AiResponse{Analysis: Analysis{
			ProposedOperation:   ProposedOperationCreate,
			TargetResourceYaml:  "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: foo\n",
			CurrentResourceYaml: "hallucinated: yes\n",
		}}}
		ai.finalizeTaskOutcome(task)
		assert.Equal(t, AI_TASK_STATE_PROPOSED, task.State)
		assert.Empty(t, task.Response.Analysis.CurrentResourceYaml, "create proposals diff against an empty document")
	})

	t.Run("create without yaml stays completed", func(t *testing.T) {
		task := &AiTask{Response: &AiResponse{Analysis: Analysis{ProposedOperation: ProposedOperationCreate}}}
		ai.finalizeTaskOutcome(task)
		assert.Equal(t, AI_TASK_STATE_COMPLETED, task.State)
	})

	t.Run("update without target name stays completed", func(t *testing.T) {
		task := &AiTask{Response: &AiResponse{Analysis: Analysis{
			ProposedOperation:  ProposedOperationUpdate,
			TargetResourceYaml: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: foo\n",
		}}}
		ai.finalizeTaskOutcome(task)
		assert.Equal(t, AI_TASK_STATE_COMPLETED, task.State)
	})
}

func TestCanceledByMessage(t *testing.T) {
	assert.Equal(t, "canceled by user", canceledByMessage(structs.User{}))
	assert.Equal(t, "canceled by bene@mogenius.com", canceledByMessage(structs.User{Email: "bene@mogenius.com"}))
}

func TestTaskCancelKey(t *testing.T) {
	assert.Equal(t, "ai_task_cancel:ai_tasks:Agent:calico:cleaner-run-1", taskCancelKey("ai_tasks:Agent:calico:cleaner-run-1"))
}

func TestExecuteProposalValidation(t *testing.T) {
	ai := &aiManager{}

	baseTask := func() *AiTask {
		return &AiTask{Response: &AiResponse{Analysis: Analysis{
			ProposedOperation:  ProposedOperationUpdate,
			TargetResourceYaml: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n  namespace: prod\n",
			TargetResource: utils.WorkloadSingleRequest{
				ResourceDescriptor: utils.ResourceDescriptor{Kind: "Deployment", Plural: "deployments", ApiVersion: "apps/v1", Namespaced: true},
				Namespace:          "prod",
				ResourceName:       "web",
			},
		}}}
	}

	t.Run("missing resource descriptor fails", func(t *testing.T) {
		task := baseTask()
		task.Response.Analysis.TargetResource.Plural = ""
		_, err := ai.executeProposal(task, &ToolContext{})
		assert.ErrorContains(t, err, "descriptor")
	})

	t.Run("yaml name mismatch fails", func(t *testing.T) {
		task := baseTask()
		task.Response.Analysis.TargetResource.ResourceName = "other"
		_, err := ai.executeProposal(task, &ToolContext{})
		assert.ErrorContains(t, err, "does not match")
	})

	t.Run("yaml namespace mismatch fails", func(t *testing.T) {
		task := baseTask()
		task.Response.Analysis.TargetResource.Namespace = "staging"
		_, err := ai.executeProposal(task, &ToolContext{})
		assert.ErrorContains(t, err, "does not match")
	})

	t.Run("missing yaml fails", func(t *testing.T) {
		task := baseTask()
		task.Response.Analysis.TargetResourceYaml = ""
		_, err := ai.executeProposal(task, &ToolContext{})
		assert.ErrorContains(t, err, "no target resource YAML")
	})

	t.Run("unknown operation fails", func(t *testing.T) {
		task := baseTask()
		task.Response.Analysis.ProposedOperation = ProposedOperationOther
		_, err := ai.executeProposal(task, &ToolContext{})
		assert.ErrorContains(t, err, "no executable proposed operation")
	})
}

func TestBuildAgentRunPrompt(t *testing.T) {
	agent := &v1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{Name: "cost-agent"},
		Spec:       v1alpha1.AgentSpec{Instruction: "look for wasted resources"},
	}
	prompt := buildAgentRunPrompt(agent, []string{"prod", "staging"})

	assert.True(t, strings.Contains(prompt, "prod, staging"))
	assert.True(t, strings.Contains(prompt, "look for wasted resources"))
	assert.True(t, strings.Contains(prompt, "read-only"))
}
