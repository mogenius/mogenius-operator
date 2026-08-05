package ai

import (
	"context"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestValidateAgentSpec(t *testing.T) {
	validSpec := v1alpha1.AgentSpec{
		Enabled: true,
		Scope:   v1alpha1.AgentScope{Namespaces: []string{"default"}},
		Triggers: v1alpha1.AgentTriggers{
			Cron: "*/5 * * * *",
			OnChange: &v1alpha1.AgentChangeTrigger{
				Kinds: []string{"Pod"},
				On:    []string{"created", "updated"},
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
			name: "invalid change type",
			mutate: func(spec *v1alpha1.AgentSpec) {
				spec.Triggers.OnChange = &v1alpha1.AgentChangeTrigger{On: []string{"modified"}}
			},
			wantErr: "invalid change type",
		},
		{
			name: "onChange with empty kinds and on is valid (matches all)",
			mutate: func(spec *v1alpha1.AgentSpec) {
				spec.Triggers.OnChange = &v1alpha1.AgentChangeTrigger{}
			},
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

func TestChangeTriggerSelectors(t *testing.T) {
	// Empty lists mean "match all".
	assert.True(t, changeTypeSelected(nil, "created"))
	assert.True(t, changeTypeSelected([]string{"updated"}, "updated"))
	assert.False(t, changeTypeSelected([]string{"updated"}, "created"))

	assert.True(t, kindSelected(nil, "Pod"))
	assert.True(t, kindSelected([]string{"Pod", "Job"}, "Job"))
	assert.False(t, kindSelected([]string{"Pod"}, "Deployment"))

	assert.True(t, namespaceSelected([]string{"*"}, "anything"))
	assert.True(t, namespaceSelected([]string{"prod", "staging"}, "prod"))
	assert.False(t, namespaceSelected([]string{"prod"}, "dev"))
}

func TestChangeCooldownDefault(t *testing.T) {
	assert.Equal(t, defaultChangeCooldown, changeCooldown(nil))
	assert.Equal(t, defaultChangeCooldown, changeCooldown(&v1alpha1.AgentChangeTrigger{}))
	assert.Equal(t, 90*time.Minute, changeCooldown(&v1alpha1.AgentChangeTrigger{MinInterval: metav1.Duration{Duration: 90 * time.Minute}}))
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

func TestCanceledByMessage(t *testing.T) {
	assert.Equal(t, "canceled by user", canceledByMessage(structs.User{}))
	assert.Equal(t, "canceled by bene@mogenius.com", canceledByMessage(structs.User{Email: "bene@mogenius.com"}))
}

func TestTaskCancelKey(t *testing.T) {
	assert.Equal(t, "ai_task_cancel:ai_tasks:Agent:calico:cleaner-run-1", taskCancelKey("ai_tasks:Agent:calico:cleaner-run-1"))
}

func TestCancelLocalRun(t *testing.T) {
	ai := &aiManager{runCancels: make(map[string]context.CancelFunc)}

	ctx, cancel := context.WithCancel(context.Background())
	ai.registerRunCancel("task-1", cancel)

	// A cancel for a task running on another replica is a local no-op.
	ai.cancelLocalRun("task-2")
	assert.NoError(t, ctx.Err())

	ai.cancelLocalRun("task-1")
	assert.ErrorIs(t, ctx.Err(), context.Canceled)

	ai.unregisterRunCancel("task-1")
	ai.cancelLocalRun("task-1")
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

func TestAgentTaskKeyForNamespace(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		namespace string
		expected  string
	}{
		{
			name:      "re-homes an agent finding key to the target namespace",
			key:       "ai_tasks:Agent:argocd:doctor-run-123-f2",
			namespace: "development",
			expected:  "ai_tasks:Agent:development:doctor-run-123-f2",
		},
		{
			name:      "empty namespace leaves the key unchanged",
			key:       "ai_tasks:Agent:argocd:doctor-run-123-f2",
			namespace: "",
			expected:  "ai_tasks:Agent:argocd:doctor-run-123-f2",
		},
		{
			name:      "event task keys are never re-homed",
			key:       "ai_tasks:Deployment:argocd:my-app",
			namespace: "development",
			expected:  "ai_tasks:Deployment:argocd:my-app",
		},
		{
			name:      "malformed keys are left unchanged",
			key:       "ai_tasks:Agent:short",
			namespace: "development",
			expected:  "ai_tasks:Agent:short",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, agentTaskKeyForNamespace(tt.key, tt.namespace))
		})
	}
}

func TestAgentTaskVisibleInNamespaces(t *testing.T) {
	// The workspace under test contains the namespace "development".
	workspace := map[string]bool{"development": true}

	tests := []struct {
		name    string
		task    AiTask
		visible bool
	}{
		{
			name: "task with scope including workspace namespace is visible",
			task: AiTask{
				ID:              "ai_tasks:Agent:argocd:doctor-run-1-f2",
				ScopeNamespaces: []string{"argocd", "development"},
			},
			visible: true,
		},
		{
			name: "all-clear report with wildcard scope is visible everywhere",
			task: AiTask{
				ID:                 "ai_tasks:Agent:argocd:doctor-run-1",
				ScopeNamespaces:    []string{"argocd", "development"},
				ScopeAllNamespaces: true,
			},
			visible: true,
		},
		{
			name: "all-clear report is visible when the scope overlaps the workspace",
			task: AiTask{
				ID:              "ai_tasks:Agent:argocd:doctor-run-1",
				ScopeNamespaces: []string{"argocd", "development"},
			},
			visible: true,
		},
		{
			name: "all-clear report stays hidden when the scope does not overlap",
			task: AiTask{
				ID:              "ai_tasks:Agent:argocd:doctor-run-1",
				ScopeNamespaces: []string{"argocd", "team-backend"},
			},
			visible: false,
		},
		{
			name: "legacy task without scope info falls back to the key namespace",
			task: AiTask{
				ID: "ai_tasks:Agent:development:doctor-run-1",
			},
			visible: true,
		},
		{
			name: "legacy task keyed to a foreign namespace stays hidden",
			task: AiTask{
				ID: "ai_tasks:Agent:argocd:doctor-run-1",
			},
			visible: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.visible, agentTaskVisibleInNamespaces(&tt.task, workspace))
		})
	}
}

func TestLatestTaskNamespaces(t *testing.T) {
	tests := []struct {
		name     string
		task     AiTask
		key      string
		expected []string
	}{
		{
			name:     "event task belongs to its key namespace",
			task:     AiTask{},
			key:      "ai_tasks:Deployment:development:my-app",
			expected: []string{"development"},
		},
		{
			name:     "agent run with scope namespaces belongs to every scope namespace",
			task:     AiTask{ScopeNamespaces: []string{"argocd", "development"}},
			key:      "ai_tasks:Agent:argocd:doctor-run-1",
			expected: []string{"argocd", "development"},
		},
		{
			name:     "legacy agent task falls back to the key namespace",
			task:     AiTask{},
			key:      "ai_tasks:Agent:argocd:doctor-run-1",
			expected: []string{"argocd"},
		},
		{
			name:     "malformed key yields no namespaces",
			task:     AiTask{},
			key:      "ai_tasks:short",
			expected: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, latestTaskNamespaces(&tt.task, tt.key))
		})
	}
}

// The agent cache exists because getEnabledAgents runs on every watch event
// in the cluster (MOG-4518): a fresh cache must be served without any config
// or store access. The nil config/valkeyClient in the bare aiManager doubles
// as the assertion — any fetch attempt panics.
func TestGetEnabledAgentsServesFreshCacheWithoutFetch(t *testing.T) {
	cached := []v1alpha1.Agent{{Spec: v1alpha1.AgentSpec{Enabled: true}}}
	ai := &aiManager{
		cachedEnabledAgents: cached,
		agentCacheValid:     true,
		agentCacheTime:      time.Now(),
	}

	got := ai.getEnabledAgents()

	assert.Equal(t, cached, got)
}

func TestGetEnabledAgentsReturnsPreviousListWhileFetching(t *testing.T) {
	previous := []v1alpha1.Agent{{Spec: v1alpha1.AgentSpec{Enabled: true}}}
	ai := &aiManager{
		cachedEnabledAgents: previous,
		agentCacheValid:     false, // stale — but a fetch is already running
		agentCacheFetching:  true,
	}

	got := ai.getEnabledAgents()

	assert.Equal(t, previous, got)
	assert.True(t, ai.agentCacheFetching, "caller must not clear the in-flight fetch marker")
}

func TestInvalidateAgentCacheBumpsGenerationAndMarksStale(t *testing.T) {
	ai := &aiManager{
		agentCacheValid: true,
		agentCacheTime:  time.Now(),
	}
	genBefore := ai.agentCacheGen

	ai.invalidateAgentCache()

	assert.False(t, ai.agentCacheValid)
	assert.Equal(t, genBefore+1, ai.agentCacheGen, "generation must change so a racing fetch cannot re-validate stale data")
}

func TestProcessObjectInvalidatesCacheOnAgentEvents(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{}}

	ai := &aiManager{agentCacheValid: true, agentCacheTime: time.Now()}
	ai.ProcessObject(obj, "update", utils.AgentResource)
	assert.False(t, ai.agentCacheValid, "Agent CR events must invalidate the cache")

	ai = &aiManager{agentCacheValid: true, agentCacheTime: time.Now()}
	ai.ProcessObject(obj, "update", utils.PodResource)
	assert.True(t, ai.agentCacheValid, "non-Agent events must not invalidate the cache")
}
