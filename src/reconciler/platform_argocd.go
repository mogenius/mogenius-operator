package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"mogenius-operator/src/utils"
	"strings"
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
					"clusterResourceWhitelist": []map[string]any{{"group": "*", "kind": "*"}},
					"destinations":             []map[string]any{{"namespace": "*", "server": "*"}},
					"sourceRepos":              []string{"*"},
				},
			}
			extraObjects = append(extraObjects, appProject)

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
			}

			return extraObjects, nil
		},
		func(ctx context.Context) (map[string]any, error) {
			return nil, nil
		},
	)
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
