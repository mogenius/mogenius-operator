package reconciler

import (
	"reflect"

	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
)

// gitOpsWriteExternalSecretName is the Secret write.secretRef.externalSecret
// materializes into, in the operator's namespace.
func gitOpsWriteExternalSecretName(name string) string {
	return name + "-write"
}

// gitOpsRepositoryName is the name a repository entry's objects and
// credentials are derived from.
func gitOpsRepositoryName(repo v1alpha1.GitOpsRepositoryConfig) string {
	if repo.Name != "" {
		return repo.Name
	}
	return repositorySecretName(repo.URL)
}

// buildRepositoryStatus reports every declared repository with the write
// credential the operator resolved for it. Never nil, so an emptied spec
// clears the status.
func buildRepositoryStatus(gitOps *v1alpha1.GitOpsConfig, engine, engineNamespace, operatorNamespace string) []v1alpha1.GitOpsRepositoryStatus {
	statuses := []v1alpha1.GitOpsRepositoryStatus{}
	if gitOps == nil {
		return statuses
	}
	for _, repo := range gitOps.Repositories {
		name := gitOpsRepositoryName(repo)
		repositoryType := repo.Type
		if repositoryType == "" {
			repositoryType = v1alpha1.RepositoryTypePlatform
		}
		statuses = append(statuses, v1alpha1.GitOpsRepositoryStatus{
			Name:            name,
			URL:             repo.URL,
			Type:            repositoryType,
			WriteCredential: writeCredentialReference(name, repo, engine, engineNamespace, operatorNamespace),
		})
	}
	return statuses
}

// writeCredentialReference is the credential the spec declares for writes,
// nil when it declares none. A token the platform stored itself is never
// reported: the platform knows where it put it, and guessing its name here
// would contradict it.
func writeCredentialReference(name string, repo v1alpha1.GitOpsRepositoryConfig, engine, engineNamespace, operatorNamespace string) *v1alpha1.GitOpsCredentialReference {
	if repo.GitHubApp != nil {
		// The app's credential is materialized in the engine's namespace,
		// which only exists once an engine delivers.
		if engine == "" {
			return nil
		}
		return &v1alpha1.GitOpsCredentialReference{
			Kind:      v1alpha1.GitOpsCredentialKindGitHubApp,
			Namespace: engineNamespace,
			Name:      name + repositorySecretSuffix,
		}
	}

	if repo.Write == nil || repo.Write.SecretRef == nil {
		return nil
	}
	if secret := repo.Write.SecretRef.Secret; secret != nil && secret.Name != "" {
		key := secret.Key
		if key == "" {
			key = gitops.WriteSecretTokenKey
		}
		return &v1alpha1.GitOpsCredentialReference{
			Kind:      v1alpha1.GitOpsCredentialKindToken,
			Namespace: operatorNamespace,
			Name:      secret.Name,
			Key:       key,
		}
	}
	if es := repo.Write.SecretRef.ExternalSecret; es != nil {
		// externalSecretResource stores the value under the remote key's
		// name, or "token" when the reference names none.
		key := es.Key
		if key == "" {
			key = gitops.WriteSecretTokenKey
		}
		return &v1alpha1.GitOpsCredentialReference{
			Kind:      v1alpha1.GitOpsCredentialKindToken,
			Namespace: operatorNamespace,
			Name:      gitOpsWriteExternalSecretName(name),
			Key:       key,
		}
	}
	return nil
}

// repositoryStatusEqual treats an absent and an empty list as the same report.
func repositoryStatusEqual(current, desired []v1alpha1.GitOpsRepositoryStatus) bool {
	if len(current) == 0 && len(desired) == 0 {
		return true
	}
	return reflect.DeepEqual(current, desired)
}
