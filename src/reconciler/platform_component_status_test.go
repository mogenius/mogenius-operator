package reconciler

import (
	"testing"

	"mogenius-operator/src/crds/v1alpha1"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDeclaredComponentsReportsOnlyWhatTheSpecCarries(t *testing.T) {
	// The spec from the cluster this was found on: an engine, Traefik and the
	// Renovate operator, and nothing else.
	spec := v1alpha1.PlatformConfigSpec{
		GitOps:           &v1alpha1.GitOpsConfig{FluxCD: &v1alpha1.FluxCDInstallConfig{Enabled: true}},
		Traefik:          &v1alpha1.TraefikConfig{Enabled: true},
		RenovateOperator: &v1alpha1.RenovateOperatorConfig{Enabled: true},
	}

	declared := declaredComponents(spec)

	assert.True(t, declared[componentTraefik].declared)
	assert.True(t, declared[componentTraefik].enabled)
	assert.True(t, declared[componentFluxCD].enabled)
	assert.True(t, declared[componentRenovateOperator].enabled)

	// The ones that used to be reported as Ready on exactly this cluster.
	for _, component := range []string{
		componentCertManager,
		componentExternalDNS,
		componentKubePrometheusStack,
		componentLoki,
		componentAlloy,
		componentExternalSecretsOperator,
		componentArgoCD,
	} {
		assert.False(t, declared[component].declared, "%s is not in the spec and must not be reported", component)
	}
}

func TestDeclaredComponentsSeparatesDisabledFromAbsent(t *testing.T) {
	declared := declaredComponents(v1alpha1.PlatformConfigSpec{
		Traefik: &v1alpha1.TraefikConfig{Enabled: false},
	})

	assert.True(t, declared[componentTraefik].declared, "an explicit block is declared even when it is off")
	assert.False(t, declared[componentTraefik].enabled)
	assert.False(t, declared[componentLoki].declared)
}

func TestDeclaredComponentsIgnoresApplicationRepositories(t *testing.T) {
	// An application entry is declared for the mogenius platform, not for this
	// operator -- no sync objects, so no condition claiming any.
	onlyApplications := declaredComponents(v1alpha1.PlatformConfigSpec{
		GitOps: &v1alpha1.GitOpsConfig{
			Repositories: []v1alpha1.GitOpsRepositoryConfig{
				{Name: "apps", URL: "https://example.com/apps.git", Type: v1alpha1.RepositoryTypeApplication},
			},
		},
	})
	assert.False(t, onlyApplications[componentPlatformRepositories].declared)

	mixed := declaredComponents(v1alpha1.PlatformConfigSpec{
		GitOps: &v1alpha1.GitOpsConfig{
			Repositories: []v1alpha1.GitOpsRepositoryConfig{
				{Name: "apps", URL: "https://example.com/apps.git", Type: v1alpha1.RepositoryTypeApplication},
				{Name: "platform", URL: "https://example.com/p.git"},
			},
		},
	})
	assert.True(t, mixed[componentPlatformRepositories].declared, "the untyped entry is a platform repository")
}

func TestDeclaredComponentsReportsRepositoriesOnlyWhenDeclared(t *testing.T) {
	without := declaredComponents(v1alpha1.PlatformConfigSpec{GitOps: &v1alpha1.GitOpsConfig{}})
	assert.False(t, without[componentPlatformRepositories].declared)

	with := declaredComponents(v1alpha1.PlatformConfigSpec{
		GitOps: &v1alpha1.GitOpsConfig{
			Repositories: []v1alpha1.GitOpsRepositoryConfig{{Name: "platform", URL: "https://example.com/p.git"}},
		},
	})
	assert.True(t, with[componentPlatformRepositories].declared)
	assert.True(t, with[componentPlatformRepositories].enabled)
}

func helmRelease(conditions []any) *unstructured.Unstructured {
	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "helm.toolkit.fluxcd.io/v2",
		"kind":       "HelmRelease",
		"metadata":   map[string]any{"name": "traefik", "namespace": "flux-system"},
	}}
	if conditions != nil {
		object.Object["status"] = map[string]any{"conditions": conditions}
	}
	return object
}

// The failure this was built for: the operator's own defaults against a newer
// chart. Applying the HelmRelease worked, the install did not, and the only
// place that existed was the engine's object.
func TestFluxDeliveryStateReportsAFailedInstall(t *testing.T) {
	message := "Helm install failed for release traefik/traefik with chart traefik@41.5.0: " +
		"values don't meet the specifications of the schema(s) in the following chart(s): " +
		"traefik: - at '': additional properties 'logs' not allowed"

	state := fluxDeliveryState(helmRelease([]any{
		map[string]any{"type": "Released", "status": "False", "message": "install retries exhausted"},
		map[string]any{"type": "Ready", "status": "False", "message": message},
	}))

	assert.True(t, state.found)
	assert.False(t, state.ready)
	assert.Equal(t, message, state.message, "the engine's message is what makes the failure diagnosable")
}

func TestFluxDeliveryStateReportsASuccessfulInstall(t *testing.T) {
	state := fluxDeliveryState(helmRelease([]any{
		map[string]any{"type": "Ready", "status": "True", "message": "Helm install succeeded"},
	}))

	assert.True(t, state.found)
	assert.True(t, state.ready)
}

// Right after the object is applied there is no status yet. That is pending,
// not ready and not failed.
func TestFluxDeliveryStateWithoutStatusIsNeitherReadyNorFailed(t *testing.T) {
	state := fluxDeliveryState(helmRelease(nil))

	assert.True(t, state.found)
	assert.False(t, state.ready)
	assert.Empty(t, state.message)
}

func argoApp(health string, sync string, operation string) *unstructured.Unstructured {
	status := map[string]any{}
	if health != "" {
		status["health"] = map[string]any{"status": health}
	}
	if sync != "" {
		status["sync"] = map[string]any{"status": sync}
	}
	if operation != "" {
		status["operationState"] = map[string]any{"message": operation}
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]any{"name": "traefik"},
		"status":     status,
	}}
}

func TestArgoDeliveryStateNeedsHealthAndSync(t *testing.T) {
	assert.True(t, argoDeliveryState(argoApp("Healthy", "Synced", "")).ready)

	// Synced but broken is not a running component.
	degraded := argoDeliveryState(argoApp("Degraded", "Synced", "pod crash looping"))
	assert.False(t, degraded.ready)
	assert.Contains(t, degraded.message, "Degraded")
	assert.Contains(t, degraded.message, "pod crash looping")

	// Healthy but never synced reports on the previous revision.
	outOfSync := argoDeliveryState(argoApp("Healthy", "OutOfSync", ""))
	assert.False(t, outOfSync.ready)
	assert.Contains(t, outOfSync.message, "OutOfSync")
}

func TestArgoDeliveryStateWithoutStatusIsPending(t *testing.T) {
	state := argoDeliveryState(argoApp("", "", ""))

	assert.True(t, state.found)
	assert.False(t, state.ready)
	assert.Empty(t, state.message)
}

// The engine is installed with the Helm SDK, so there is no engine object to
// read: the detected status is the only truth about it.
func TestDeliveredStateForTheEngineUsesTheDetectedStatus(t *testing.T) {
	running, ok := deliveredStateFor(componentFluxCD, nil, &v1alpha1.GitOpsStatus{Installed: true})
	assert.True(t, ok)
	assert.True(t, running.ready)

	absent, ok := deliveredStateFor(componentFluxCD, nil, &v1alpha1.GitOpsStatus{Installed: false})
	assert.True(t, ok)
	assert.False(t, absent.ready)
	assert.Contains(t, absent.message, "controllers are not running")

	_, ok = deliveredStateFor(componentArgoCD, nil, nil)
	assert.False(t, ok, "without a status there is nothing to claim")
}

// The repository sync objects are applied directly, not through an engine
// object, so their own reconcile result is the only verdict.
func TestDeliveredStateForRepositoriesHasNoEngineObject(t *testing.T) {
	_, ok := deliveredStateFor(componentPlatformRepositories, map[string]deliveryState{
		componentPlatformRepositories: {found: true, ready: true},
	}, &v1alpha1.GitOpsStatus{Installed: true})

	assert.False(t, ok)
}

func TestDeliveredStateForAComponentTheEngineHasNotSeen(t *testing.T) {
	_, ok := deliveredStateFor(componentTraefik, map[string]deliveryState{}, &v1alpha1.GitOpsStatus{Installed: true})

	assert.False(t, ok, "no object yet is not the same as a verdict")
}
