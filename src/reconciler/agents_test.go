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
		Scope:   v1alpha1.AgentScope{Namespaces: []string{"default"}},
	})

	status, reason, message := module.evaluateAgent(agent)
	assert.Equal(t, metav1.ConditionFalse, status)
	assert.Equal(t, "IgnoredNamespace", reason)
	assert.Contains(t, message, `"mogenius"`)
}

func TestEvaluateAgentInvalidSpec(t *testing.T) {
	module := newAgentTestModule(t)

	// Empty scope must be rejected.
	agent := agentFixture("mogenius", v1alpha1.AgentSpec{Enabled: true})
	status, reason, _ := module.evaluateAgent(agent)
	assert.Equal(t, metav1.ConditionFalse, status)
	assert.Equal(t, "InvalidSpec", reason)

	// Invalid cron expression must be rejected.
	agent = agentFixture("mogenius", v1alpha1.AgentSpec{
		Enabled:  true,
		Scope:    v1alpha1.AgentScope{Namespaces: []string{"prod"}},
		Triggers: v1alpha1.AgentTriggers{Cron: "not-a-cron"},
	})
	status, reason, message := module.evaluateAgent(agent)
	assert.Equal(t, metav1.ConditionFalse, status)
	assert.Equal(t, "InvalidSpec", reason)
	assert.Contains(t, message, "cron")

	// Event filter without contains conditions must be rejected.
	agent = agentFixture("mogenius", v1alpha1.AgentSpec{
		Enabled: true,
		Scope:   v1alpha1.AgentScope{Namespaces: []string{"prod"}},
		Triggers: v1alpha1.AgentTriggers{
			Events: []v1alpha1.AgentEventFilter{{Kind: "Pod"}},
		},
	})
	status, reason, _ = module.evaluateAgent(agent)
	assert.Equal(t, metav1.ConditionFalse, status)
	assert.Equal(t, "InvalidSpec", reason)
}

func TestEvaluateAgentValid(t *testing.T) {
	module := newAgentTestModule(t)

	agent := agentFixture("mogenius", v1alpha1.AgentSpec{
		Enabled:  true,
		Scope:    v1alpha1.AgentScope{Namespaces: []string{"prod", "staging"}},
		Triggers: v1alpha1.AgentTriggers{Cron: "0 6 * * 1", Manual: true},
	})
	status, reason, message := module.evaluateAgent(agent)
	assert.Equal(t, metav1.ConditionTrue, status)
	assert.Equal(t, "Valid", reason)
	assert.NotContains(t, message, "disabled")

	// Wildcard scope is valid.
	agent = agentFixture("mogenius", v1alpha1.AgentSpec{
		Enabled: true,
		Scope:   v1alpha1.AgentScope{Namespaces: []string{"*"}},
	})
	status, reason, _ = module.evaluateAgent(agent)
	assert.Equal(t, metav1.ConditionTrue, status)
	assert.Equal(t, "Valid", reason)

	// Disabled agents are still valid, but the message says so.
	agent = agentFixture("mogenius", v1alpha1.AgentSpec{
		Enabled: false,
		Scope:   v1alpha1.AgentScope{Namespaces: []string{"prod"}},
	})
	status, _, message = module.evaluateAgent(agent)
	assert.Equal(t, metav1.ConditionTrue, status)
	assert.Contains(t, message, "disabled")
}
