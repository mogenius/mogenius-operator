package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
)

func (d *reconcilerModule) reconcileCertManager(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller) *ReconcileResult {

	certManager := spec.CertManager
	if certManager == nil {
		if err := installer.UnInstall(componentCertManager); err != nil {
			return &ReconcileResult{Err: fmt.Errorf("failed to uninstall %s: %w", componentCertManager, err)}
		}
		return nil
	}

	defaultComponentConfig, err := getDefaultConfig(spec.PlatformSource, spec.PlatformVersion, componentCertManager)
	if err != nil {
		return &ReconcileResult{Err: fmt.Errorf("fetch default config for %s: %w", componentCertManager, err)}
	}

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
		if err := installer.Install(componentCertManager, artifact); err != nil {
			return &ReconcileResult{Err: fmt.Errorf("failed to install %s: %w", componentCertManager, err)}
		}
	} else {
		if err := installer.UnInstall(componentCertManager); err != nil {
			return &ReconcileResult{Err: fmt.Errorf("failed to uninstall %s: %w", componentCertManager, err)}
		}
	}

	return nil
}
