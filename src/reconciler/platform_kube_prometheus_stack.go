package reconciler

import (
	"context"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
)

func (d *reconcilerModule) reconcileKubePrometheusStack(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	c := spec.KubePrometheusStack
	if c == nil {
		c = &v1alpha1.KubePrometheusStackConfig{}
	}
	return d.reconcileComponent(ctx, spec, installer, op,
		componentSpec{
			enabled:          c.Enabled,
			chart:            c.Chart,
			patch:            c.Patch,
			name:             componentKubePrometheusStack,
			defaultChart:     "kube-prometheus-stack",
			defaultRepo:      "https://prometheus-community.github.io/helm-charts",
			defaultName:      "kube-prometheus-stack",
			defaultNamespace: "monitoring",
		},
		func(ctx context.Context) ([]any, error) {
			return []any{}, nil
		},
		func(ctx context.Context) (map[string]interface{}, error) {
			return nil, nil
		},
	)
}
