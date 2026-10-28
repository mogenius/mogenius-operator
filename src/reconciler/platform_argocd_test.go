package reconciler

import (
	"testing"

	"mogenius-operator/src/crds/v1alpha1"

	"github.com/stretchr/testify/assert"
)

func TestArgoSyncObjectSkipsApplicationRepositories(t *testing.T) {
	repo := v1alpha1.GitOpsRepositoryConfig{
		Name: "workloads",
		URL:  "https://github.com/acme/workloads.git",
		Type: v1alpha1.RepositoryTypeApplication,
		Path: "apps",
	}

	assert.Nil(t, argoSyncObject("workloads", repo, "argocd", "default"))
}

func TestArgoSyncObjectSyncsPlatformRepositories(t *testing.T) {
	repo := v1alpha1.GitOpsRepositoryConfig{
		Name: "platform",
		URL:  "https://github.com/acme/platform.git",
		Path: "mogenius",
	}

	assert.Equal(t, "Application", argoSyncObject("platform", repo, "argocd", "default")["kind"])

	repo.Type = v1alpha1.RepositoryTypePlatform
	repo.Path = "clusters/**"
	assert.Equal(t, "ApplicationSet", argoSyncObject("platform", repo, "argocd", "default")["kind"])

	repo.Path = ""
	assert.Nil(t, argoSyncObject("platform", repo, "argocd", "default"))
}
