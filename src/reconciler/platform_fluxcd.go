package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"mogenius-operator/src/utils"
)

func (d *reconcilerModule) reconcileFluxCD(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	cfg := spec.GitOps.FluxCD
	if cfg == nil {
		cfg = &v1alpha1.FluxCDInstallConfig{}
	}
	namespace := helmNamespace(cfg.Chart, fluxcdDefaultNamespace)
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
			// The FluxInstance is what actually deploys the Flux controllers; the
			// flux-operator chart alone installs none. Since extra objects ship via a
			// moac HelmRelease that needs helm-controller to reconcile, the very first
			// install requires the Flux controllers to be bootstrapped out-of-band.
			extraObjects := []any{fluxInstanceObject(namespace)}

			for _, repo := range spec.GitOps.Repositories {
				name := repo.Name
				if name == "" {
					name = repositorySecretName(repo.URL)
				}

				if repo.ExternalSecret != nil {
					if repo.ExternalSecret.Vault == "" {
						if spec.ExternalSecretsOperator != nil && len(spec.ExternalSecretsOperator.Vaults) > 0 {
							repo.ExternalSecret.Vault = spec.ExternalSecretsOperator.Vaults[0].Name
						} else {
							return nil, fmt.Errorf("repository %q: provide externalSecret.vault or define a vault in spec.externalSecretsOperator", repo.URL)
						}
					}
					repoSecretKey := "token"
					if repo.ExternalSecret.Key != "" {
						repoSecretKey = repo.ExternalSecret.Key
					}
					if d.crdChecker.IsAvailable(utils.ExternalSecretResource) {
						extraObjects = append(extraObjects, externalSecretResource(name, namespace, *repo.ExternalSecret, nil,
							map[string]string{
								"username": "git",
								"password": fmt.Sprintf("{{ .%s }}", repoSecretKey),
							},
						))
					}
				}

				extraObjects = append(extraObjects,
					fluxGitRepositoryObject(name, repo, namespace),
					fluxKustomizationObject(name, repo, namespace),
				)
			}

			return extraObjects, nil
		},
		func(ctx context.Context) (map[string]any, error) {
			if d.crdChecker.IsAvailable(utils.ServiceMonitorResource) {
				return map[string]any{
					"serviceMonitor": map[string]any{
						"create": true,
					},
				}, nil
			}
			return nil, nil
		},
	)
}

func fluxInstanceObject(namespace string) map[string]any {
	return map[string]any{
		"apiVersion": "fluxcd.controlplane.io/v1",
		"kind":       "FluxInstance",
		"metadata": map[string]any{
			"name":      "flux",
			"namespace": namespace,
		},
		"spec": map[string]any{
			"distribution": map[string]any{
				"version":  "2.x",
				"registry": "ghcr.io/fluxcd",
			},
			"components": []any{
				"source-controller",
				"kustomize-controller",
				"helm-controller",
				"notification-controller",
			},
		},
	}
}

func fluxKustomizationObject(name string, repo v1alpha1.GitOpsRepositoryConfig, namespace string) map[string]any {
	path := repo.Path
	if path == "" {
		path = "./"
	}
	return map[string]any{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]any{
			"interval": "1m",
			"sourceRef": map[string]any{
				"kind": "GitRepository",
				"name": name,
			},
			"path":  path,
			"prune": true,
		},
	}
}

func fluxGitRepositoryObject(name string, repo v1alpha1.GitOpsRepositoryConfig, namespace string) map[string]any {
	revision := repo.Revision
	if revision == "" {
		revision = "main"
	}
	spec := map[string]any{
		"interval": "1m",
		"url":      repo.URL,
		"ref": map[string]any{
			"branch": revision,
		},
	}
	if repo.ExternalSecret != nil {
		spec["secretRef"] = map[string]any{"name": name}
	}
	return map[string]any{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "GitRepository",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
		"spec": spec,
	}
}
