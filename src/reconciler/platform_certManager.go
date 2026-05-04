package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
)

func (d *reconcilerModule) reconcileCertManager(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	cm := spec.CertManager
	if cm == nil {
		cm = &v1alpha1.CertManagerConfig{}
	}
	return d.reconcileComponent(ctx, spec, installer, op,
		componentSpec{
			enabled:      cm.Enabled,
			chart:        cm.Chart,
			patch:        cm.Patch,
			name:         componentCertManager,
			namespace:    "cert-manager",
			defaultChart: "cert-manager",
			defaultRepo:  "https://charts.jetstack.io",
			defaultName:  "cert-manager",
		},
		func(ctx context.Context, patch *v1alpha1.PlatformPatch) ([]any, error) {
			return d.buildCertManagerExtraObjects(ctx, cm, patch)
		},
		func(ctx context.Context) (map[string]interface{}, error) {
			return nil, nil
		},
	)
}

// buildCertManagerExtraObjects merges ClusterIssuer objects derived from the
// issuer config with any extra objects coming from the referenced PlatformPatch.
func (d *reconcilerModule) buildCertManagerExtraObjects(ctx context.Context, certManager *v1alpha1.CertManagerConfig, patch *v1alpha1.PlatformPatch) ([]any, error) {
	extraObjects := make([]any, 0, len(certManager.Issuers))
	for _, issuer := range certManager.Issuers {
		extraObjects = append(extraObjects, buildClusterIssuerObject(issuer))
	}

	patchObjects, err := extractPatchExtraObjects(patch)
	if err != nil {
		return nil, fmt.Errorf("decode extra object from patch %q: %w", certManager.Patch.Name, err)
	}
	return append(extraObjects, patchObjects...), nil
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
