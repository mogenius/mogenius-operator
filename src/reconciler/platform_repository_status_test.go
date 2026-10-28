package reconciler

import (
	"testing"

	"mogenius-operator/src/crds/v1alpha1"

	"github.com/stretchr/testify/assert"
)

// A token the platform stored itself is not reported: the platform knows where
// it put it, and the two names it uses differ from anything derivable here.
func TestBuildRepositoryStatusReportsNoCredentialForPlatformTokens(t *testing.T) {
	gitOps := &v1alpha1.GitOpsConfig{Repositories: []v1alpha1.GitOpsRepositoryConfig{
		{Name: "platform", URL: "https://github.com/acme/platform.git"},
		{Name: "apps", URL: "https://github.com/acme/apps.git", Type: v1alpha1.RepositoryTypeApplication},
	}}

	statuses := buildRepositoryStatus(gitOps, gitOpsEngineArgoCD, "argocd", "mogenius")

	assert.Equal(t, []v1alpha1.GitOpsRepositoryStatus{
		{Name: "platform", URL: "https://github.com/acme/platform.git", Type: v1alpha1.RepositoryTypePlatform},
		{Name: "apps", URL: "https://github.com/acme/apps.git", Type: v1alpha1.RepositoryTypeApplication},
	}, statuses)
}

func TestBuildRepositoryStatusReportsTheGitHubAppCredential(t *testing.T) {
	gitOps := &v1alpha1.GitOpsConfig{Repositories: []v1alpha1.GitOpsRepositoryConfig{githubAppRepo()}}

	statuses := buildRepositoryStatus(gitOps, gitOpsEngineArgoCD, "argocd", "mogenius")

	assert.Equal(t, &v1alpha1.GitOpsCredentialReference{
		Kind:      v1alpha1.GitOpsCredentialKindGitHubApp,
		Namespace: "argocd",
		Name:      "platform-repository",
	}, statuses[0].WriteCredential)

	// Without an engine nothing materializes the credential.
	assert.Nil(t, buildRepositoryStatus(gitOps, "", "", "mogenius")[0].WriteCredential)
}

func TestBuildRepositoryStatusResolvesTheDeclaredSecretRef(t *testing.T) {
	repo := v1alpha1.GitOpsRepositoryConfig{Name: "platform", URL: "https://github.com/acme/platform.git"}

	repo.Write = &v1alpha1.GitOpsWriteConfig{SecretRef: &v1alpha1.GenericSecretReference{
		Secret: &v1alpha1.SecretReference{Name: "github-token"},
	}}
	assert.Equal(t, &v1alpha1.GitOpsCredentialReference{
		Kind: v1alpha1.GitOpsCredentialKindToken, Namespace: "mogenius", Name: "github-token", Key: "token",
	}, writeCredentialReference("platform", repo, gitOpsEngineFlux, "flux-system", "mogenius"))

	repo.Write.SecretRef = &v1alpha1.GenericSecretReference{
		ExternalSecret: &v1alpha1.ExternalSecret{Path: "git/platform", Key: "pat"},
	}
	assert.Equal(t, &v1alpha1.GitOpsCredentialReference{
		Kind: v1alpha1.GitOpsCredentialKindToken, Namespace: "mogenius", Name: "platform-write", Key: "pat",
	}, writeCredentialReference("platform", repo, gitOpsEngineFlux, "flux-system", "mogenius"))
}

func TestBuildRepositoryStatusIsNeverNil(t *testing.T) {
	assert.NotNil(t, buildRepositoryStatus(nil, "", "", "mogenius"))
	assert.True(t, repositoryStatusEqual(nil, []v1alpha1.GitOpsRepositoryStatus{}))
}
