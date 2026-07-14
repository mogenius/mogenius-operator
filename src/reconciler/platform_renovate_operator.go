package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"mogenius-operator/src/utils"
)

func (d *reconcilerModule) reconcileRenovateOperator(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	c := spec.RenovateOperator
	if c == nil {
		c = &v1alpha1.RenovateOperatorConfig{}
	}
	namespace := helmNamespace(c.Chart, "renovate-operator")
	return d.reconcileComponent(ctx, spec, installer, op,
		componentSpec{
			enabled:          c.Enabled,
			chart:            c.Chart,
			patches:          c.Patches,
			name:             componentRenovateOperator,
			defaultChart:     "renovate-operator",
			defaultRepo:      "oci://ghcr.io/mogenius/helm-charts/renovate-operator",
			defaultName:      "renovate-operator",
			defaultNamespace: namespace,
		},
		func(ctx context.Context) ([]any, error) {
			extraObjects := []any{}

			for _, rc := range c.Repositories {
				name := rc.Name
				if name == "" {
					name = rc.GitOpsRepository
				}
				if name == "" {
					return nil, fmt.Errorf("renovate job for filter %q requires a name", rc.Filter)
				}

				if rc.ExternalSecret != nil {
					if rc.ExternalSecret.Vault == "" {
						if spec.ExternalSecretsOperator != nil && len(spec.ExternalSecretsOperator.Vaults) > 0 {
							rc.ExternalSecret.Vault = spec.ExternalSecretsOperator.Vaults[0].Name
						} else {
							return nil, fmt.Errorf("renovate job %q: provide externalSecret.vault or define a vault in spec.externalSecretsOperator", name)
						}
					}
					if d.crdChecker.IsAvailable(utils.ExternalSecretResource) {
						extraObjects = append(extraObjects, externalSecretResource(name, namespace, *rc.ExternalSecret, nil, nil))
					}
				}

				extraObjects = append(extraObjects, renovateJobObject(name, rc, namespace))
			}

			return extraObjects, nil
		},
		func(ctx context.Context) (map[string]any, error) {
			values := map[string]any{}

			if c.MaxParallelJobs > 0 {
				values["config"] = map[string]any{
					"globalParallelismLimit": c.MaxParallelJobs,
				}
			}

			if d.crdChecker.IsAvailable(utils.ServiceMonitorResource) {
				values["metrics"] = map[string]any{
					"enabled": true,
					"serviceMonitor": map[string]any{
						"enabled": true,
					},
				}
			}

			if len(values) == 0 {
				return nil, nil
			}
			return values, nil
		},
	)
}

func renovateJobObject(name string, rc v1alpha1.RenovateJobConfig, namespace string) map[string]any {
	schedule := rc.Schedule
	if schedule == "" {
		schedule = "0 * * * *"
	}

	topic := rc.GitOpsRepository
	if topic == "" {
		topic = rc.Filter
	}

	provider := map[string]any{
		"name": rc.Provider.Name,
	}
	if rc.Provider.Endpoint != "" {
		provider["endpoint"] = rc.Provider.Endpoint
	}

	spec := map[string]any{
		"schedule":    schedule,
		"provider":    provider,
		"parallelism": 1,
	}
	if topic != "" {
		spec["discoverTopics"] = []any{topic}
	}
	if rc.ExternalSecret != nil {
		spec["secretRef"] = name
	}

	return map[string]any{
		"apiVersion": "renovate-operator.mogenius.com/v1alpha1",
		"kind":       "RenovateJob",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
		"spec": spec,
	}
}
