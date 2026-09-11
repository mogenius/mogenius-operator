package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"mogenius-operator/src/utils"
)

func (d *reconcilerModule) reconcileFluxCD(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, detection gitOpsDetection) *ReconcileResult {
	// Not declared: leave whatever is installed alone (see reconcileComponent).
	// Unreachable through reconcilePlatformConfig, which only dispatches here
	// for an engine the spec enables — but a nil spec.GitOps would panic.
	if spec.GitOps == nil || spec.GitOps.FluxCD == nil {
		return nil
	}
	cfg := spec.GitOps.FluxCD
	return d.reconcileGitOpsEngine(ctx, spec, installer, detection, engineSpec{
		engine: gitOpsEngineFlux,
		component: componentSpec{
			enabled:          cfg.Enabled,
			chart:            cfg.Chart,
			patches:          cfg.Patches,
			name:             componentFluxCD,
			defaultChart:     "flux-operator",
			defaultRepo:      "oci://ghcr.io/controlplaneio-fluxcd/charts/flux-operator",
			defaultName:      "flux-operator",
			defaultNamespace: fluxcdDefaultNamespace,
		},
		// The FluxInstance is what actually deploys the Flux controllers; the
		// flux-operator chart alone installs none. It therefore cannot ship as
		// an extra object, because those travel through a HelmRelease that
		// needs the very helm-controller this resource brings up.
		bootstrapObjects: func(namespace string) []engineBootstrapObject {
			return []engineBootstrapObject{
				{resource: utils.FluxInstanceResource, object: fluxInstanceObject(namespace)},
			}
		},
		extraObjects: func(ctx context.Context) ([]any, error) {
			namespace := helmNamespace(cfg.Chart, fluxcdDefaultNamespace)
			extraObjects := []any{}

			for _, repo := range spec.GitOps.Repositories {
				// Application repositories are declared for the mogenius
				// platform, not for this operator: no sync objects.
				if !repo.IsPlatformRepository() {
					continue
				}
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

				// The same credential decision the user-managed path makes: a
				// private repository is unreadable without it, and the Secret
				// sits right next to the source. This path shipped without it
				// once -- the engine ran, cloned nothing, and the only symptom
				// was "authentication required" on the GitRepository.
				extraObjects = append(extraObjects,
					fluxGitRepositoryObject(name, repo, namespace, d.fluxRepositorySecretName(ctx, name, repo, namespace)),
					fluxKustomizationObject(name, repo, namespace),
				)

				if repo.Write != nil &&
					repo.Write.Enabled != nil &&
					*repo.Write.Enabled &&
					repo.Write.SecretRef.ExternalSecret != nil {

					operatorNamespace := d.config.Get("MO_OWN_NAMESPACE")
					extraObjects = append(extraObjects, externalSecretResource(name+"-write", operatorNamespace, *repo.Write.SecretRef.ExternalSecret, nil, nil))
				}
			}

			return extraObjects, nil
		},
		extraValues: func(ctx context.Context) (map[string]any, error) {
			if d.crdChecker.IsAvailable(utils.ServiceMonitorResource) {
				return map[string]any{
					"serviceMonitor": map[string]any{
						"create": true,
					},
				}, nil
			}
			return nil, nil
		},
	})
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

// fluxGitRepositoryObject builds the source. secretName is the credential the
// source binds with, empty for none -- the caller decides, because deciding
// takes a cluster lookup (fluxRepositorySecretName) and this stays a pure
// builder.
func fluxGitRepositoryObject(name string, repo v1alpha1.GitOpsRepositoryConfig, namespace string, secretName string) map[string]any {
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
	if secretName != "" {
		spec["secretRef"] = map[string]any{"name": secretName}
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
