package core

import (
	"testing"

	"mogenius-operator/src/gitops"

	"github.com/stretchr/testify/assert"
)

func TestPlatformBootstrapConfigured(t *testing.T) {
	t.Parallel()

	assert.True(t, PlatformBootstrap{RepositoryURL: "https://github.com/acme/platform.git", Path: "platform"}.configured())

	// Both are needed: a url without a directory gives the engine nothing to
	// sync, and a directory without a url points nowhere.
	assert.False(t, PlatformBootstrap{Path: "platform"}.configured())
	assert.False(t, PlatformBootstrap{RepositoryURL: "https://github.com/acme/platform.git"}.configured())
	assert.False(t, PlatformBootstrap{}.configured())
}

func TestPlatformBootstrapGitOpsSpec(t *testing.T) {
	t.Parallel()

	bootstrap := PlatformBootstrap{
		RepositoryURL: "https://github.com/acme/platform.git",
		Branch:        "main",
		Path:          "platform",
		Engine:        gitops.EngineArgoCD,
	}

	spec := bootstrap.gitOpsSpec()

	/**
	 * Declared but not enabled. Helm installed the engine, so enabling it would
	 * have the operator install a second one next to that release -- while
	 * declaring it is still what tells the platform which engine runs here.
	 */
	argocd, ok := spec["argocd"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, false, argocd["enabled"])
	assert.NotContains(t, spec, "fluxcd")

	repositories, ok := spec["repositories"].([]any)
	assert.True(t, ok)
	assert.Len(t, repositories, 1)

	repository := repositories[0].(map[string]any)
	assert.Equal(t, "https://github.com/acme/platform.git", repository["url"])
	// The CRD calls the branch "revision".
	assert.Equal(t, "main", repository["revision"])
	assert.Equal(t, "platform", repository["path"])
	assert.Equal(t, DEFAULT_PLATFORM_CONFIG_NAME, repository["name"])
}

func TestPlatformBootstrapGitOpsSpecFlux(t *testing.T) {
	t.Parallel()

	spec := PlatformBootstrap{
		RepositoryURL: "https://github.com/acme/platform.git",
		Branch:        "main",
		Path:          "platform",
		Engine:        gitops.EngineFlux,
	}.gitOpsSpec()

	fluxcd, ok := spec["fluxcd"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, false, fluxcd["enabled"])
	assert.NotContains(t, spec, "argocd")
}

// An unrecognised engine falls back to Flux rather than declaring nothing: a
// spec with no engine block leaves the platform unable to tell what runs, and
// Flux is what the charts install when nobody chose.
func TestPlatformBootstrapGitOpsSpecUnknownEngineFallsBackToFlux(t *testing.T) {
	t.Parallel()

	for _, engine := range []string{"", "something-else"} {
		spec := PlatformBootstrap{
			RepositoryURL: "https://github.com/acme/platform.git",
			Path:          "platform",
			Engine:        engine,
		}.gitOpsSpec()

		assert.Contains(t, spec, "fluxcd", "engine %q", engine)
		assert.NotContains(t, spec, "argocd", "engine %q", engine)
	}
}
