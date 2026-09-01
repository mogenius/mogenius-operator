package reconciler

import (
	"mogenius-operator/src/config"
	"mogenius-operator/src/crds/v1alpha1"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newAgentTestModule(t *testing.T) *reconcilerModule {
	t.Helper()
	cfg := config.NewConfig()
	ownNamespace := "mogenius"
	cfg.Declare(config.ConfigDeclaration{Key: "MO_OWN_NAMESPACE", DefaultValue: &ownNamespace})
	return &reconcilerModule{config: cfg}
}

func agentFixture(namespace string, spec v1alpha1.AgentSpec) *v1alpha1.Agent {
	return &v1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{Name: "test-agent", Namespace: namespace, Generation: 3},
		Spec:       spec,
	}
}

func TestEvaluateAgentIgnoredNamespace(t *testing.T) {
	module := newAgentTestModule(t)
	agent := agentFixture("default", v1alpha1.AgentSpec{
		Enabled: true,
		Scope:   &v1alpha1.AgentScope{WorkspaceRef: "prod"},
	})

	status, reason, message := module.evaluateAgent(agent)
	assert.Equal(t, metav1.ConditionFalse, status)
	assert.Equal(t, "IgnoredNamespace", reason)
	assert.Contains(t, message, `"mogenius"`)
}

func TestEvaluateAgentInvalidSpec(t *testing.T) {
	module := newAgentTestModule(t)

	// Invalid cron expression must be rejected.
	agent := agentFixture("mogenius", v1alpha1.AgentSpec{
		Enabled:  true,
		Scope:    &v1alpha1.AgentScope{WorkspaceRef: "prod"},
		Triggers: v1alpha1.AgentTriggers{Cron: "not-a-cron"},
	})
	status, reason, message := module.evaluateAgent(agent)
	assert.Equal(t, metav1.ConditionFalse, status)
	assert.Equal(t, "InvalidSpec", reason)
	assert.Contains(t, message, "cron")

	// Change trigger with an invalid change type must be rejected.
	agent = agentFixture("mogenius", v1alpha1.AgentSpec{
		Enabled: true,
		Scope:   &v1alpha1.AgentScope{WorkspaceRef: "prod"},
		Triggers: v1alpha1.AgentTriggers{
			OnChange: &v1alpha1.AgentChangeTrigger{On: []string{"modified"}},
		},
	})
	status, reason, _ = module.evaluateAgent(agent)
	assert.Equal(t, metav1.ConditionFalse, status)
	assert.Equal(t, "InvalidSpec", reason)
}

func TestEvaluateAgentValid(t *testing.T) {
	module := newAgentTestModule(t)

	// Valid enabled agent with a cron trigger; nil scope = wildcard (avoids workspace lookup in unit tests).
	agent := agentFixture("mogenius", v1alpha1.AgentSpec{
		Enabled:  true,
		Triggers: v1alpha1.AgentTriggers{Cron: "0 6 * * 1"},
	})
	status, reason, message := module.evaluateAgent(agent)
	assert.Equal(t, metav1.ConditionTrue, status)
	assert.Equal(t, "Valid", reason)
	assert.NotContains(t, message, "disabled")

	// Wildcard scope (nil) is valid.
	agent = agentFixture("mogenius", v1alpha1.AgentSpec{
		Enabled: true,
	})
	status, reason, _ = module.evaluateAgent(agent)
	assert.Equal(t, metav1.ConditionTrue, status)
	assert.Equal(t, "Valid", reason)

	// Disabled agents are still valid, but the message says so.
	agent = agentFixture("mogenius", v1alpha1.AgentSpec{
		Enabled: false,
	})
	status, _, message = module.evaluateAgent(agent)
	assert.Equal(t, metav1.ConditionTrue, status)
	assert.Contains(t, message, "disabled")
}
