package reconciler

import (
	"context"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
)

func (d *reconcilerModule) reconcileFluxCD(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	cfg := spec.GitOps.FluxCD
	if cfg == nil {
		cfg = &v1alpha1.FluxCDInstallConfig{}
	}
	return d.reconcileComponent(ctx, spec, installer, op,
		componentSpec{
			enabled:          cfg.Enabled,
			chart:            cfg.Chart,
			patches:          cfg.Patches,
			name:             componentFluxCD,
			defaultChart:     "flux-operator",
			defaultRepo:      "oci://ghcr.io/controlplaneio-fluxcd/charts/flux-operator",
			defaultName:      "flux-operator",
			defaultNamespace: fluxcdDefaultNamespace,
		},
		func(ctx context.Context) ([]any, error) {
			return nil, nil
		},
		func(ctx context.Context) (map[string]any, error) {
			return nil, nil
		},
	)
}
