package reconciler

import (
	"context"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
)

func (d *reconcilerModule) reconcileRenovateOperator(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	c := spec.RenovateOperator
	if c == nil {
		c = &v1alpha1.RenovateOperatorConfig{}
	}
	return d.reconcileComponent(ctx, spec, installer, op,
		componentSpec{
			enabled:          c.Enabled,
			chart:            c.Chart,
			patch:            c.Patch,
			name:             componentRenovateOperator,
			defaultChart:     "renovate-operator",
			defaultRepo:      "oci://ghcr.io/mogenius/helm-charts/renovate-operator",
			defaultName:      "renovate-operator",
			defaultNamespace: "renovate-operator",
		},
		func(ctx context.Context) ([]any, error) {
			return []any{}, nil
		},
		func(ctx context.Context) (map[string]any, error) {
			return nil, nil
		},
	)
}
