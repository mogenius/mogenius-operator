package reconciler

import (
	"context"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
)

// reconcileAgentSandboxes installs the mogenius agent-sandbox stack as a
// platform component. It reuses the existing mogenius-agent-sandbox Helm chart
// (published to https://helm.mogenius.com/public) as the install unit — the
// chart ships the sandbox namespace, the mogenius-provided default
// SandboxTemplate/WarmPool objects and the egress NetworkPolicies, so there are
// no extra objects to emit here. The spec only templates the chart's values.
//
// The value keys written below (sandboxes.namespace.name,
// sandboxes.networkPolicy.{managed,additionalBlockedCidrs}) are honoured by the
// chart as-is; leaving a field unset keeps the chart / platform-defaults default.
func (d *reconcilerModule) reconcileAgentSandboxes(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	c := spec.AgentSandboxes
	// Not declared: leave whatever is installed alone (see reconcileComponent).
	if c == nil {
		return nil
	}
	return d.reconcileComponent(ctx, spec, installer, op,
		componentSpec{
			enabled:          c.Enabled,
			chart:            c.Chart,
			patches:          c.Patches,
			name:             componentAgentSandboxes,
			defaultChart:     "mogenius-agent-sandbox",
			defaultRepo:      "https://helm.mogenius.com/public",
			defaultName:      "agent-sandbox",
			defaultNamespace: "agent-sandbox-system",
		},
		func(ctx context.Context) ([]any, error) {
			return []any{}, nil
		},
		func(ctx context.Context) (map[string]any, error) {
			return buildAgentSandboxValues(c), nil
		},
	)
}

// buildAgentSandboxValues templates the spec into the chart's `sandboxes` values
// tree. Only keys the spec actually sets are written, so an unset field keeps the
// chart / platform-defaults default instead of being forced to its zero value.
// Returns nil when there is nothing to override.
func buildAgentSandboxValues(c *v1alpha1.AgentSandboxesConfig) map[string]any {
	sandboxes := map[string]any{}

	if c.Namespace != "" {
		sandboxes["namespace"] = map[string]any{"name": c.Namespace}
	}

	if np := c.NetworkPolicy; np != nil {
		networkPolicy := map[string]any{}
		if np.Managed != nil {
			networkPolicy["managed"] = *np.Managed
		}
		if len(np.AdditionalBlockedCidrs) > 0 {
			networkPolicy["additionalBlockedCidrs"] = np.AdditionalBlockedCidrs
		}
		if len(networkPolicy) > 0 {
			sandboxes["networkPolicy"] = networkPolicy
		}
	}

	if len(sandboxes) == 0 {
		return nil
	}
	return map[string]any{"sandboxes": sandboxes}
}
