package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// componentSpec holds the shared configuration needed to reconcile a platform component.
type componentSpec struct {
	enabled          bool
	chart            *v1alpha1.HelmChartReference
	patches          []v1alpha1.PlatformConfigPatchReference
	name             string // component constant, e.g. componentCertManager
	defaultNamespace string // target namespace, e.g. "cert-manager"
	defaultChart     string // default Helm chart name
	defaultRepo      string // default Helm repository URL
	defaultName      string // default Helm release name
	// argoIgnoreDifferences, when set, is written to the component's Argo CD
	// Application (spec.ignoreDifferences + RespectIgnoreDifferences). Use it for
	// a component whose controller writes back a field Argo never sends, which
	// would otherwise pin the Application at Healthy/OutOfSync. Ignored on Flux.
	argoIgnoreDifferences []any
}

// reconcileComponent runs the shared reconcile flow for a platform component.
// buildExtraObjects is an optional callback to produce component-specific extra
// Kubernetes objects (e.g. ClusterIssuers for cert-manager). When nil, only
// extra objects from the PlatformPatch are used.
//
// Only a component the spec actually declares reaches this function: its
// caller returns early when the block is absent. That is the difference
// between "not mentioned" and "turned off" — the first leaves an installed
// release alone, only the second removes it. The PlatformConfig is synced from
// git with prune enabled, so the resource disappears when the file is renamed
// or its path breaks, and treating an absent block as enabled:false would tear
// down cert-manager, Traefik, Prometheus and Loki over a rename.
func (d *reconcilerModule) reconcileComponent(
	ctx context.Context,
	platformSpec v1alpha1.PlatformConfigSpec,
	installer gitops.GitOpsInstaller,
	op operation,
	cs componentSpec,
	buildExtraObjects func(ctx context.Context) ([]any, error),
	buildExtraValues func(ctx context.Context) (map[string]any, error),
) *ReconcileResult {
	// The resource being gone is not an instruction to uninstall anything.
	// reconcilePlatformConfig already returns before it gets here; this keeps
	// the invariant local to the function that would do the damage.
	if op == deleteOperation {
		return nil
	}

	if !cs.enabled {
		if err := installer.UnInstall(cs.name); err != nil {
			return &ReconcileResult{Err: fmt.Errorf("failed to uninstall %s: %w", cs.name, err)}
		}
		return nil
	}

	artifact, result := d.buildComponentArtifact(ctx, platformSpec, cs, buildExtraObjects, buildExtraValues)
	if result != nil {
		return result
	}

	// No chart declared: the component ships entirely as extra objects, the
	// same path the GitOps engine's own chart uses for what sits beside it
	// (see ApplyExtras — the engine is installed by the Helm SDK directly, not
	// through this installer, so there is no HelmRelease for it either).
	if !hasChart(cs) {
		if err := installer.ApplyExtras(cs.name, artifact); err != nil {
			return &ReconcileResult{Err: fmt.Errorf("failed to apply %s: %w", cs.name, err)}
		}
		return nil
	}

	if err := installer.Install(cs.name, artifact); err != nil {
		return &ReconcileResult{Err: fmt.Errorf("failed to install %s: %w", cs.name, err)}
	}

	return nil
}

// hasChart reports whether cs names a Helm chart to install at all. Absent
// one (see reconcileVibeCoding), there is nothing to fetch default config or
// values for — the component is delivered purely through buildExtraObjects.
func hasChart(cs componentSpec) bool {
	return cs.defaultChart != "" || (cs.chart != nil && cs.chart.Chart != "")
}

// buildComponentArtifact resolves a component's chart, values and extra objects
// into the artifact an installer ships. Shared with the engine path, which
// needs the same resolution but installs the chart itself with the Helm SDK.
//
// A non-nil ReconcileResult is the failure; the artifact is only valid when it
// is nil.
func (d *reconcilerModule) buildComponentArtifact(
	ctx context.Context,
	platformSpec v1alpha1.PlatformConfigSpec,
	cs componentSpec,
	buildExtraObjects func(ctx context.Context) ([]any, error),
	buildExtraValues func(ctx context.Context) (map[string]any, error),
) (gitops.GitOpsArtifact, *ReconcileResult) {
	// No chart to fetch a default config or resolve a name/repo/version for
	// (see reconcileVibeCoding): the component is extra objects only.
	var defaultComponentConfig componentDefaultSpec
	var chart gitops.HelmChartReference
	if hasChart(cs) {
		var err error
		defaultComponentConfig, err = getDefaultConfig(platformSpec.PlatformSource, platformSpec.PlatformVersion, cs.name)
		if err != nil {
			return gitops.GitOpsArtifact{}, &ReconcileResult{Err: fmt.Errorf("fetch default config for %s: %w", cs.name, err)}
		}

		chart = gitops.HelmChartReference{
			Chart:      helmChartName(cs.chart, cs.defaultChart),
			Repository: helmRepository(cs.chart, cs.defaultRepo),
			Version:    helmVersion(cs.chart, defaultComponentConfig.Version),
			Name:       helmReleaseName(cs.chart, cs.defaultName),
		}
	}

	var patches []v1alpha1.PlatformPatch
	if len(cs.patches) > 0 {
		for _, patchRef := range cs.patches {
			patch, err := d.fetchPlatformPatch(ctx, patchRef)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					return gitops.GitOpsArtifact{}, &ReconcileResult{Err: fmt.Errorf("fetch platform patch for %s: %w", cs.name, err)}
				}
				continue
			}
			patches = append(patches, *patch)
		}
	}

	componentValues, err := buildExtraValues(ctx)
	if err != nil {
		return gitops.GitOpsArtifact{}, &ReconcileResult{Err: fmt.Errorf("failed to create component values for %s: %w", cs.name, err)}
	}

	mergedValues, err := mergeHelmValues(defaultComponentConfig, componentValues, patches)
	if err != nil {
		return gitops.GitOpsArtifact{}, &ReconcileResult{Err: fmt.Errorf("merge helm values for %s: %w", cs.name, err)}
	}

	extraObjects, err := buildExtraObjects(ctx)
	if err != nil {
		return gitops.GitOpsArtifact{}, &ReconcileResult{Err: fmt.Errorf("build extra objects for %s: %w", cs.name, err)}
	}

	extraPatchObjects, err := extractPatchExtraObjects(patches)
	if err != nil {
		return gitops.GitOpsArtifact{}, &ReconcileResult{Err: fmt.Errorf("extract extra objects for %s: %w", cs.name, err)}
	}
	extraObjects = append(extraObjects, extraPatchObjects...)

	argoConfig, fluxConfig := resolveComponentGitOpsConfig(platformSpec.GitOps, cs.argoIgnoreDifferences)

	return gitops.GitOpsArtifact{
		Namespace:    helmNamespace(cs.chart, cs.defaultNamespace),
		HelmChart:    chart,
		Values:       mergedValues,
		ExtraObjects: extraObjects,
		ArgoCD:       argoConfig,
		FluxCD:       fluxConfig,
	}, nil
}

func getSpecificGitOpsConfig(settings *v1alpha1.GitOpsConfig) (*gitops.ArgoCDSettings, *gitops.FluxCDSettings) {
	if settings == nil {
		return nil, nil
	}

	if settings.ArgoCD != nil {
		return &gitops.ArgoCDSettings{
			Project: argoProjectName(settings),
		}, nil
	}

	if settings.FluxCD != nil {
		return nil, &gitops.FluxCDSettings{}
	}

	return nil, nil
}

// resolveComponentGitOpsConfig picks a component's Argo/Flux settings and
// carries its Argo ignore-differences onto the Argo settings.
//
// The ignore is intrinsic to the component — a controller writing back a field
// Argo never sends (the agent-sandbox controller stamping a ref-hash label on
// its SandboxTemplate), which otherwise sits Healthy/OutOfSync and selfHeals
// forever (MOG-4827). getSpecificGitOpsConfig only returns an argoConfig when
// spec.gitOps.argocd is explicitly declared, but Argo is just as often only
// detected (user-managed), leaving argoConfig nil and the ignore silently
// dropped — so the Application still selfHeals and its PlatformConfig component
// never reports ready (MOG-4819). Attach the ignore whenever Argo is not ruled
// out by an explicit Flux block; buildArgoApplication (the only consumer, run
// solely for Argo delivery) emits it. The synthesized settings leave Project
// empty so getArgoProject keeps its "default" fallback unchanged.
func resolveComponentGitOpsConfig(settings *v1alpha1.GitOpsConfig, argoIgnoreDifferences []any) (*gitops.ArgoCDSettings, *gitops.FluxCDSettings) {
	argoConfig, fluxConfig := getSpecificGitOpsConfig(settings)
	if len(argoIgnoreDifferences) > 0 && fluxConfig == nil {
		if argoConfig == nil {
			argoConfig = &gitops.ArgoCDSettings{}
		}
		argoConfig.IgnoreDifferences = argoIgnoreDifferences
	}
	return argoConfig, fluxConfig
}
