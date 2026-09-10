package reconciler

import (
	"testing"

	"mogenius-operator/src/crds/v1alpha1"

	"github.com/stretchr/testify/assert"
)

// The incident this pins down: the engine-extras path built the GitRepository
// without any secretRef, so a private repository failed with "authentication
// required" while its credential Secret sat unused in the same namespace. Both
// delivery paths now pass the secret name through this one builder.
func TestFluxGitRepositoryObjectBindsTheNamedSecret(t *testing.T) {
	repo := v1alpha1.GitOpsRepositoryConfig{URL: "https://github.com/acme/platform.git", Revision: "main"}

	object := fluxGitRepositoryObject("platform", repo, "flux-system", "platform-repository")

	spec := object["spec"].(map[string]any)
	assert.Equal(t, map[string]any{"name": "platform-repository"}, spec["secretRef"])
}

// A public repository needs no credential, and a secretRef naming a Secret
// that is not there fails the source outright — so no name means no reference.
func TestFluxGitRepositoryObjectWithoutASecretCarriesNoReference(t *testing.T) {
	repo := v1alpha1.GitOpsRepositoryConfig{URL: "https://github.com/acme/platform.git"}

	object := fluxGitRepositoryObject("platform", repo, "flux-system", "")

	spec := object["spec"].(map[string]any)
	_, present := spec["secretRef"]
	assert.False(t, present)
	// The default branch when the spec names none.
	assert.Equal(t, map[string]any{"branch": "main"}, spec["ref"])
}
