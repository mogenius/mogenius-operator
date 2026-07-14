package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/ai"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/store"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const agentReadyCondition = "Ready"

// reconcileAgents validates Agent CRs and reports the result as a "Ready"
// status condition, so declaratively managed agents (kubectl/GitOps) get
// feedback without the UI: `kubectl get agents -n mogenius` shows READY and
// REASON columns. Spec validation itself stays fail-closed at run time; the
// condition is purely informational.
func (d *reconcilerModule) reconcileAgents(ctx context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	if op == deleteOperation {
		return nil
	}

	var agent v1alpha1.Agent
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &agent); err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("failed to parse Agent: %w", err)}}
	}

	conditionStatus, reason, message := d.evaluateAgent(&agent)

	current := apimeta.FindStatusCondition(agent.Status.Conditions, agentReadyCondition)
	upToDate := current != nil &&
		current.Status == conditionStatus &&
		current.Reason == reason &&
		current.Message == message &&
		agent.Status.ObservedGeneration == agent.Generation
	if !upToDate {
		apimeta.SetStatusCondition(&agent.Status.Conditions, metav1.Condition{
			Type:               agentReadyCondition,
			Status:             conditionStatus,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: agent.Generation,
		})
		agent.Status.ObservedGeneration = agent.Generation
		if _, err := d.clientProvider.MogeniusClientSet().MogeniusV1alpha1.UpdateAgentStatus(&agent); err != nil {
			return []ReconcileResult{{Err: fmt.Errorf("failed to update status of agent %q: %w", agent.Name, err)}}
		}
	}

	// Surface user configuration problems as warnings in the reconciler
	// status API as well — the condition alone is easy to miss.
	if conditionStatus == metav1.ConditionFalse {
		return []ReconcileResult{{Err: fmt.Errorf("agent %q is not ready: %s: %s", agent.Name, reason, message), IsWarning: true}}
	}
	return nil
}

// evaluateAgent computes the Ready condition for an agent. Fail reasons are
// CamelCase identifiers per the metav1.Condition convention.
func (d *reconcilerModule) evaluateAgent(agent *v1alpha1.Agent) (metav1.ConditionStatus, string, string) {
	ownNamespace := d.config.Get("MO_OWN_NAMESPACE")
	if agent.Namespace != ownNamespace {
		return metav1.ConditionFalse, "IgnoredNamespace", fmt.Sprintf("agents are only processed in namespace %q", ownNamespace)
	}

	if err := ai.ValidateAgentSpec(agent.Spec); err != nil {
		return metav1.ConditionFalse, "InvalidSpec", err.Error()
	}

	if ref := agent.Spec.Scope.WorkspaceRef; ref != "" {
		workspace, err := store.GetWorkspace(ownNamespace, ref)
		if err != nil || workspace == nil {
			return metav1.ConditionFalse, "WorkspaceNotFound", fmt.Sprintf("scope references workspace %q which does not exist", ref)
		}
	}

	if !agent.Spec.Enabled {
		return metav1.ConditionTrue, "Valid", "spec is valid; agent is disabled"
	}
	return metav1.ConditionTrue, "Valid", "spec is valid"
}
