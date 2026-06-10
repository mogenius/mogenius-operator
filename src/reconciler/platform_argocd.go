package reconciler

import (
	"context"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
)

func (d *reconcilerModule) reconcileArgoCD(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	cfg := spec.GitOps.ArgoCD
	if cfg == nil {
		cfg = &v1alpha1.ArgoCDInstallConfig{}
	}
	return d.reconcileComponent(ctx, spec, installer, op,
		componentSpec{
			enabled:          cfg.Enabled,
			chart:            cfg.Chart,
			patches:          cfg.Patches,
			name:             componentArgoCD,
			defaultChart:     "argo-cd",
			defaultRepo:      "https://argoproj.github.io/argo-helm",
			defaultName:      "argocd",
			defaultNamespace: argocdDefaultNamespace,
		},
		func(ctx context.Context) ([]any, error) {
			return nil, nil
		},
		func(ctx context.Context) (map[string]any, error) {
			return nil, nil
		},
	)
}
