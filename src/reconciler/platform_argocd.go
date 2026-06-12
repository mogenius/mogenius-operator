package reconciler

import (
	"context"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"mogenius-operator/src/utils"
)

func (d *reconcilerModule) reconcileArgoCD(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	cfg := spec.GitOps.ArgoCD
	if cfg == nil {
		cfg = &v1alpha1.ArgoCDInstallConfig{}
	}
	namespace := helmNamespace(spec.GitOps.ArgoCD.Chart, argocdDefaultNamespace)
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
			extraObjects := []any{}

			project := "mogenius"
			if spec.GitOps.ArgoCD.Project != "" {
				project = spec.GitOps.ArgoCD.Project
			}

			appProject := map[string]any{
				"apiVersion": utils.AppProjectResource.ApiVersion,
				"kind":       utils.AppProjectResource.Kind,
				"metadata": map[string]any{
					"name":      project,
					"namespace": namespace,
				},
				"spec": map[string]any{
					"clusterResourceWhitelist": []map[string]any{{
						"group": "*",
						"kind":  "*",
					},
					},
					"destinations": []map[string]any{{
						"namespace": "*",
						"server":    "*",
					},
					},
					"sourceRepos": []string{
						"*",
					},
				},
			}
			extraObjects = append(extraObjects, appProject)

			return extraObjects, nil
		},
		func(ctx context.Context) (map[string]any, error) {
			return nil, nil
		},
	)
}
