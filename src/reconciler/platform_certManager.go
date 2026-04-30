package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
)

func (d *reconcilerModule) reconcileCertManager(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller) *ReconcileResult {
	const componentName = "cert-manager"

	certManager := spec.CertManager
	if certManager == nil {
		if err := installer.UnInstall(componentName); err != nil {
			return &ReconcileResult{Err: fmt.Errorf("failed to uninstall %s: %w", componentName, err)}
		}
		return nil
	}

	defaultComponentConfig := getDefaultConfig("certManager", spec.PlatformVersion, spec.PlatformSource)
	chart := gitops.HelmChartReference{
		Chart:      helmChartName(certManager.Chart, "cert-manager"),
		Repository: helmRepository(certManager.Chart, "https://charts.jetstack.io"),
		Version:    helmVersion(certManager.Chart, defaultComponentConfig.Version),
		Name:       helmReleaseName(certManager.Chart, "cert-manager"),
	}

	artifact := gitops.GitOpsArtifact{
		Namespace: "cert-manager",
		HelmChart: chart,
	}

	if certManager.Enabled {
		if err := installer.Install(componentName, artifact); err != nil {
			return &ReconcileResult{Err: fmt.Errorf("failed to install %s: %w", componentName, err)}
		}
	} else {
		if err := installer.UnInstall(componentName); err != nil {
			return &ReconcileResult{Err: fmt.Errorf("failed to uninstall %s: %w", componentName, err)}
		}
	}

	return nil
}
