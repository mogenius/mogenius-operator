package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"mogenius-operator/src/utils"
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
			extraObjects := []any{}

			for _, vault := range spec.ExternalSecretsOperator.Vaults {
				var provider map[string]any
				if err := json.Unmarshal(vault.Provider.Raw, &provider); err != nil {
					return nil, fmt.Errorf("parse provider for vault %q: %w", vault.Name, err)
				}

				extraObjects = append(extraObjects, map[string]any{
					"apiVersion": utils.ClusterSecretStoreResource.ApiVersion,
					"kind":       utils.ClusterSecretStoreResource.Kind,
					"metadata": map[string]any{
						"name": vault.Name,
					},
					"spec": map[string]any{
						"provider": map[string]any{
							vault.Type: provider,
						},
					},
				})
			}

			return extraObjects, nil
		},
		func(ctx context.Context) (map[string]any, error) {
			return nil, nil
		},
	)
}
