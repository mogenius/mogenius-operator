package reconciler

import (
	"context"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
)

func (d *reconcilerModule) reconcileLoki(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	c := spec.Loki
	if c == nil {
		c = &v1alpha1.LokiConfig{}
	}
	return d.reconcileComponent(ctx, spec, installer, op,
		componentSpec{
			enabled:          c.Enabled,
			chart:            c.Chart,
			patch:            c.Patch,
			name:             componentLoki,
			defaultChart:     "loki",
			defaultRepo:      "https://grafana.github.io/helm-charts",
			defaultName:      "loki",
			defaultNamespace: "monitoring",
		},
		func(ctx context.Context) ([]any, error) {
			return []any{}, nil
		},
		func(ctx context.Context) (map[string]any, error) {
			return nil, nil
		},
	)
}

func (d *reconcilerModule) reconcileAlloy(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	c := spec.Alloy
	if c == nil {
		c = &v1alpha1.AlloyConfig{}
	}
	return d.reconcileComponent(ctx, spec, installer, op,
		componentSpec{
			enabled:          c.Enabled,
			chart:            c.Chart,
			patch:            c.Patch,
			name:             componentAlloy,
			defaultChart:     "alloy",
			defaultRepo:      "https://grafana.github.io/helm-charts",
			defaultName:      "alloy",
			defaultNamespace: "monitoring",
		},
		func(ctx context.Context) ([]any, error) {
			return []any{}, nil
		},
		func(ctx context.Context) (map[string]any, error) {
			return nil, nil
		},
	)
}
