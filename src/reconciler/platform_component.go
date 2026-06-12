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
}

// reconcileComponent runs the shared reconcile flow for a platform component.
// buildExtraObjects is an optional callback to produce component-specific extra
// Kubernetes objects (e.g. ClusterIssuers for cert-manager). When nil, only
// extra objects from the PlatformPatch are used.
func (d *reconcilerModule) reconcileComponent(
	ctx context.Context,
	platformSpec v1alpha1.PlatformConfigSpec,
	installer gitops.GitOpsInstaller,
	op operation,
	cs componentSpec,
	buildExtraObjects func(ctx context.Context) ([]any, error),
	buildExtraValues func(ctx context.Context) (map[string]any, error),
) *ReconcileResult {
	if !cs.enabled || op == deleteOperation {
		if err := installer.UnInstall(cs.name); err != nil {
			return &ReconcileResult{Err: fmt.Errorf("failed to uninstall %s: %w", cs.name, err)}
		}
		return nil
	}

	defaultComponentConfig, err := getDefaultConfig(platformSpec.PlatformSource, platformSpec.PlatformVersion, cs.name)
	if err != nil {
		return &ReconcileResult{Err: fmt.Errorf("fetch default config for %s: %w", cs.name, err)}
	}

	chart := gitops.HelmChartReference{
		Chart:      helmChartName(cs.chart, cs.defaultChart),
		Repository: helmRepository(cs.chart, cs.defaultRepo),
		Version:    helmVersion(cs.chart, defaultComponentConfig.Version),
		Name:       helmReleaseName(cs.chart, cs.defaultName),
	}

	var patches []v1alpha1.PlatformPatch
	if len(cs.patches) > 0 {
		for _, patchRef := range cs.patches {
			patch, err := d.fetchPlatformPatch(ctx, patchRef)
			if err != nil && !apierrors.IsNotFound(err) {
				return &ReconcileResult{Err: fmt.Errorf("fetch platform patch for %s: %w", cs.name, err)}
			}
			patches = append(patches, *patch)
		}
	}

	componentValues, err := buildExtraValues(ctx)
	if err != nil {
		return &ReconcileResult{Err: fmt.Errorf("failed to create component values for %s: %w", cs.name, err)}
	}

	mergedValues, err := mergeHelmValues(defaultComponentConfig, componentValues, patches)
	if err != nil {
		return &ReconcileResult{Err: fmt.Errorf("merge helm values for %s: %w", cs.name, err)}
	}

	extraObjects, err := buildExtraObjects(ctx)
	if err != nil {
		return &ReconcileResult{Err: fmt.Errorf("build extra objects for %s: %w", cs.name, err)}
	}

	extraPatchObjects, err := extractPatchExtraObjects(patches)
	if err != nil {
		return &ReconcileResult{Err: fmt.Errorf("extract extra objects for %s: %w", cs.name, err)}
	}
	extraObjects = append(extraObjects, extraPatchObjects...)

	argoConfig, fluxConfig := getSpecificGitOpsConfig(platformSpec.GitOps)

	artifact := gitops.GitOpsArtifact{
		Namespace:    helmNamespace(cs.chart, cs.defaultNamespace),
		HelmChart:    chart,
		Values:       mergedValues,
		ExtraObjects: extraObjects,
		ArgoCD:       argoConfig,
		FluxCD:       fluxConfig,
	}

	if err := installer.Install(cs.name, artifact); err != nil {
		return &ReconcileResult{Err: fmt.Errorf("failed to install %s: %w", cs.name, err)}
	}

	return nil
}

func getSpecificGitOpsConfig(settings *v1alpha1.GitOpsConfig) (*gitops.ArgoCDSettings, *gitops.FluxCDSettings) {
	if settings == nil {
		return nil, nil
	}

	if settings.ArgoCD != nil {
		project := "mogenius"
		if settings.ArgoCD.Project != "" {
			project = settings.ArgoCD.Project
		}
		return &gitops.ArgoCDSettings{
			Project: project,
		}, nil
	}

	if settings.FluxCD != nil {
		return nil, &gitops.FluxCDSettings{}
	}

	return nil, nil
}
