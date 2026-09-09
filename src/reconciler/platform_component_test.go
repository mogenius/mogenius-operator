package reconciler

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// recordingInstaller captures what a component reconciler asked for instead of
// touching the cluster.
type recordingInstaller struct {
	installed   []string
	uninstalled []string
}

func (r *recordingInstaller) Install(component string, _ gitops.GitOpsArtifact) error {
	r.installed = append(r.installed, component)
	return nil
}

func (r *recordingInstaller) UnInstall(component string) error {
	r.uninstalled = append(r.uninstalled, component)
	return nil
}

func testModule() *reconcilerModule {
	return &reconcilerModule{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

// componentReconciler is the shared shape of every component reconcile method.
type componentReconciler func(*reconcilerModule, context.Context, v1alpha1.PlatformConfigSpec, gitops.GitOpsInstaller, operation) *ReconcileResult

// componentCase pairs a reconciler with a spec that declares its component
// disabled and one that does not mention it at all.
type componentCase struct {
	name      string
	component string
	reconcile componentReconciler
	absent    v1alpha1.PlatformConfigSpec
	disabled  v1alpha1.PlatformConfigSpec
	enabled   v1alpha1.PlatformConfigSpec
}

func componentCases() []componentCase {
	return []componentCase{
		{
			name:      "certManager",
			component: componentCertManager,
			reconcile: (*reconcilerModule).reconcileCertManager,
			absent:    v1alpha1.PlatformConfigSpec{},
			disabled:  v1alpha1.PlatformConfigSpec{CertManager: &v1alpha1.CertManagerConfig{Enabled: false}},
			enabled:   v1alpha1.PlatformConfigSpec{CertManager: &v1alpha1.CertManagerConfig{Enabled: true}},
		},
		{
			name:      "traefik",
			component: componentTraefik,
			reconcile: (*reconcilerModule).reconcileTraefik,
			absent:    v1alpha1.PlatformConfigSpec{},
			disabled:  v1alpha1.PlatformConfigSpec{Traefik: &v1alpha1.TraefikConfig{Enabled: false}},
			enabled:   v1alpha1.PlatformConfigSpec{Traefik: &v1alpha1.TraefikConfig{Enabled: true}},
		},
		{
			name:      "externalDns",
			component: componentExternalDNS,
			reconcile: (*reconcilerModule).reconcileExternalDNS,
			absent:    v1alpha1.PlatformConfigSpec{},
			disabled:  v1alpha1.PlatformConfigSpec{ExternalDNS: &v1alpha1.ExternalDNSConfig{Enabled: false}},
			enabled:   v1alpha1.PlatformConfigSpec{ExternalDNS: &v1alpha1.ExternalDNSConfig{Enabled: true}},
		},
		{
			name:      "kubePrometheusStack",
			component: componentKubePrometheusStack,
			reconcile: (*reconcilerModule).reconcileKubePrometheusStack,
			absent:    v1alpha1.PlatformConfigSpec{},
			disabled:  v1alpha1.PlatformConfigSpec{KubePrometheusStack: &v1alpha1.KubePrometheusStackConfig{Enabled: false}},
			enabled:   v1alpha1.PlatformConfigSpec{KubePrometheusStack: &v1alpha1.KubePrometheusStackConfig{Enabled: true}},
		},
		{
			name:      "loki",
			component: componentLoki,
			reconcile: (*reconcilerModule).reconcileLoki,
			absent:    v1alpha1.PlatformConfigSpec{},
			disabled:  v1alpha1.PlatformConfigSpec{Loki: &v1alpha1.LokiConfig{Enabled: false}},
			enabled:   v1alpha1.PlatformConfigSpec{Loki: &v1alpha1.LokiConfig{Enabled: true}},
		},
		{
			name:      "alloy",
			component: componentAlloy,
			reconcile: (*reconcilerModule).reconcileAlloy,
			absent:    v1alpha1.PlatformConfigSpec{},
			disabled:  v1alpha1.PlatformConfigSpec{Alloy: &v1alpha1.AlloyConfig{Enabled: false}},
			enabled:   v1alpha1.PlatformConfigSpec{Alloy: &v1alpha1.AlloyConfig{Enabled: true}},
		},
		{
			name:      "renovateOperator",
			component: componentRenovateOperator,
			reconcile: (*reconcilerModule).reconcileRenovateOperator,
			absent:    v1alpha1.PlatformConfigSpec{},
			disabled:  v1alpha1.PlatformConfigSpec{RenovateOperator: &v1alpha1.RenovateOperatorConfig{Enabled: false}},
			enabled:   v1alpha1.PlatformConfigSpec{RenovateOperator: &v1alpha1.RenovateOperatorConfig{Enabled: true}},
		},
		{
			name:      "externalSecretsOperator",
			component: componentExternalSecretsOperator,
			reconcile: (*reconcilerModule).reconcileExternalSecretsOperator,
			absent:    v1alpha1.PlatformConfigSpec{},
			disabled:  v1alpha1.PlatformConfigSpec{ExternalSecretsOperator: &v1alpha1.ExternalSecretsOperatorConfig{Enabled: false}},
			enabled:   v1alpha1.PlatformConfigSpec{ExternalSecretsOperator: &v1alpha1.ExternalSecretsOperatorConfig{Enabled: true}},
		},
		{
			name:      "fluxcd",
			component: componentFluxCD,
			reconcile: (*reconcilerModule).reconcileFluxCD,
			absent:    v1alpha1.PlatformConfigSpec{},
			disabled:  v1alpha1.PlatformConfigSpec{GitOps: &v1alpha1.GitOpsConfig{FluxCD: &v1alpha1.FluxCDInstallConfig{Enabled: false}}},
			enabled:   v1alpha1.PlatformConfigSpec{GitOps: &v1alpha1.GitOpsConfig{FluxCD: &v1alpha1.FluxCDInstallConfig{Enabled: true}}},
		},
		{
			name:      "argocd",
			component: componentArgoCD,
			reconcile: (*reconcilerModule).reconcileArgoCD,
			absent:    v1alpha1.PlatformConfigSpec{},
			disabled:  v1alpha1.PlatformConfigSpec{GitOps: &v1alpha1.GitOpsConfig{ArgoCD: &v1alpha1.ArgoCDInstallConfig{Enabled: false}}},
			enabled:   v1alpha1.PlatformConfigSpec{GitOps: &v1alpha1.GitOpsConfig{ArgoCD: &v1alpha1.ArgoCDInstallConfig{Enabled: true}}},
		},
	}
}

// An absent block is not a request to remove anything. The PlatformConfig is
// synced from git with prune enabled, so a renamed file or a broken path makes
// the whole spec disappear — and uninstalling on that would take the platform
// down with it.
func TestComponentAbsentFromSpecTouchesNothing(t *testing.T) {
	for _, tc := range componentCases() {
		t.Run(tc.name, func(t *testing.T) {
			installer := &recordingInstaller{}

			result := tc.reconcile(testModule(), context.Background(), tc.absent, installer, updateOperation)

			assert.Nil(t, result)
			assert.Empty(t, installer.uninstalled, "an undeclared component must not be uninstalled")
			assert.Empty(t, installer.installed, "an undeclared component must not be installed")
		})
	}
}

// enabled:false is the only way to remove a component.
func TestComponentDisabledInSpecIsUninstalled(t *testing.T) {
	for _, tc := range componentCases() {
		t.Run(tc.name, func(t *testing.T) {
			installer := &recordingInstaller{}

			result := tc.reconcile(testModule(), context.Background(), tc.disabled, installer, updateOperation)

			assert.Nil(t, result)
			assert.Equal(t, []string{tc.component}, installer.uninstalled)
			assert.Empty(t, installer.installed)
		})
	}
}

// The delete guard sits in reconcileComponent as well as in
// reconcilePlatformConfig, so an enabled component is still left alone when the
// resource itself is being deleted.
func TestComponentDeleteOperationTouchesNothing(t *testing.T) {
	for _, tc := range componentCases() {
		t.Run(tc.name, func(t *testing.T) {
			installer := &recordingInstaller{}

			result := tc.reconcile(testModule(), context.Background(), tc.enabled, installer, deleteOperation)

			assert.Nil(t, result)
			assert.Empty(t, installer.uninstalled)
			assert.Empty(t, installer.installed)
		})
	}
}

// The primary guard: a deleted PlatformConfig returns before any component is
// reconciled and before a status patch is attempted on a resource that is gone.
func TestReconcilePlatformConfigIgnoresDelete(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "mogenius.com/v1alpha1",
		"kind":       "PlatformConfig",
		"metadata":   map[string]any{"name": "platform"},
		"spec": map[string]any{
			"certManager": map[string]any{"enabled": true},
			"gitOps": map[string]any{
				"fluxcd": map[string]any{"enabled": true},
			},
		},
	}}

	// A nil clientProvider would panic on any cluster access, which is what
	// makes this assertion meaningful: returning nil proves nothing was tried.
	results := testModule().reconcilePlatformConfig(context.Background(), obj, deleteOperation)

	assert.Nil(t, results)
}
