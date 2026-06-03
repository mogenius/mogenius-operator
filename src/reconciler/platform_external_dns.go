package reconciler

import (
	"context"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
)

func (d *reconcilerModule) reconcileExternalDNS(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	c := spec.ExternalDNS
	if c == nil {
		c = &v1alpha1.ExternalDNSConfig{}
	}
	return d.reconcileComponent(ctx, spec, installer, op,
		componentSpec{
			enabled:          c.Enabled,
			chart:            c.Chart,
			patch:            c.Patch,
			name:             componentExternalDNS,
			defaultChart:     "external-dns",
			defaultRepo:      "https://kubernetes-sigs.github.io/external-dns/",
			defaultName:      "external-dns",
			defaultNamespace: "external-dns",
		},
		func(ctx context.Context) ([]any, error) {
			return []any{}, nil
		},
		func(ctx context.Context) (map[string]any, error) {
			return nil, nil
		},
	)
}
