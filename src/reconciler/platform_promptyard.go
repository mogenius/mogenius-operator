package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
)

// promptyard points Flux directly at the signed promptyard-gitops-infra OCI
// artifact instead of mogenius's own per-component delivery. See
// docs/SETUP.md step 6 in that repository: the pull secret and the Flux
// Operator itself (spec.gitOps.fluxcd, required by the CRD's XValidation
// rules) are prerequisites this component does not manage.
const (
	promptyardOCIRepositoryURL         = "oci://ghcr.io/mogenius/promptyard-gitops-infra"
	promptyardPullSecretName           = "promptyard-oci"
	promptyardRuntimeInfoConfigMapName = "flux-runtime-info"
)

// reconcilePromptyard goes through reconcileComponent like every other
// component. It declares no chart (see hasChart/buildComponentArtifact):
// promptyard-gitops-infra is a plain Kustomize tree, not a Helm chart, so
// there is no HelmRelease to template it into. Its OCIRepository,
// Kustomization and the flux-runtime-info ConfigMap all ship as extra
// objects instead, through installer.ApplyExtras — the same path the GitOps
// engine's own chart uses for what sits beside it.
func (d *reconcilerModule) reconcilePromptyard(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	c := spec.Promptyard
	// Not declared: leave whatever is installed alone (see reconcileComponent).
	if c == nil {
		return nil
	}

	namespace := fluxcdDefaultNamespace
	if spec.GitOps != nil && spec.GitOps.FluxCD != nil {
		namespace = helmNamespace(spec.GitOps.FluxCD.Chart, fluxcdDefaultNamespace)
	}

	return d.reconcileComponent(ctx, spec, installer, op,
		componentSpec{
			enabled:          c.Enabled,
			name:             componentPromptyard,
			defaultNamespace: namespace,
		},
		func(ctx context.Context) ([]any, error) {
			defaults, err := getDefaultConfig(spec.PlatformSource, spec.PlatformVersion, componentPromptyard)
			if err != nil {
				return nil, fmt.Errorf("fetch default config for %s: %w", componentPromptyard, err)
			}

			return []any{
				promptyardOCIRepositoryObject(namespace, defaults.Version),
				promptyardKustomizationObject(namespace),
				promptyardRuntimeInfoConfigMap(namespace, c.Settings),
			}, nil
		},
		nil,
	)
}

// promptyardOCIRepositoryObject sources promptyard-gitops-infra directly.
// Only release artifacts signed by that repository's release workflow are
// accepted; the pull secret is a prerequisite this operator does not manage
// (see docs/SETUP.md step 6 in that repository).
func promptyardOCIRepositoryObject(namespace, version string) map[string]any {
	semver := version
	if semver == "" {
		semver = "*"
	}

	return map[string]any{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "OCIRepository",
		"metadata": map[string]any{
			"name":      componentPromptyard,
			"namespace": namespace,
		},
		"spec": map[string]any{
			"interval": "10m",
			"url":      promptyardOCIRepositoryURL,
			"ref": map[string]any{
				"semver": semver,
			},
			"secretRef": map[string]any{
				"name": promptyardPullSecretName,
			},
			"verify": map[string]any{
				"provider": "cosign",
				"matchOIDCIdentity": []any{
					map[string]any{
						"issuer":  `^https://token\.actions\.githubusercontent\.com$`,
						"subject": `^https://github\.com/mogenius/promptyard-gitops-infra/\.github/workflows/release\.yaml@refs/heads/main$`,
					},
				},
			},
		},
	}
}

// promptyardKustomizationObject syncs cluster/default of the OCIRepository
// above, with the flux-runtime-info ConfigMap substituted into its manifests.
func promptyardKustomizationObject(namespace string) map[string]any {
	return map[string]any{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata": map[string]any{
			"name":      componentPromptyard,
			"namespace": namespace,
		},
		"spec": map[string]any{
			"interval": "1m",
			"sourceRef": map[string]any{
				"kind": "OCIRepository",
				"name": componentPromptyard,
			},
			"path":  "./cluster/default",
			"prune": true,
			"postBuild": map[string]any{
				"substituteFrom": []any{
					map[string]any{
						"kind": "ConfigMap",
						"name": promptyardRuntimeInfoConfigMapName,
					},
				},
			},
		},
	}
}

// promptyardRuntimeInfoConfigMap carries spec.promptyard.settings into the
// Kustomization's postBuild substitution above.
func promptyardRuntimeInfoConfigMap(namespace string, settings map[string]string) map[string]any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      promptyardRuntimeInfoConfigMapName,
			"namespace": namespace,
			"labels": map[string]any{
				"toolkit.fluxcd.io/runtime": "true",
				"reconcile.fluxcd.io/watch": "Enabled",
			},
		},
		"data": settings,
	}
}
