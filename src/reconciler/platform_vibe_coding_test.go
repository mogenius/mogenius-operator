package reconciler

import (
	"context"
	"testing"

	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"mogenius-operator/src/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// artifactInstaller keeps the artifact ApplyExtras was handed.
type artifactInstaller struct {
	recordingInstaller
	artifact gitops.GitOpsArtifact
}

func (a *artifactInstaller) ApplyExtras(component string, artifact gitops.GitOpsArtifact) error {
	a.artifact = artifact
	return a.recordingInstaller.ApplyExtras(component, artifact)
}

// vibeCodingTestModule reports Flux's CRDs as installed, so the OCIRepository
// and Kustomization are built.
func vibeCodingTestModule() *reconcilerModule {
	d := testModule()
	d.crdChecker = &crdChecker{present: map[string]struct{}{
		utils.OCIRepositoryResource.ApiVersion + "/" + utils.OCIRepositoryResource.Kind: {},
		utils.KustomizationResource.ApiVersion + "/" + utils.KustomizationResource.Kind: {},
	}}
	return d
}

// Version is set so the test does not fetch platform-defaults.
func vibeCodingSpec(enabled bool) v1alpha1.PlatformConfigSpec {
	return v1alpha1.PlatformConfigSpec{
		GitOps: &v1alpha1.GitOpsConfig{FluxCD: &v1alpha1.FluxCDInstallConfig{Enabled: true}},
		VibeCoding: &v1alpha1.VibeCodingConfig{
			Enabled:  enabled,
			Version:  ">=1.0.0 <2.0.0",
			Settings: map[string]string{"CLUSTER_NAME": "test"},
		},
	}
}

func vibeCodingObject(t *testing.T, artifact gitops.GitOpsArtifact, kind string) map[string]any {
	t.Helper()
	for _, raw := range artifact.ExtraObjects {
		object, ok := raw.(map[string]any)
		require.True(t, ok)
		if object["kind"] == kind {
			return object
		}
	}
	t.Fatalf("no %s among the extra objects", kind)
	return nil
}

func TestVibeCodingEnabledAppliesTheSync(t *testing.T) {
	installer := &artifactInstaller{}

	result := (*reconcilerModule).reconcileVibeCoding(vibeCodingTestModule(), context.Background(), vibeCodingSpec(true), installer, updateOperation)

	assert.Nil(t, result)
	assert.Equal(t, []string{componentVibeCoding}, installer.extras)
	assert.Empty(t, installer.uninstalled)
	assert.Len(t, installer.artifact.ExtraObjects, 3)

	source := vibeCodingObject(t, installer.artifact, "OCIRepository")
	assert.Equal(t, componentVibeCoding, source["metadata"].(map[string]any)["name"])
	assert.Equal(t, ">=1.0.0 <2.0.0", source["spec"].(map[string]any)["ref"].(map[string]any)["semver"])

	root := vibeCodingObject(t, installer.artifact, "Kustomization")
	metadata := root["metadata"].(map[string]any)
	spec := root["spec"].(map[string]any)
	// "vibe-coding" is a layer of cluster/default in the same namespace.
	assert.Equal(t, "vibe-coding-root", metadata["name"])
	assert.Equal(t, "Orphan", spec["deletionPolicy"])
	assert.Equal(t, componentVibeCoding, spec["sourceRef"].(map[string]any)["name"])
}

// enabled:false uninstalls like every other component. The root's
// deletionPolicy Orphan keeps the installation itself from being pruned.
func TestVibeCodingDisabledIsUninstalled(t *testing.T) {
	installer := &artifactInstaller{}

	result := (*reconcilerModule).reconcileVibeCoding(vibeCodingTestModule(), context.Background(), vibeCodingSpec(false), installer, updateOperation)

	assert.Nil(t, result)
	assert.Equal(t, []string{componentVibeCoding}, installer.uninstalled)
	assert.Empty(t, installer.extras)
}

func TestVibeCodingDeleteOperationTouchesNothing(t *testing.T) {
	installer := &artifactInstaller{}

	result := (*reconcilerModule).reconcileVibeCoding(vibeCodingTestModule(), context.Background(), vibeCodingSpec(true), installer, deleteOperation)

	assert.Nil(t, result)
	assert.Empty(t, installer.uninstalled)
	assert.Empty(t, installer.extras)
}
