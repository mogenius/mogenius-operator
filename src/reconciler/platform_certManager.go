package reconciler

import (
	"context"
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
			enabled:          cm.Enabled,
			chart:            cm.Chart,
			patches:          cm.Patches,
			name:             componentCertManager,
			defaultChart:     "cert-manager",
			defaultRepo:      "https://charts.jetstack.io",
			defaultName:      "cert-manager",
			defaultNamespace: "cert-manager",
		},
		func(ctx context.Context) ([]any, error) {
			return d.buildCertManagerExtraObjects(ctx, cm)
		},
		func(ctx context.Context) (map[string]any, error) {
			return nil, nil
		},
	)
}

// buildCertManagerExtraObjects merges ClusterIssuer objects derived from the
// issuer config with any extra objects coming from the referenced PlatformPatch.
func (d *reconcilerModule) buildCertManagerExtraObjects(ctx context.Context, certManager *v1alpha1.CertManagerConfig) ([]any, error) {
	extraObjects := make([]any, 0, len(certManager.Issuers))
	for _, issuer := range certManager.Issuers {
		extraObjects = append(extraObjects, buildClusterIssuerObject(issuer))
	}

	return extraObjects, nil
}

const letsEncryptProdServer = "https://acme-v02.api.letsencrypt.org/directory"

func buildClusterIssuerObject(issuer v1alpha1.CertManagerIssuerConfig) map[string]any {
	server := issuer.Server
	if server == "" {
		server = letsEncryptProdServer
	}

	acme := map[string]any{
		"server": server,
		"email":  issuer.Email,
		"privateKeySecretRef": map[string]any{
			"name": issuer.Name + "-account-key",
		},
	}

	if issuer.HTTP01 != nil {
		ingress := map[string]any{}
		if issuer.HTTP01.IngressClass != "" {
			ingress["class"] = issuer.HTTP01.IngressClass
		}
		if len(issuer.HTTP01.IngressAnnotations) > 0 {
			ingress["podTemplate"] = map[string]any{
				"metadata": map[string]any{
					"annotations": issuer.HTTP01.IngressAnnotations,
				},
			}
		}
		acme["solvers"] = []any{
			map[string]any{
				"http01": map[string]any{
					"ingress": ingress,
				},
			},
		}
	}

	return map[string]any{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "ClusterIssuer",
		"metadata": map[string]any{
			"name": issuer.Name,
		},
		"spec": map[string]any{
			"acme": acme,
		},
	}
}
