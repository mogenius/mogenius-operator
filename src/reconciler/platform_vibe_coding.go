package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"mogenius-operator/src/utils"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/yaml"
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
	// vibeCodingKustomizationName is not componentVibeCoding: "vibe-coding" is
	// the name of a layer Kustomization inside cluster/default, in the same
	// namespace. A root of that name would be overwritten by its own layer.
	vibeCodingKustomizationName = "vibe-coding-root"
)

// vibeCodingKeep keeps an object when the moac HelmRelease carrying it is
// uninstalled, e.g. by garbage collection after the PlatformConfig is deleted.
var vibeCodingKeep = map[string]any{"helm.sh/resource-policy": "keep"}

// reconcileVibeCoding goes through reconcileComponent like every other
// component. It declares no chart (see hasChart/buildComponentArtifact):
// vibe-coding-gitops-infra is a plain Kustomize tree, not a Helm chart, so
// there is no HelmRelease to template it into. Its OCIRepository,
// Kustomization and the flux-runtime-info ConfigMap all ship as extra
// objects instead, through installer.ApplyExtras — the same path the GitOps
// engine's own chart uses for what sits beside it.
//
// Unlike every other component, enabled:false never uninstalls: the
// Kustomization owns every app, database and generated secret of the
// installation, so it is suspended instead and keeps the cluster on its last
// release. Uninstalling vibeCoding is a manual runbook.
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
			// Always applied, see above: disabled suspends.
			enabled:          true,
			name:             componentVibeCoding,
			defaultNamespace: namespace,
		},
		func(ctx context.Context) ([]any, error) {
			version := c.Version
			if version == "" {
				defaults, err := getDefaultConfig(spec.PlatformSource, spec.PlatformVersion, componentVibeCoding)
				if err != nil {
					return nil, fmt.Errorf("fetch default config for %s: %w", componentVibeCoding, err)
				}
				version = defaults.Version
			}

			objects := []any{
				d.vibeCodingOCIRepositoryObject(namespace, version),
				d.vibeCodingKustomizationObject(namespace, !c.Enabled),
				vibeCodingRuntimeInfoConfigMap(namespace, c.Settings),
			}

			patchObjects, err := d.vibeCodingPatchObjects(ctx, namespace, c.Patches)
			if err != nil {
				return nil, err
			}

			return append(objects, patchObjects...), nil
		},
		nil,
	)
}

// vibeCodingPatchObjects turns each VibeCodingPatchReference into the objects
// that customize its target: a ConfigMap carrying the referenced
// PlatformPatch's ValuesObject as "<target>-custom-values" for that resource
// in vibe-coding-gitops-infra to read, plus the patch's own ExtraObjects
// verbatim -- there is no chart here for either to merge into, unlike every
// other component's Patches (see mergeHelmValues).
func (d *reconcilerModule) vibeCodingPatchObjects(ctx context.Context, namespace string, refs []v1alpha1.VibeCodingPatchReference) ([]any, error) {
	var objects []any
	for _, ref := range refs {
		patch, err := d.fetchPlatformPatch(ctx, v1alpha1.PlatformConfigPatchReference{Name: ref.Name})
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("fetch platform patch %q for vibeCoding target %q: %w", ref.Name, ref.Target, err)
		}

		if patch.Spec.ValuesObject != nil {
			configMap, err := vibeCodingCustomValuesConfigMap(ref.Target, namespace, patch.Spec.ValuesObject.Raw)
			if err != nil {
				return nil, fmt.Errorf("build custom values configmap for vibeCoding target %q: %w", ref.Target, err)
			}
			objects = append(objects, configMap)
		}

		extraObjects, err := extractPatchExtraObjects([]v1alpha1.PlatformPatch{*patch})
		if err != nil {
			return nil, fmt.Errorf("extract extra objects for vibeCoding target %q: %w", ref.Target, err)
		}
		objects = append(objects, extraObjects...)
	}
	return objects, nil
}

// vibeCodingCustomValuesConfigMap carries a patch's ValuesObject into
// vibe-coding-gitops-infra as YAML, the same shape a Helm values.yaml takes,
// since its manifests have no chart to merge a values map into directly.
func vibeCodingCustomValuesConfigMap(target, namespace string, rawValues []byte) (map[string]any, error) {
	values := map[string]any{}
	if len(rawValues) > 0 {
		if err := json.Unmarshal(rawValues, &values); err != nil {
			return nil, fmt.Errorf("parse values: %w", err)
		}
	}

	valuesYAML, err := yaml.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("marshal values: %w", err)
	}

	return map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      target + "-custom-values",
			"namespace": namespace,
		},
		"data": map[string]any{
			"values.yaml": string(valuesYAML),
		},
	}, nil
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
			"name":        componentVibeCoding,
			"namespace":   namespace,
			"annotations": vibeCodingKeep,
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
// It is never removed (see reconcileVibeCoding), and if it is deleted anyway,
// deletionPolicy Orphan leaves the installation running instead of pruning it.
func (d *reconcilerModule) vibeCodingKustomizationObject(namespace string, suspend bool) map[string]any {
	if !d.crdChecker.IsAvailable(utils.KustomizationResource) {
		return nil
	}

	return map[string]any{
		"apiVersion": utils.KustomizationResource.ApiVersion,
		"kind":       utils.KustomizationResource.Kind,
		"metadata": map[string]any{
			"name":        vibeCodingKustomizationName,
			"namespace":   namespace,
			"annotations": vibeCodingKeep,
		},
		"spec": map[string]any{
			"interval": "1m",
			"suspend":  suspend,
			"sourceRef": map[string]any{
				"kind": "OCIRepository",
				"name": componentVibeCoding,
			},
			"path":           "./cluster/default",
			"prune":          true,
			"deletionPolicy": "Orphan",
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
			"name":        vibeCodingRuntimeInfoConfigMapName,
			"namespace":   namespace,
			"annotations": vibeCodingKeep,
			"labels": map[string]any{
				"toolkit.fluxcd.io/runtime": "true",
				"reconcile.fluxcd.io/watch": "Enabled",
			},
		},
		"data": settings,
	}
}
