package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"mogenius-operator/src/utils"
)

func (d *reconcilerModule) reconcileExternalDNS(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	c := spec.ExternalDNS
	if c == nil {
		c = &v1alpha1.ExternalDNSConfig{}
	}

	providerSecretName := fmt.Sprintf("%s-external-dns", c.Provider)
	externalDnsNamespace := helmNamespace(c.Chart, "external-dns")

	return d.reconcileComponent(ctx, spec, installer, op,
		componentSpec{
			enabled:          c.Enabled,
			chart:            c.Chart,
			patches:          c.Patches,
			name:             componentExternalDNS,
			defaultChart:     "external-dns",
			defaultRepo:      "https://kubernetes-sigs.github.io/external-dns/",
			defaultName:      "external-dns",
			defaultNamespace: externalDnsNamespace,
		},
		func(ctx context.Context) ([]any, error) {
			extraObjects := []any{}

			if c.ExternalSecret.Vault == "" {
				if len(spec.ExternalSecretsOperator.Vaults) > 0 {
					c.ExternalSecret.Vault = spec.ExternalSecretsOperator.Vaults[0].Name
				} else {
					return nil, fmt.Errorf("please provide a externalDns.externalSecret.vault or define a vault in spec.externalSecretsOperator")
				}
			}

			if d.crdChecker.IsAvailable(utils.ExternalSecretResource) {
				extraObjects = append(extraObjects, externalSecretResource(providerSecretName, externalDnsNamespace, c.ExternalSecret, nil, nil))
			}

			return extraObjects, nil
		},
		func(ctx context.Context) (map[string]any, error) {
			values := map[string]any{
				"provider": map[string]any{
					"name": c.Provider,
				},
				"domainFilters": c.DomainFilters,
			}

			clusterName, err := d.config.TryGet("MO_CLUSTER_NAME")
			if err != nil && clusterName != "" {
				values["txtOwnerId"] = clusterName
			}

			secretKey := "token"
			if c.ExternalSecret.Key != "" {
				secretKey = c.ExternalSecret.Key
			}

			switch c.Provider {
			case "cloudflare":
				values["env"] = []map[string]any{
					{
						"name": "CF_API_TOKEN",
						"valueFrom": map[string]any{
							"secretKeyRef": map[string]any{
								"name": providerSecretName,
								"key":  secretKey,
							},
						},
					},
				}
			}

			if d.crdChecker.IsAvailable(utils.ServiceMonitorResource) {
				values["serviceMonitor"] = map[string]any{
					"enabled": true,
				}
			}

			return values, nil
		},
	)
}
