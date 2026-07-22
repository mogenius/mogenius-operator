package reconciler

import (
	"context"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"mogenius-operator/src/utils"
)

// reconcileOpenBao installs the OpenBao Helm chart via the GitOps engine. The
// actual initialization, unsealing, and ESO wiring happen out of band in the
// openbaomanager control loop, because the workload only comes up after the
// GitOps engine reconciles the artifact created here.
func (d *reconcilerModule) reconcileOpenBao(ctx context.Context, spec v1alpha1.PlatformConfigSpec, installer gitops.GitOpsInstaller, op operation) *ReconcileResult {
	c := spec.OpenBao
	if c == nil {
		c = &v1alpha1.OpenBaoConfig{}
	}
	return d.reconcileComponent(ctx, spec, installer, op,
		componentSpec{
			enabled:          c.Enabled,
			chart:            c.Chart,
			patches:          c.Patches,
			name:             componentOpenBao,
			defaultChart:     "openbao",
			defaultRepo:      "https://openbao.github.io/openbao-helm",
			defaultName:      "openbao",
			defaultNamespace: "openbao",
		},
		func(ctx context.Context) ([]any, error) {
			// The ClusterSecretStore is created by the openbaomanager once the
			// Kubernetes auth role exists in OpenBao, so ESO validation does not
			// fail against a backend that is not yet configured.
			return nil, nil
		},
		func(ctx context.Context) (map[string]any, error) {
			// Single-node integrated raft. HA can be tuned later via chart
			// values in the platform-defaults openbao.yaml or PlatformPatches.
			values := map[string]any{
				"server": map[string]any{
					"ha": map[string]any{
						"enabled":  true,
						"replicas": 1,
						"raft": map[string]any{
							"enabled": true,
						},
					},
				},
			}
			if d.crdChecker.IsAvailable(utils.ServiceMonitorResource) {
				values["serverTelemetry"] = map[string]any{
					"serviceMonitor": map[string]any{
						"enabled": true,
					},
				}
			}
			return values, nil
		},
	)
}
