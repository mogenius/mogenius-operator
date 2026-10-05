package gitops

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFluxHelmReleasesForceServerSideApply(t *testing.T) {
	artifact := moacArtifact()
	releases := map[string]*unstructured.Unstructured{
		"repository": buildFluxHelmRelease("c", artifact, nil, "flux-system"),
		"oci":        buildFluxOCIHelmRelease("c", artifact, "flux-system"),
		"moac":       buildFluxMoacHelmRelease("c", artifact, "flux-system", true),
	}
	for name, release := range releases {
		t.Run(name, func(t *testing.T) {
			install, _, _ := unstructured.NestedBool(release.Object, "spec", "install", "serverSideApply")
			upgrade, _, _ := unstructured.NestedString(release.Object, "spec", "upgrade", "serverSideApply")
			rollback, _, _ := unstructured.NestedString(release.Object, "spec", "rollback", "serverSideApply")
			assert.True(t, install)
			assert.Equal(t, "enabled", upgrade)
			assert.Equal(t, "enabled", rollback)
		})
	}
}
