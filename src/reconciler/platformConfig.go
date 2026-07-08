package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

var platformConfigGVR = schema.GroupVersionResource{
	Group:    "mogenius.com",
	Version:  "v1alpha1",
	Resource: "platformconfigs",
}

var platformPatchGVR = schema.GroupVersionResource{
	Group:    "mogenius.com",
	Version:  "v1alpha1",
	Resource: "platformpatches",
}

const (
	componentCertManager             = "cert-manager"
	componentTraefik                 = "traefik"
	componentArgoCD                  = "argocd"
	componentFluxCD                  = "flux-operator"
	componentExternalDNS             = "external-dns"
	componentKubePrometheusStack     = "kube-prometheus-stack"
	componentLoki                    = "loki"
	componentAlloy                   = "alloy"
	componentRenovateOperator        = "renovate-operator"
	componentExternalSecretsOperator = "external-secrets-operator"
)

const (
	argocdDefaultNamespace = "argocd"
	fluxcdDefaultNamespace = "flux-system"
)

func (d *reconcilerModule) reconcilePlatformConfig(ctx context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	var platformConfig v1alpha1.PlatformConfig
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &platformConfig); err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("failed to parse PlatformConfig: %w", err)}}
	}

	engine, engineNs, err := inferGitOpsEngine(platformConfig.Spec.GitOps)
	if err != nil {
		return []ReconcileResult{{Err: err}}
	}
	if engine == "" {
		d.logger.Info("no GitOps engine enabled, skipping reconciliation of GitOps components")
		return []ReconcileResult{{Err: fmt.Errorf("no GitOps engine enabled")}}
	}

	ownerRef := metav1.OwnerReference{
		APIVersion: "mogenius.com/v1alpha1",
		Kind:       "PlatformConfig",
		Name:       platformConfig.Name,
		UID:        platformConfig.UID,
	}
	installer := gitops.NewGitOpsInstaller(engine, engineNs, d.clientProvider, []metav1.OwnerReference{ownerRef})

	type componentResult struct {
		name   string
		result *ReconcileResult
	}

	var gitopsResult componentResult
	switch engine {
	case "argocd":
		gitopsResult = componentResult{name: componentArgoCD, result: d.reconcileArgoCD(ctx, platformConfig.Spec, installer, op)}
	case "flux":
		gitopsResult = componentResult{name: componentFluxCD, result: d.reconcileFluxCD(ctx, platformConfig.Spec, installer, op)}
	}

	components := []componentResult{
		gitopsResult,
		{componentExternalSecretsOperator, d.reconcileExternalSecretsOperator(ctx, platformConfig.Spec, installer, op)},
		{componentCertManager, d.reconcileCertManager(ctx, platformConfig.Spec, installer, op)},
		{componentTraefik, d.reconcileTraefik(ctx, platformConfig.Spec, installer, op)},
		{componentExternalDNS, d.reconcileExternalDNS(ctx, platformConfig.Spec, installer, op)},
		{componentKubePrometheusStack, d.reconcileKubePrometheusStack(ctx, platformConfig.Spec, installer, op)},
		{componentLoki, d.reconcileLoki(ctx, platformConfig.Spec, installer, op)},
		{componentAlloy, d.reconcileAlloy(ctx, platformConfig.Spec, installer, op)},
		{componentRenovateOperator, d.reconcileRenovateOperator(ctx, platformConfig.Spec, installer, op)},
	}

	statuses := make([]v1alpha1.PlatformComponentStatus, 0, len(components))
	results := make([]ReconcileResult, 0)
	now := metav1.Now()

	for _, c := range components {
		ready := c.result == nil || (c.result != nil && c.result.Err == nil)
		message := "ready"
		if c.result != nil && c.result.Err != nil {
			message = c.result.Err.Error()
		}

		status := v1alpha1.PlatformComponentStatus{
			Name:     c.name,
			Ready:    ready,
			LastSync: now,
			Message:  message,
		}
		if c.result != nil {
			results = append(results, *c.result)
		}
		statuses = append(statuses, status)
	}

	// Only patch when Ready/Message actually changed. LastSync alone would
	// otherwise produce a new resourceVersion on every reconcile.
	if !componentStatusesEqual(platformConfig.Status.Components, statuses) {
		if err := d.updatePlatformConfigStatus(ctx, obj.GetName(), statuses); err != nil {
			d.logger.Warn("failed to update PlatformConfig status", "name", obj.GetName(), "error", err)
		}
	}

	return results
}

// componentStatusesEqual compares component statuses ignoring LastSync.
func componentStatusesEqual(current, desired []v1alpha1.PlatformComponentStatus) bool {
	if len(current) != len(desired) {
		return false
	}
	for i := range desired {
		if current[i].Name != desired[i].Name ||
			current[i].Ready != desired[i].Ready ||
			current[i].Message != desired[i].Message {
			return false
		}
	}
	return true
}

func (d *reconcilerModule) updatePlatformConfigStatus(ctx context.Context, name string, components []v1alpha1.PlatformComponentStatus) error {
	patch := map[string]any{
		"status": map[string]any{
			"components": components,
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal status patch: %w", err)
	}

	_, err = d.clientProvider.DynamicClient().Resource(platformConfigGVR).Patch(
		ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status",
	)
	return err
}

func (d *reconcilerModule) fetchPlatformPatch(ctx context.Context, ref v1alpha1.PlatformConfigPatchReference) (*v1alpha1.PlatformPatch, error) {
	obj, err := d.clientProvider.DynamicClient().Resource(platformPatchGVR).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	var patch v1alpha1.PlatformPatch
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &patch); err != nil {
		return nil, fmt.Errorf("convert PlatformPatch: %w", err)
	}
	return &patch, nil
}

// inferGitOpsEngine inspects the GitOpsConfig and returns the engine name and the
// namespace where that engine is installed. Exactly one engine may be enabled;
// having both enabled is a configuration error.
func inferGitOpsEngine(gitOps *v1alpha1.GitOpsConfig) (engine, namespace string, err error) {
	if gitOps == nil {
		return "", "", nil
	}
	argoCDEnabled := gitOps.ArgoCD != nil && gitOps.ArgoCD.Enabled
	fluxCDEnabled := gitOps.FluxCD != nil && gitOps.FluxCD.Enabled

	if argoCDEnabled && fluxCDEnabled {
		return "", "", fmt.Errorf("invalid gitops config: argocd and fluxcd cannot both be enabled")
	}
	if argoCDEnabled {
		return "argocd", helmNamespace(gitOps.ArgoCD.Chart, argocdDefaultNamespace), nil
	}
	if fluxCDEnabled {
		return "flux", helmNamespace(gitOps.FluxCD.Chart, fluxcdDefaultNamespace), nil
	}
	return "", "", nil
}

// extractPatchExtraObjects decodes the raw ExtraObjects from a PlatformPatch into
// a slice of map[string]interface{} suitable for GitOpsArtifact.ExtraObjects.
func extractPatchExtraObjects(patches []v1alpha1.PlatformPatch) ([]any, error) {
	if len(patches) == 0 {
		return nil, nil
	}
	objects := make([]any, 0)
	for _, patch := range patches {
		for _, rawObj := range patch.Spec.ExtraObjects {
			if rawObj.Raw == nil {
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal(rawObj.Raw, &obj); err != nil {
				return nil, err
			}
			objects = append(objects, obj)
		}
	}
	return objects, nil
}
