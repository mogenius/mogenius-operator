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

	gitOpsStatus := buildGitOpsStatus(platformConfig.Spec)

	if engine == "" {
		d.logger.Info("no GitOps engine enabled, skipping reconciliation of GitOps components")
		if !gitOpsStatusEqual(platformConfig.Status.GitOpsStatus, gitOpsStatus) {
			if err := d.updatePlatformConfigStatus(ctx, obj.GetName(), nil, gitOpsStatus); err != nil {
				d.logger.Warn("failed to update PlatformConfig status", "name", obj.GetName(), "error", err)
			}
		}
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

	// Index existing conditions so LastTransitionTime is preserved when status hasn't changed.
	existingConditions := make(map[string]metav1.Condition, len(platformConfig.Status.Conditions))
	for _, c := range platformConfig.Status.Conditions {
		existingConditions[c.Type] = c
	}

	conditions := make([]metav1.Condition, 0, len(components))
	results := make([]ReconcileResult, 0)
	now := metav1.Now()

	for _, c := range components {
		condStatus := metav1.ConditionTrue
		reason := "Ready"
		message := "ready"
		if c.result != nil && c.result.Err != nil {
			condStatus = metav1.ConditionFalse
			reason = "ReconcileFailed"
			message = c.result.Err.Error()
		}

		lastTransition := now
		if prev, ok := existingConditions[c.name]; ok && prev.Status == condStatus {
			lastTransition = prev.LastTransitionTime
		}

		if c.result != nil {
			results = append(results, *c.result)
		}
		conditions = append(conditions, metav1.Condition{
			Type:               c.name,
			Status:             condStatus,
			ObservedGeneration: platformConfig.Generation,
			LastTransitionTime: lastTransition,
			Reason:             reason,
			Message:            message,
		})
	}

	// Only patch when status/message actually changed.
	if !conditionsEqual(platformConfig.Status.Conditions, conditions) || !gitOpsStatusEqual(platformConfig.Status.GitOpsStatus, gitOpsStatus) {
		if err := d.updatePlatformConfigStatus(ctx, obj.GetName(), conditions, gitOpsStatus); err != nil {
			d.logger.Warn("failed to update PlatformConfig status", "name", obj.GetName(), "error", err)
		}
	}

	return results
}

// conditionsEqual compares conditions ignoring LastTransitionTime and ObservedGeneration.
func conditionsEqual(current, desired []metav1.Condition) bool {
	if len(current) != len(desired) {
		return false
	}
	for i := range desired {
		if current[i].Type != desired[i].Type ||
			current[i].Status != desired[i].Status ||
			current[i].Message != desired[i].Message {
			return false
		}
	}
	return true
}

func (d *reconcilerModule) updatePlatformConfigStatus(ctx context.Context, name string, conditions []metav1.Condition, gitOpsStatus *v1alpha1.GitOpsStatus) error {
	status := map[string]any{}
	if conditions != nil {
		status["conditions"] = conditions
	}
	if gitOpsStatus != nil {
		status["gitOpsStatus"] = gitOpsStatus
	}
	patchBytes, err := json.Marshal(map[string]any{"status": status})
	if err != nil {
		return fmt.Errorf("marshal status patch: %w", err)
	}

	_, err = d.clientProvider.DynamicClient().Resource(platformConfigGVR).Patch(
		ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status",
	)
	return err
}

func buildGitOpsStatus(spec v1alpha1.PlatformConfigSpec) *v1alpha1.GitOpsStatus {
	gitOps := spec.GitOps
	if gitOps == nil {
		return nil
	}

	if gitOps.ArgoCD != nil {
		project := "mogenius"
		if gitOps.ArgoCD.Project != "" {
			project = gitOps.ArgoCD.Project
		}
		return &v1alpha1.GitOpsStatus{
			IsUserManaged:      !gitOps.ArgoCD.Enabled,
			Engine:             componentArgoCD,
			Namespace:          helmNamespace(gitOps.ArgoCD.Chart, argocdDefaultNamespace),
			ReleaseName:        helmReleaseName(gitOps.ArgoCD.Chart, "argocd"),
			DefaultProjectName: project,
		}
	}

	if gitOps.FluxCD != nil {
		return &v1alpha1.GitOpsStatus{
			IsUserManaged: !gitOps.FluxCD.Enabled,
			Engine:        componentFluxCD,
			Namespace:     helmNamespace(gitOps.FluxCD.Chart, fluxcdDefaultNamespace),
			ReleaseName:   helmReleaseName(gitOps.FluxCD.Chart, "flux-operator"),
		}
	}

	return nil
}

func gitOpsStatusEqual(current, desired *v1alpha1.GitOpsStatus) bool {
	if current == nil && desired == nil {
		return true
	}
	if current == nil || desired == nil {
		return false
	}
	return *current == *desired
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
