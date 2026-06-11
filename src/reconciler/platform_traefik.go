package reconciler

import (
	"context"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
)

func (d *reconcilerModule) reconcileTraefik(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	t := spec.Traefik
	if t == nil {
		t = &v1alpha1.TraefikConfig{}
	}
	return d.reconcileComponent(ctx, spec, installer, op,
		componentSpec{
			enabled:          t.Enabled,
			chart:            t.Chart,
			patches:          t.Patches,
			name:             componentTraefik,
			defaultChart:     "traefik",
			defaultRepo:      "https://helm.traefik.io/traefik",
			defaultName:      "traefik",
			defaultNamespace: "traefik",
		},
		func(ctx context.Context) ([]any, error) {
			return []any{}, nil
		},
		func(ctx context.Context) (map[string]any, error) {
			values := map[string]any{}

			if spec.Traefik.Service != nil {
				values["service"] = spec.Traefik.Service
			}

			return values, nil
		},
	)
}
