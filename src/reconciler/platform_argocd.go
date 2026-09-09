package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"mogenius-operator/src/utils"
	"strings"
)

func (d *reconcilerModule) reconcileArgoCD(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, detection gitOpsDetection) *ReconcileResult {
	// Not declared: leave whatever is installed alone (see reconcileComponent).
	// Unreachable through reconcilePlatformConfig, which only dispatches here
	// for an engine the spec enables — but a nil spec.GitOps would panic.
	if spec.GitOps == nil || spec.GitOps.ArgoCD == nil {
		return nil
	}
	cfg := spec.GitOps.ArgoCD
	return d.reconcileGitOpsEngine(ctx, spec, installer, detection, engineSpec{
		engine: gitOpsEngineArgoCD,
		component: componentSpec{
			enabled:          cfg.Enabled,
			chart:            cfg.Chart,
			patches:          cfg.Patches,
			name:             componentArgoCD,
			defaultChart:     "argo-cd",
			defaultRepo:      "https://argoproj.github.io/argo-helm",
			defaultName:      "argocd",
			defaultNamespace: argocdDefaultNamespace,
		},
		// The AppProject has to exist before anything is placed in it — and
		// every Application the platform creates is, including the one that
		// would otherwise have carried this object.
		bootstrapObjects: func(namespace string) []engineBootstrapObject {
			return []engineBootstrapObject{{
				resource: utils.AppProjectResource,
				object: map[string]any{
					"apiVersion": utils.AppProjectResource.ApiVersion,
					"kind":       utils.AppProjectResource.Kind,
					"metadata": map[string]any{
						"name":      argoProjectName(spec.GitOps),
						"namespace": namespace,
					},
					"spec": map[string]any{
						"clusterResourceWhitelist": []map[string]any{{"group": "*", "kind": "*"}},
						"destinations":             []map[string]any{{"namespace": "*", "server": "*"}},
						"sourceRepos":              []string{"*"},
					},
				},
			}}
		},
		extraObjects: func(ctx context.Context) ([]any, error) {
			namespace := helmNamespace(cfg.Chart, argocdDefaultNamespace)
			project := argoProjectName(spec.GitOps)
			extraObjects := []any{}

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
						extraObjects = append(extraObjects, externalSecretResource(name, namespace, *repo.ExternalSecret,
							map[string]string{"argocd.argoproj.io/secret-type": "repository"},
							map[string]string{
								"type":     "git",
								"url":      repo.URL,
								"username": "x-token-auth",
								"password": fmt.Sprintf("{{ .%s }}", repoSecretKey),
							},
						))
					}
				}

				if repo.Path != "" {
					if strings.HasSuffix(repo.Path, "/**") {
						extraObjects = append(extraObjects, argoAppSetObject(name, repo, namespace, project))
					} else {
						extraObjects = append(extraObjects, argoApplicationObject(name, repo, namespace, project))
					}
				}

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
				metricsBlock := map[string]any{
					"enabled": true,
					"serviceMonitor": map[string]any{
						"enabled": true,
					},
				}
				return map[string]any{
					"controller":     map[string]any{"metrics": metricsBlock},
					"server":         map[string]any{"metrics": metricsBlock},
					"repoServer":     map[string]any{"metrics": metricsBlock},
					"applicationSet": map[string]any{"metrics": metricsBlock},
				}, nil
			}
			return nil, nil
		},
	})
}

func argoApplicationObject(name string, repo v1alpha1.GitOpsRepositoryConfig, namespace, project string) map[string]any {
	revision := repo.Revision
	if revision == "" {
		revision = "HEAD"
	}
	return map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]any{
			"name":       name,
			"namespace":  namespace,
			"finalizers": []any{"resources-finalizer.argocd.argoproj.io"},
		},
		"spec": map[string]any{
			"project": project,
			"source": map[string]any{
				"repoURL":        repo.URL,
				"path":           repo.Path,
				"targetRevision": revision,
			},
			"destination": map[string]any{
				"name":      "in-cluster",
				"namespace": "default",
			},
			"syncPolicy": map[string]any{
				"automated":   map[string]any{"prune": true, "selfHeal": true},
				"syncOptions": []any{"CreateNamespace=true", "ServerSideApply=true"},
			},
		},
	}
}

func argoAppSetObject(name string, repo v1alpha1.GitOpsRepositoryConfig, namespace, project string) map[string]any {
	revision := repo.Revision
	if revision == "" {
		revision = "HEAD"
	}
	return map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "ApplicationSet",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]any{
			"generators": []any{
				map[string]any{
					"git": map[string]any{
						"repoURL":  repo.URL,
						"revision": revision,
						"directories": []any{
							map[string]any{"path": repo.Path},
						},
					},
				},
			},
			"template": map[string]any{
				"metadata": map[string]any{
					"name": "{{path.basename}}-app",
				},
				"spec": map[string]any{
					"project": project,
					"source": map[string]any{
						"repoURL":        repo.URL,
						"path":           "{{path}}",
						"targetRevision": revision,
					},
					"destination": map[string]any{
						"name":      "in-cluster",
						"namespace": "{{path.basename}}",
					},
					"syncPolicy": map[string]any{
						"automated":   map[string]any{"prune": true, "selfHeal": true},
						"syncOptions": []any{"CreateNamespace=true", "ServerSideApply=true"},
					},
				},
			},
		},
	}
}
