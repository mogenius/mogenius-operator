package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var platformPatchGVR = schema.GroupVersionResource{
	Group:    "mogenius.com",
	Version:  "v1alpha1",
	Resource: "platformpatches",
}

func (d *reconcilerModule) reconcileCertManager(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	certManager := spec.CertManager
	if certManager == nil || op == deleteOperation {
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

	var patch *v1alpha1.PlatformPatch
	if certManager.Patch != nil && certManager.Patch.Name != "" {
		patch, err = d.fetchPlatformPatch(ctx, certManager.Patch)
		if err != nil && !apierrors.IsNotFound(err) {
			return &ReconcileResult{Err: fmt.Errorf("fetch platform patch for %s: %w", componentCertManager, err)}
		}
	}

	extraObjects, err := d.buildCertManagerExtraObjects(ctx, certManager, patch)
	if err != nil {
		return &ReconcileResult{Err: fmt.Errorf("build extra objects for %s: %w", componentCertManager, err)}
	}

	mergedValues, err := mergeHelmValues(defaultComponentConfig, nil, patch)
	if err != nil {
		return &ReconcileResult{Err: fmt.Errorf("merge helm values for %s: %w", componentCertManager, err)}
	}

	artifact := gitops.GitOpsArtifact{
		Namespace:    "cert-manager",
		HelmChart:    chart,
		ExtraObjects: extraObjects,
		Values:       mergedValues,
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

// buildCertManagerExtraObjects merges ClusterIssuer objects derived from the
// issuer config with any extra objects coming from the referenced PlatformPatch.
func (d *reconcilerModule) buildCertManagerExtraObjects(ctx context.Context, certManager *v1alpha1.CertManagerConfig, patch *v1alpha1.PlatformPatch) ([]any, error) {
	extraObjects := make([]any, 0, len(certManager.Issuers))

	for _, issuer := range certManager.Issuers {
		extraObjects = append(extraObjects, buildClusterIssuerObject(issuer))
	}

	if patch == nil {
		return extraObjects, nil
	}

	for _, rawObj := range patch.Spec.ExtraObjects {
		if rawObj.Raw == nil {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal(rawObj.Raw, &obj); err != nil {
			return nil, fmt.Errorf("decode extra object from patch %q: %w", certManager.Patch.Name, err)
		}
		extraObjects = append(extraObjects, obj)
	}

	return extraObjects, nil
}

const letsEncryptProdServer = "https://acme-v02.api.letsencrypt.org/directory"

func buildClusterIssuerObject(issuer v1alpha1.CertManagerIssuerConfig) map[string]interface{} {
	server := issuer.Server
	if server == "" {
		server = letsEncryptProdServer
	}

	acme := map[string]interface{}{
		"server": server,
		"email":  issuer.Email,
		"privateKeySecretRef": map[string]interface{}{
			"name": issuer.Name + "-account-key",
		},
	}

	if issuer.HTTP01 != nil {
		ingress := map[string]interface{}{}
		if issuer.HTTP01.IngressClass != "" {
			ingress["class"] = issuer.HTTP01.IngressClass
		}
		if len(issuer.HTTP01.IngressAnnotations) > 0 {
			ingress["podTemplate"] = map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": issuer.HTTP01.IngressAnnotations,
				},
			}
		}
		acme["solvers"] = []interface{}{
			map[string]interface{}{
				"http01": map[string]interface{}{
					"ingress": ingress,
				},
			},
		}
	}

	return map[string]interface{}{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "ClusterIssuer",
		"metadata": map[string]interface{}{
			"name": issuer.Name,
		},
		"spec": map[string]interface{}{
			"acme": acme,
		},
	}
}
