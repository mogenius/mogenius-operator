package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"mogenius-operator/src/utils"
)

// vibeCoding points Flux directly at the signed vibe-coding-gitops-infra OCI
// artifact instead of mogenius's own per-component delivery. See
// docs/SETUP.md step 6 in that repository: the pull secret and the Flux
// Operator itself (spec.gitOps.fluxcd, required by the CRD's XValidation
// rules) are prerequisites this component does not manage.
const (
	vibeCodingOCIRepositoryURL         = "oci://ghcr.io/mogenius/vibe-coding-gitops-infra"
	vibeCodingPullSecretName           = "vibe-coding-oci"
	vibeCodingRuntimeInfoConfigMapName = "flux-runtime-info"
)

// reconcileVibeCoding goes through reconcileComponent like every other
// component. It declares no chart (see hasChart/buildComponentArtifact):
// vibe-coding-gitops-infra is a plain Kustomize tree, not a Helm chart, so
// there is no HelmRelease to template it into. Its OCIRepository,
// Kustomization and the flux-runtime-info ConfigMap all ship as extra
// objects instead, through installer.ApplyExtras — the same path the GitOps
// engine's own chart uses for what sits beside it.
func (d *reconcilerModule) reconcileVibeCoding(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	c := spec.VibeCoding
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
			name:             componentVibeCoding,
			defaultNamespace: namespace,
		},
		func(ctx context.Context) ([]any, error) {
			defaults, err := getDefaultConfig(spec.PlatformSource, spec.PlatformVersion, componentVibeCoding)
			if err != nil {
				return nil, fmt.Errorf("fetch default config for %s: %w", componentVibeCoding, err)
			}

			return []any{
				d.vibeCodingOCIRepositoryObject(namespace, defaults.Version),
				d.vibeCodingKustomizationObject(namespace),
				vibeCodingRuntimeInfoConfigMap(namespace, c.Settings),
			}, nil
		},
		nil,
	)
}

// vibeCodingOCIRepositoryObject sources vibe-coding-gitops-infra directly.
// Only release artifacts signed by that repository's release workflow are
// accepted; the pull secret is a prerequisite this operator does not manage
// (see docs/SETUP.md step 6 in that repository).
func (d *reconcilerModule) vibeCodingOCIRepositoryObject(namespace, version string) map[string]any {
	if !d.crdChecker.IsAvailable(utils.OCIRepositoryResource) {
		return nil
	}

	semver := version
	if semver == "" {
		semver = "*"
	}

	return map[string]any{
		"apiVersion": utils.OCIRepositoryResource.ApiVersion,
		"kind":       utils.OCIRepositoryResource.Kind,
		"metadata": map[string]any{
			"name":      componentVibeCoding,
			"namespace": namespace,
		},
		"spec": map[string]any{
			"interval": "10m",
			"url":      vibeCodingOCIRepositoryURL,
			"ref": map[string]any{
				"semver": semver,
			},
			"secretRef": map[string]any{
				"name": vibeCodingPullSecretName,
			},
			"verify": map[string]any{
				"provider": "cosign",
				"matchOIDCIdentity": []any{
					map[string]any{
						"issuer":  `^https://token\.actions\.githubusercontent\.com$`,
						"subject": `^https://github\.com/mogenius/vibe-coding-gitops-infra/\.github/workflows/release\.yaml@refs/heads/main$`,
					},
				},
			},
		},
	}
}

// vibeCodingKustomizationObject syncs cluster/default of the OCIRepository
// above, with the flux-runtime-info ConfigMap substituted into its manifests.
func (d *reconcilerModule) vibeCodingKustomizationObject(namespace string) map[string]any {
	if !d.crdChecker.IsAvailable(utils.KustomizationResource) {
		return nil
	}

	return map[string]any{
		"apiVersion": utils.KustomizationResource.ApiVersion,
		"kind":       utils.KustomizationResource.Kind,
		"metadata": map[string]any{
			"name":      componentVibeCoding,
			"namespace": namespace,
		},
		"spec": map[string]any{
			"interval": "1m",
			"sourceRef": map[string]any{
				"kind": "OCIRepository",
				"name": componentVibeCoding,
			},
			"path":  "./cluster/default",
			"prune": true,
			"postBuild": map[string]any{
				"substituteFrom": []any{
					map[string]any{
						"kind": "ConfigMap",
						"name": vibeCodingRuntimeInfoConfigMapName,
					},
				},
			},
		},
	}
}

// vibeCodingRuntimeInfoConfigMap carries spec.vibeCoding.settings into the
// Kustomization's postBuild substitution above.
func vibeCodingRuntimeInfoConfigMap(namespace string, settings map[string]string) map[string]any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      vibeCodingRuntimeInfoConfigMapName,
			"namespace": namespace,
			"labels": map[string]any{
				"toolkit.fluxcd.io/runtime": "true",
				"reconcile.fluxcd.io/watch": "Enabled",
			},
		},
		"data": settings,
	}
}
