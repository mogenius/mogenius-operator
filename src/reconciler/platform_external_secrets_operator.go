package reconciler

import (
	"context"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
)

func (d *reconcilerModule) reconcileExternalSecretsOperator(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	c := spec.ExternalSecretsOperator
	if c == nil {
		c = &v1alpha1.ExternalSecretsOperatorConfig{}
	}
	defaultNs := "external-secrets-operator"
	return d.reconcileComponent(ctx, spec, installer, op,
		componentSpec{
			enabled:          c.Enabled,
			chart:            c.Chart,
			patches:          c.Patches,
			name:             componentExternalSecretsOperator,
			defaultChart:     "external-secrets",
			defaultRepo:      "https://charts.external-secrets.io",
			defaultName:      "external-secrets-operator",
			defaultNamespace: defaultNs,
		},
		func(ctx context.Context) ([]any, error) {
			// build extra objects
			extraObjects := []any{}

			installNamespace := helmNamespace(spec.ExternalSecretsOperator.Chart, defaultNs)

			for _, vault := range spec.ExternalSecretsOperator.Vaults {
				vaultObj := map[string]any{
					"apiVersion": "external-secrets.io/v1",
					"kind":       "ClusterSecretStore",
					"metadata": map[string]any{
						"name": vault.Name,
					},
					"spec": map[string]any{
						"provider": getVaultProvider(vault, installNamespace),
					},
				}

				extraObjects = append(extraObjects, vaultObj)
			}

			return extraObjects, nil
		},
		func(ctx context.Context) (map[string]any, error) {
			return nil, nil
		},
	)
}

func getVaultProvider(vault v1alpha1.ExternalSecretVault, namespace string) map[string]any {

	switch vault.Type {
	case "1password":
		return map[string]any{
			"onepasswordSDK": map[string]any{
				"vault": vault.Name,
				"auth": map[string]any{
					"serviceAccountSecretRef": map[string]any{
						"name":      vault.ServiceAccountSecretRef.Name,
						"key":       vault.ServiceAccountSecretRef.Key,
						"namespace": namespace,
					},
				},
			},
		}
	}

	return map[string]any{}
}
