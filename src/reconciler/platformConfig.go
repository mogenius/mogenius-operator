package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"reflect"

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
	// Not a Helm chart like the others: the condition reports whether the
	// objects that make the engine sync spec.gitOps.repositories are in place.
	componentPlatformRepositories = "platform-repositories"
)

// GitOps engine identities as reported in status.gitOpsStatus.engine. These are
// a different concept from the component* constants above, which name the
// platform-defaults file of the engine's Helm chart.
const (
	gitOpsEngineArgoCD = gitops.EngineArgoCD
	gitOpsEngineFlux   = gitops.EngineFlux
)

const (
	argocdDefaultNamespace = "argocd"
	fluxcdDefaultNamespace = "flux-system"
)

// Where the reported GitOps information originates.
const (
	gitOpsSourceSpec     = "spec"
	gitOpsSourceDetected = "detected"
)

// argoCDDefaultProject is the AppProject mogenius creates alongside its own
// Argo CD install. argoCDFallbackProject is what the platform uses instead when
// the engine belongs to the user: "default" ships with every Argo CD, while
// "mogenius" would only exist where mogenius installed the engine itself.
const (
	argoCDDefaultProject  = "mogenius"
	argoCDFallbackProject = "default"
)

// argoProjectName is the AppProject the platform's Applications are created in.
//
// An explicit spec.gitOps.argocd.project is always honoured — it is the user's
// declaration, and keeping it existing is then their business. Without one the
// answer depends on who owns the engine, because the platform can only rely on
// a project it created itself.
func argoProjectName(gitOps *v1alpha1.GitOpsConfig) string {
	if gitOps == nil || gitOps.ArgoCD == nil {
		return argoCDFallbackProject
	}
	if gitOps.ArgoCD.Project != "" {
		return gitOps.ArgoCD.Project
	}
	if gitOps.ArgoCD.Enabled {
		return argoCDDefaultProject
	}
	return argoCDFallbackProject
}

func (d *reconcilerModule) reconcilePlatformConfig(ctx context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	// A deleted PlatformConfig is not an instruction to tear the platform down.
	// The resource is synced from git with prune enabled, so it disappears when
	// platformconfig.yaml is renamed, its path breaks or a bad commit lands —
	// and uninstalling every component over that would take cert-manager,
	// Traefik, Prometheus and Loki with it. Removal is expressed as
	// enabled:false in the spec, which the component reconcilers act on.
	//
	// There is also nothing left to patch a status onto, so returning here
	// avoids a guaranteed-failing status update on a resource that is gone.
	if op == deleteOperation {
		d.logger.Info("PlatformConfig deleted, leaving installed components untouched", "name", obj.GetName())
		return nil
	}

	var platformConfig v1alpha1.PlatformConfig
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &platformConfig); err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("failed to parse PlatformConfig: %w", err)}}
	}

	gitOpsStatus := buildGitOpsStatus(platformConfig.Spec, d.detectGitOpsStatus(ctx))

	// specEngine is the engine mogenius is asked to install; it is empty unless
	// spec.gitOps enables one.
	specEngine, specEngineNs, err := inferGitOpsEngine(platformConfig.Spec.GitOps)
	if err != nil {
		d.patchGitOpsStatus(ctx, obj.GetName(), platformConfig.Status.GitOpsStatus, gitOpsStatus)
		return []ReconcileResult{{Err: err}}
	}

	engine, engineNs := deliveryEngine(specEngine, specEngineNs, gitOpsStatus)

	// No engine anywhere: components ship as Applications/HelmReleases, so there
	// is nothing to deliver them with. The status still gets reported.
	if engine == "" {
		d.logger.Info("skipping reconciliation of platform components, reporting GitOps status only",
			"name", obj.GetName(), "specEngine", specEngine, "engine", gitOpsStatus.Engine, "installed", gitOpsStatus.Installed)
		d.patchGitOpsStatus(ctx, obj.GetName(), platformConfig.Status.GitOpsStatus, gitOpsStatus)
		return nil
	}

	ownerRef := metav1.OwnerReference{
		APIVersion: "mogenius.com/v1alpha1",
		Kind:       "PlatformConfig",
		Name:       platformConfig.Name,
		UID:        platformConfig.UID,
	}
	installer := gitops.NewGitOpsInstaller(engine, engineNs, d.clientProvider, []metav1.OwnerReference{ownerRef}, d.logger)

	type componentResult struct {
		name   string
		result *ReconcileResult
	}

	// Capacity: the eight non-engine components plus the engine, when mogenius
	// owns it. The engine is only reconciled in that case — reconciling a
	// user-managed engine would adopt a Helm release someone else installed and
	// overwrite their values on the next sweep.
	components := make([]componentResult, 0, 9)
	switch specEngine {
	case gitOpsEngineArgoCD:
		components = append(components, componentResult{name: componentArgoCD, result: d.reconcileArgoCD(ctx, platformConfig.Spec, installer, op)})
	case gitOpsEngineFlux:
		components = append(components, componentResult{name: componentFluxCD, result: d.reconcileFluxCD(ctx, platformConfig.Spec, installer, op)})
	}

	// Repositories the platform syncs itself. Only when mogenius does not own
	// the engine: when it does, reconcileArgoCD/reconcileFluxCD ship the same
	// objects as extra objects of the release they install.
	if specEngine == "" {
		components = append(components, componentResult{
			name:   componentPlatformRepositories,
			result: d.reconcilePlatformRepositories(ctx, platformConfig.Spec, engine, engineNs),
		})
	}

	components = append(components,
		componentResult{componentExternalSecretsOperator, d.reconcileExternalSecretsOperator(ctx, platformConfig.Spec, installer, op)},
		componentResult{componentCertManager, d.reconcileCertManager(ctx, platformConfig.Spec, installer, op)},
		componentResult{componentTraefik, d.reconcileTraefik(ctx, platformConfig.Spec, installer, op)},
		componentResult{componentExternalDNS, d.reconcileExternalDNS(ctx, platformConfig.Spec, installer, op)},
		componentResult{componentKubePrometheusStack, d.reconcileKubePrometheusStack(ctx, platformConfig.Spec, installer, op)},
		componentResult{componentLoki, d.reconcileLoki(ctx, platformConfig.Spec, installer, op)},
		componentResult{componentAlloy, d.reconcileAlloy(ctx, platformConfig.Spec, installer, op)},
		componentResult{componentRenovateOperator, d.reconcileRenovateOperator(ctx, platformConfig.Spec, installer, op)},
	)

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

// patchGitOpsStatus writes the GitOps status when it differs from what the
// object already reports. Failures are logged, never propagated.
func (d *reconcilerModule) patchGitOpsStatus(ctx context.Context, name string, current, desired *v1alpha1.GitOpsStatus) {
	if gitOpsStatusEqual(current, desired) {
		return
	}
	if err := d.updatePlatformConfigStatus(ctx, name, nil, desired); err != nil {
		d.logger.Warn("failed to update PlatformConfig status", "name", name, "error", err)
	}
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

// buildGitOpsStatus merges what the spec declares with what was found on the
// cluster. An engine the spec enables is authoritative for the identity, while
// detection always contributes the live facts (installed, version, controllers).
// The result is never nil: an empty status with installed=false is what tells a
// consumer that the cluster was checked and carries no engine.
func buildGitOpsStatus(spec v1alpha1.PlatformConfigSpec, detection gitOpsDetection) *v1alpha1.GitOpsStatus {
	engine, enabled, chart, project := specGitOps(spec.GitOps)
	detected := detection.forEngine(engine)
	source := gitOpsSourceSpec

	// Without an engine that mogenius owns, whatever actually runs wins.
	if !enabled {
		if fallback := detection.preferred(); fallback != nil {
			if fallback.engine != engine {
				engine, chart, project = fallback.engine, nil, ""
			}
			detected = fallback
			source = gitOpsSourceDetected
		}
	}

	if engine == "" {
		return &v1alpha1.GitOpsStatus{Source: gitOpsSourceDetected}
	}

	status := &v1alpha1.GitOpsStatus{
		Engine:        engine,
		IsUserManaged: !enabled,
		Source:        source,
		Namespace:     helmNamespace(chart, ""),
		ReleaseName:   helmReleaseName(chart, ""),
	}
	if engine == gitOpsEngineArgoCD {
		// The same project getSpecificGitOpsConfig hands to the installer: the API
		// places its own Applications by this value, so a name that only exists on
		// a mogenius-installed engine would break them on a user-managed one.
		status.DefaultProjectName = firstNonEmpty(project, argoProjectName(spec.GitOps))
	}

	if detected != nil {
		status.Installed = detected.installed
		status.Version = detected.version
		status.Controllers = detected.controllers
		status.Namespace = firstNonEmpty(status.Namespace, detected.namespace)
		status.ReleaseName = firstNonEmpty(status.ReleaseName, detected.releaseName)
	}

	status.Namespace = firstNonEmpty(status.Namespace, defaultEngineNamespace(engine))
	if source == gitOpsSourceSpec {
		// A user-managed install can carry any release name, so only a
		// mogenius-declared engine falls back to the name we would install it under.
		status.ReleaseName = firstNonEmpty(status.ReleaseName, defaultEngineReleaseName(engine))
	}

	return status
}

// specGitOps reports the engine spec.gitOps declares, whether mogenius installs
// it, and the chart/project settings that belong to it.
func specGitOps(gitOps *v1alpha1.GitOpsConfig) (engine string, enabled bool, chart *v1alpha1.HelmChartReference, project string) {
	if gitOps == nil {
		return "", false, nil, ""
	}
	switch {
	case gitOps.ArgoCD != nil && gitOps.ArgoCD.Enabled:
		return gitOpsEngineArgoCD, true, gitOps.ArgoCD.Chart, gitOps.ArgoCD.Project
	case gitOps.FluxCD != nil && gitOps.FluxCD.Enabled:
		return gitOpsEngineFlux, true, gitOps.FluxCD.Chart, ""
	case gitOps.ArgoCD != nil:
		return gitOpsEngineArgoCD, false, gitOps.ArgoCD.Chart, gitOps.ArgoCD.Project
	case gitOps.FluxCD != nil:
		return gitOpsEngineFlux, false, gitOps.FluxCD.Chart, ""
	}
	return "", false, nil, ""
}

func defaultEngineNamespace(engine string) string {
	switch engine {
	case gitOpsEngineArgoCD:
		return argocdDefaultNamespace
	case gitOpsEngineFlux:
		return fluxcdDefaultNamespace
	}
	return ""
}

func defaultEngineReleaseName(engine string) string {
	switch engine {
	case gitOpsEngineArgoCD:
		return "argocd"
	case gitOpsEngineFlux:
		return "flux-operator"
	}
	return ""
}

func gitOpsStatusEqual(current, desired *v1alpha1.GitOpsStatus) bool {
	return reflect.DeepEqual(current, desired)
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
		return gitOpsEngineArgoCD, helmNamespace(gitOps.ArgoCD.Chart, argocdDefaultNamespace), nil
	}
	if fluxCDEnabled {
		return gitOpsEngineFlux, helmNamespace(gitOps.FluxCD.Chart, fluxcdDefaultNamespace), nil
	}
	return "", "", nil
}

// deliveryEngine picks the engine the platform components are delivered
// through. That is a different question from who installs the engine: during
// onboarding Helm brings Argo CD or Flux and spec.gitOps leaves it disabled, so
// without falling back to the detected engine those clusters would get no
// components at all — the spec engine is empty and everything ships as an
// Application or HelmRelease.
//
// An empty engine means there is nothing to deliver with.
func deliveryEngine(specEngine, specEngineNamespace string, status *v1alpha1.GitOpsStatus) (engine, namespace string) {
	if specEngine != "" {
		return specEngine, specEngineNamespace
	}
	// Only a running engine counts. A declared-but-absent one has no namespace
	// to create objects in, and they would be rejected outright.
	if status != nil && status.Installed {
		return status.Engine, status.Namespace
	}
	return "", ""
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
