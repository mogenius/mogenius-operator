package reconciler

import (
	"context"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"mogenius-operator/src/utils"

	"k8s.io/apimachinery/pkg/runtime"
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
			extraObjects := []any{}

			for _, issuer := range spec.CertManager.Issuers {
				extraObjects = append(extraObjects, buildIssuerObject(issuer))
			}

			for _, clusterIssuer := range spec.CertManager.ClusterIssuers {
				extraObjects = append(extraObjects, buildClusterIssuerObject(clusterIssuer))
			}

			return extraObjects, nil
		},
		func(ctx context.Context) (map[string]any, error) {
			return nil, nil
		},
	)
}

func buildClusterIssuerObject(issuer v1alpha1.ClusterIssuerConfig) map[string]any {
	return map[string]any{
		"apiVersion": utils.ClusterIssuerResource.ApiVersion,
		"kind":       utils.ClusterIssuerResource.Kind,
		"metadata": map[string]any{
			"name": issuer.Name,
		},
		"spec": map[string]any{
			"acme": map[string]any{
				"email": issuer.Email,
				"privateKeySecretRef": map[string]any{
					"name": issuer.Name,
				},
				"server":  getServer(issuer.Server),
				"solvers": issuer.Solvers,
			},
		},
	}
}

func buildIssuerObject(issuer v1alpha1.IssuerConfig) map[string]any {
	return map[string]any{
		"apiVersion": utils.IssuerResource.ApiVersion,
		"kind":       utils.IssuerResource.Kind,
		"metadata": map[string]any{
			"name":      issuer.Name,
			"namespace": issuer.Namespace,
		},
		"spec": map[string]any{
			"acme": map[string]any{
				"email": issuer.Email,
				"privateKeySecretRef": map[string]any{
					"name": issuer.Name,
				},
				"server":  getServer(issuer.Server),
				"solvers": issuer.Solvers,
			},
		},
	}
}

func getServer(server string) string {
	if server == "" {
		return "https://acme-v02.api.letsencrypt.org/directory"
	}
	return server
}

func getSolvers(solvers []runtime.RawExtension) any {

	if len(solvers) == 0 {
		return []map[string]any{
			{
				"http01": map[string]any{},
			},
		}
	}
	return solvers
}
