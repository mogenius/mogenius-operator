package reconciler

import (
	"testing"

	"mogenius-operator/src/crds/v1alpha1"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func fluxDetection() *engineDetection {
	return &engineDetection{
		engine:      gitOpsEngineFlux,
		installed:   true,
		namespace:   "flux-system",
		releaseName: "flux-operator",
		version:     "v2.7.5",
		controllers: []v1alpha1.GitOpsControllerStatus{
			{Name: "source-controller", Version: "v1.7.1", Ready: true},
		},
	}
}

func argoCDDetection() *engineDetection {
	return &engineDetection{
		engine:      gitOpsEngineArgoCD,
		installed:   true,
		namespace:   "argo",
		releaseName: "my-argo",
		version:     "v2.13.0",
		controllers: []v1alpha1.GitOpsControllerStatus{
			{Name: "argocd-server", Version: "v2.13.0", Ready: true},
		},
	}
}

// leftoverDetection is what an uninstalled engine leaves behind: its CRDs are
// still registered, but nothing runs.
func leftoverDetection(engine string) *engineDetection {
	return &engineDetection{engine: engine, installed: false}
}

func TestBuildGitOpsStatusSpecWins(t *testing.T) {
	t.Parallel()

	spec := v1alpha1.PlatformConfigSpec{
		GitOps: &v1alpha1.GitOpsConfig{
			ArgoCD: &v1alpha1.ArgoCDInstallConfig{Enabled: true, Project: "custom"},
		},
	}
	// A flux install alongside must not override the engine the spec owns.
	detection := gitOpsDetection{argoCD: argoCDDetection(), flux: fluxDetection()}

	status := buildGitOpsStatus(spec, detection)

	assert.Equal(t, gitOpsEngineArgoCD, status.Engine)
	assert.Equal(t, gitOpsSourceSpec, status.Source)
	assert.False(t, status.IsUserManaged)
	assert.Equal(t, "custom", status.DefaultProjectName)
	// Live facts still come from the cluster.
	assert.True(t, status.Installed)
	assert.Equal(t, "argo", status.Namespace)
	assert.Equal(t, "my-argo", status.ReleaseName)
	assert.Equal(t, "v2.13.0", status.Version)
	assert.Len(t, status.Controllers, 1)
}

func TestBuildGitOpsStatusSpecChartOverridesDetection(t *testing.T) {
	t.Parallel()

	spec := v1alpha1.PlatformConfigSpec{
		GitOps: &v1alpha1.GitOpsConfig{
			FluxCD: &v1alpha1.FluxCDInstallConfig{
				Enabled: true,
				Chart:   &v1alpha1.HelmChartReference{Namespace: "gitops", Name: "flux"},
			},
		},
	}

	status := buildGitOpsStatus(spec, gitOpsDetection{flux: fluxDetection()})

	assert.Equal(t, "gitops", status.Namespace)
	assert.Equal(t, "flux", status.ReleaseName)
	assert.Equal(t, "v2.7.5", status.Version)
}

func TestBuildGitOpsStatusSpecWithoutDetection(t *testing.T) {
	t.Parallel()

	spec := v1alpha1.PlatformConfigSpec{
		GitOps: &v1alpha1.GitOpsConfig{
			FluxCD: &v1alpha1.FluxCDInstallConfig{Enabled: true},
		},
	}

	status := buildGitOpsStatus(spec, gitOpsDetection{})

	assert.Equal(t, gitOpsEngineFlux, status.Engine)
	assert.Equal(t, gitOpsSourceSpec, status.Source)
	assert.False(t, status.Installed)
	assert.Equal(t, fluxcdDefaultNamespace, status.Namespace)
	assert.Equal(t, "flux-operator", status.ReleaseName)
	assert.Empty(t, status.Version)
}

func TestBuildGitOpsStatusDetectedOnly(t *testing.T) {
	t.Parallel()

	status := buildGitOpsStatus(v1alpha1.PlatformConfigSpec{}, gitOpsDetection{flux: fluxDetection()})

	assert.Equal(t, gitOpsEngineFlux, status.Engine)
	assert.Equal(t, gitOpsSourceDetected, status.Source)
	assert.True(t, status.Installed)
	assert.True(t, status.IsUserManaged)
	assert.Equal(t, "flux-system", status.Namespace)
	assert.Equal(t, "flux-operator", status.ReleaseName)
	assert.Equal(t, "v2.7.5", status.Version)
	assert.Empty(t, status.DefaultProjectName)
}

func TestBuildGitOpsStatusDetectedWinsOverDisabledSpec(t *testing.T) {
	t.Parallel()

	spec := v1alpha1.PlatformConfigSpec{
		GitOps: &v1alpha1.GitOpsConfig{
			ArgoCD: &v1alpha1.ArgoCDInstallConfig{Enabled: false},
		},
	}

	status := buildGitOpsStatus(spec, gitOpsDetection{flux: fluxDetection()})

	assert.Equal(t, gitOpsEngineFlux, status.Engine)
	assert.Equal(t, gitOpsSourceDetected, status.Source)
	assert.True(t, status.IsUserManaged)
	assert.True(t, status.Installed)
	// The ArgoCD project of the disabled spec must not leak into a flux status.
	assert.Empty(t, status.DefaultProjectName)
}

func TestBuildGitOpsStatusDisabledSpecWithoutDetection(t *testing.T) {
	t.Parallel()

	spec := v1alpha1.PlatformConfigSpec{
		GitOps: &v1alpha1.GitOpsConfig{
			ArgoCD: &v1alpha1.ArgoCDInstallConfig{Enabled: false},
		},
	}

	status := buildGitOpsStatus(spec, gitOpsDetection{})

	assert.Equal(t, gitOpsEngineArgoCD, status.Engine)
	assert.Equal(t, gitOpsSourceSpec, status.Source)
	assert.True(t, status.IsUserManaged)
	assert.False(t, status.Installed)
	// A disabled spec means the engine belongs to the user, so the reported
	// project must be one that exists there. "mogenius" only exists where
	// mogenius installed Argo CD and created it.
	assert.Equal(t, argoCDFallbackProject, status.DefaultProjectName)
}

func TestArgoProjectName(t *testing.T) {
	t.Parallel()

	// Explicit wins in both directions -- it is the user's declaration.
	assert.Equal(t, "custom", argoProjectName(&v1alpha1.GitOpsConfig{
		ArgoCD: &v1alpha1.ArgoCDInstallConfig{Enabled: true, Project: "custom"},
	}))
	assert.Equal(t, "custom", argoProjectName(&v1alpha1.GitOpsConfig{
		ArgoCD: &v1alpha1.ArgoCDInstallConfig{Enabled: false, Project: "custom"},
	}))

	// mogenius owns the engine, so it also creates the project it names.
	assert.Equal(t, argoCDDefaultProject, argoProjectName(&v1alpha1.GitOpsConfig{
		ArgoCD: &v1alpha1.ArgoCDInstallConfig{Enabled: true},
	}))

	// The engine is the user's: only Argo CD's built-in project can be assumed.
	assert.Equal(t, argoCDFallbackProject, argoProjectName(&v1alpha1.GitOpsConfig{
		ArgoCD: &v1alpha1.ArgoCDInstallConfig{Enabled: false},
	}))
	assert.Equal(t, argoCDFallbackProject, argoProjectName(nil))
	assert.Equal(t, argoCDFallbackProject, argoProjectName(&v1alpha1.GitOpsConfig{}))
}

func TestBuildGitOpsStatusNothingFound(t *testing.T) {
	t.Parallel()

	status := buildGitOpsStatus(v1alpha1.PlatformConfigSpec{}, gitOpsDetection{})

	// "checked, nothing there" must be distinguishable from "never checked".
	assert.NotNil(t, status)
	assert.False(t, status.Installed)
	assert.Empty(t, status.Engine)
	assert.Equal(t, gitOpsSourceDetected, status.Source)
}

func TestGitOpsStatusEqual(t *testing.T) {
	t.Parallel()

	current := buildGitOpsStatus(v1alpha1.PlatformConfigSpec{}, gitOpsDetection{flux: fluxDetection()})
	desired := buildGitOpsStatus(v1alpha1.PlatformConfigSpec{}, gitOpsDetection{flux: fluxDetection()})
	assert.True(t, gitOpsStatusEqual(current, desired))

	desired.Controllers[0].Ready = false
	assert.False(t, gitOpsStatusEqual(current, desired))

	assert.True(t, gitOpsStatusEqual(nil, nil))
	assert.False(t, gitOpsStatusEqual(nil, current))
}

func TestControllerStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		deployment map[string]any
		want       v1alpha1.GitOpsControllerStatus
	}{
		{
			name: "version from label, ready",
			deployment: map[string]any{
				"metadata": map[string]any{
					"name":   "source-controller",
					"labels": map[string]any{versionLabel: "v1.7.1"},
				},
				"spec":   map[string]any{"replicas": int64(1)},
				"status": map[string]any{"readyReplicas": int64(1)},
			},
			want: v1alpha1.GitOpsControllerStatus{Name: "source-controller", Version: "v1.7.1", Ready: true},
		},
		{
			name: "version falls back to image tag",
			deployment: map[string]any{
				"metadata": map[string]any{"name": "kustomize-controller"},
				"spec": map[string]any{
					"replicas": int64(2),
					"template": map[string]any{"spec": map[string]any{"containers": []any{
						map[string]any{"image": "ghcr.io/fluxcd/kustomize-controller:v1.7.0"},
					}}},
				},
				"status": map[string]any{"readyReplicas": int64(1)},
			},
			want: v1alpha1.GitOpsControllerStatus{Name: "kustomize-controller", Version: "v1.7.0", Ready: false},
		},
		{
			name: "scaled to zero is not ready",
			deployment: map[string]any{
				"metadata": map[string]any{"name": "notification-controller"},
				"spec":     map[string]any{"replicas": int64(0)},
				"status":   map[string]any{},
			},
			want: v1alpha1.GitOpsControllerStatus{Name: "notification-controller"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, controllerStatus(unstructured.Unstructured{Object: tt.deployment}))
		})
	}
}

func TestImageTag(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "v1.7.1", imageTag("ghcr.io/fluxcd/source-controller:v1.7.1"))
	assert.Equal(t, "v1.7.1", imageTag("registry.local:5000/fluxcd/source-controller:v1.7.1"))
	assert.Equal(t, "v1.7.1", imageTag("ghcr.io/fluxcd/source-controller:v1.7.1@sha256:abc"))
	assert.Empty(t, imageTag("ghcr.io/fluxcd/source-controller"))
	assert.Empty(t, imageTag(""))
}

func TestMostCommonControllerVersion(t *testing.T) {
	t.Parallel()

	controllers := []v1alpha1.GitOpsControllerStatus{
		{Version: "v1.7.1"},
		{Version: "v1.7.1"},
		{Version: "v1.6.0"},
		{Version: ""},
	}
	assert.Equal(t, "v1.7.1", mostCommonControllerVersion(controllers))
	assert.Empty(t, mostCommonControllerVersion(nil))
}

func TestDeliveryEngineSpecWins(t *testing.T) {
	t.Parallel()

	// mogenius installs the engine, so its own namespace is authoritative even
	// when a different engine happens to be running as well.
	engine, namespace := deliveryEngine(gitOpsEngineArgoCD, "argocd", &v1alpha1.GitOpsStatus{
		Engine:    gitOpsEngineFlux,
		Installed: true,
		Namespace: "flux-system",
	})

	assert.Equal(t, gitOpsEngineArgoCD, engine)
	assert.Equal(t, "argocd", namespace)
}

func TestDeliveryEngineFallsBackToDetected(t *testing.T) {
	t.Parallel()

	// The onboarding case: Helm installed the engine, spec.gitOps leaves it
	// disabled. Components must still be delivered through it.
	engine, namespace := deliveryEngine("", "", &v1alpha1.GitOpsStatus{
		Engine:    gitOpsEngineFlux,
		Installed: true,
		Namespace: "flux-system",
	})

	assert.Equal(t, gitOpsEngineFlux, engine)
	assert.Equal(t, "flux-system", namespace)
}

func TestDeliveryEngineIgnoresDeclaredButAbsentEngine(t *testing.T) {
	t.Parallel()

	// Declared without running: there is no namespace to create Applications
	// in, so this must not be mistaken for a usable engine.
	engine, namespace := deliveryEngine("", "", &v1alpha1.GitOpsStatus{
		Engine:    gitOpsEngineArgoCD,
		Installed: false,
		Namespace: "argocd",
	})

	assert.Empty(t, engine)
	assert.Empty(t, namespace)
}

func TestDeliveryEngineNoEngineAnywhere(t *testing.T) {
	t.Parallel()

	engine, namespace := deliveryEngine("", "", &v1alpha1.GitOpsStatus{})
	assert.Empty(t, engine)
	assert.Empty(t, namespace)

	engine, namespace = deliveryEngine("", "", nil)
	assert.Empty(t, engine)
	assert.Empty(t, namespace)
}

func TestBuildGitOpsStatusLeftoverCRDsDoNotOutrankRunningEngine(t *testing.T) {
	t.Parallel()

	// The reported symptom: Argo CD was uninstalled, its CRDs stayed, and Flux
	// is what actually runs. Reporting Argo CD here also hides Flux, because the
	// API matches the reported engine before it believes either is installed.
	spec := v1alpha1.PlatformConfigSpec{
		GitOps: &v1alpha1.GitOpsConfig{
			FluxCD: &v1alpha1.FluxCDInstallConfig{Enabled: false},
		},
	}
	detection := gitOpsDetection{argoCD: leftoverDetection(gitOpsEngineArgoCD), flux: fluxDetection()}

	status := buildGitOpsStatus(spec, detection)

	assert.Equal(t, gitOpsEngineFlux, status.Engine)
	assert.Equal(t, gitOpsSourceDetected, status.Source)
	assert.True(t, status.Installed)
	assert.Equal(t, "flux-system", status.Namespace)
}

func TestBuildGitOpsStatusLeftoverCRDsAreNotInstalled(t *testing.T) {
	t.Parallel()

	spec := v1alpha1.PlatformConfigSpec{
		GitOps: &v1alpha1.GitOpsConfig{
			ArgoCD: &v1alpha1.ArgoCDInstallConfig{Enabled: false},
		},
	}

	status := buildGitOpsStatus(spec, gitOpsDetection{argoCD: leftoverDetection(gitOpsEngineArgoCD)})

	// Nothing runs, so nothing may be delivered through it either.
	assert.False(t, status.Installed)
	engine, namespace := deliveryEngine("", "", status)
	assert.Empty(t, engine)
	assert.Empty(t, namespace)
}

func TestPreferredIgnoresEnginesThatDoNotRun(t *testing.T) {
	t.Parallel()

	assert.Nil(t, gitOpsDetection{}.preferred())
	assert.Nil(t, gitOpsDetection{argoCD: leftoverDetection(gitOpsEngineArgoCD)}.preferred())

	// Both running: Argo CD wins, it is the engine mogenius installs by default.
	both := gitOpsDetection{argoCD: argoCDDetection(), flux: fluxDetection()}
	assert.Equal(t, gitOpsEngineArgoCD, both.preferred().engine)
}
