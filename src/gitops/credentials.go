package gitops

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

// The write credential is a contract with the platform API: it reads
// mogenius-gitops-write-<repository name> from the operator's namespace and
// commits platformconfig.yaml with the token inside. Name, keys, labels and
// annotations are the API's format.
const (
	WriteSecretPrefix = "mogenius-gitops-write-"

	WriteSecretTokenKey    = "token"
	WriteSecretUsernameKey = "username"
	WriteSecretProviderKey = "provider"

	// The repository name is how the platform finds its own entry in
	// spec.gitOps.repositories[] again; the write mode lives on the Secret so a
	// restored backup or a re-registered cluster keeps the setting.
	WriteRepositoryAnnotation    = "mogenius.com/gitops-repository"
	WriteRepositoryURLAnnotation = "mogenius.com/gitops-repository-url"
	WriteModeAnnotation          = "mogenius.com/gitops-write-mode"

	// WriteManagedByLabelKey is bare, not app.kubernetes.io/managed-by: the
	// platform API recognises its own write credentials by it.
	WriteManagedByLabelKey   = "managed-by"
	WriteManagedByLabelValue = "mogenius"

	ComponentLabelKey        = "mogenius.com/component"
	WriteComponentLabelValue = "gitops-write"

	ManagedByLabelKey    = "app.kubernetes.io/managed-by"
	ManagedByLabelValue  = "mogenius"
	PartOfLabelKey       = "app.kubernetes.io/part-of"
	PartOfLabelValue     = "mogenius"
	ComponentK8sLabelKey = "app.kubernetes.io/component"
	ComponentK8sValue    = "platform-config"

	// RepositorySecretSuffix names a repository's read credential,
	// <repository>-repository, in the engine's namespace.
	RepositorySecretSuffix = "-repository"

	// ArgoCDSecretTypeLabelKey is how Argo CD discovers repository
	// credentials: by label, with the repository URL inside the Secret.
	ArgoCDSecretTypeLabelKey   = "argocd.argoproj.io/secret-type"
	ArgoCDSecretTypeLabelValue = "repository"

	// GitHubAppLabelKey marks the credentials the operator derives from a
	// repository's GitHub App. They are owned by the PlatformConfig, not by
	// the platform's token rotation, and are removed once no repository
	// declares them anymore.
	GitHubAppLabelKey   = "mogenius.com/github-app"
	GitHubAppLabelValue = "true"
)

// Keys of a GitHub App repository credential. Argo CD and Flux share the first
// three and differ only in how they spell the Enterprise API root.
const (
	GitHubAppIDKey                      = "githubAppID"
	GitHubAppInstallationIDKey          = "githubAppInstallationID"
	GitHubAppPrivateKeyKey              = "githubAppPrivateKey"
	ArgoCDGitHubAppEnterpriseBaseURLKey = "githubAppEnterpriseBaseUrl"
	FluxGitHubAppBaseURLKey             = "githubAppBaseURL"
)

// Write modes the platform API accepts. PULL_REQUEST is its own default.
const (
	WriteModePullRequest  = "PULL_REQUEST"
	WriteModeDirectCommit = "DIRECT_COMMIT"
)

// Git providers, in the platform's spelling.
const (
	ProviderGitHub = "GIT_HUB"
	ProviderGitLab = "GIT_LAB"
	ProviderGitea  = "GITEA"
)

// DefaultWriteUsername is the platform's default; only some provider APIs use
// the username at all.
const DefaultWriteUsername = "mogenius"

// WriteSecretName returns the name the platform API reads the write credential
// under.
func WriteSecretName(repositoryName string) string {
	return WriteSecretPrefix + repositoryName
}

// WriteCredential is everything the write credential Secret carries.
type WriteCredential struct {
	Namespace      string
	RepositoryName string
	RepositoryURL  string
	Token          string
	Username       string
	Provider       string
	WriteMode      string
	// Annotations are added next to the contract's own.
	Annotations map[string]string
}

// WriteCredentialSecret is the desired write credential.
//
// Everything goes into Data rather than StringData. The platform's own secret
// validation only ever looks at data, so a Secret carrying just stringData is
// rejected as having no keys when the API updates it on the next rotation.
func WriteCredentialSecret(credential WriteCredential) *corev1.Secret {
	annotations := map[string]string{}
	for key, value := range credential.Annotations {
		annotations[key] = value
	}
	annotations[WriteRepositoryAnnotation] = credential.RepositoryName
	annotations[WriteRepositoryURLAnnotation] = credential.RepositoryURL
	annotations[WriteModeAnnotation] = credential.WriteMode

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WriteSecretName(credential.RepositoryName),
			Namespace: credential.Namespace,
			Labels: map[string]string{
				WriteManagedByLabelKey: WriteManagedByLabelValue,
				ComponentLabelKey:      WriteComponentLabelValue,
				ManagedByLabelKey:      ManagedByLabelValue,
				ComponentK8sLabelKey:   ComponentK8sValue,
				PartOfLabelKey:         PartOfLabelValue,
			},
			Annotations: annotations,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			WriteSecretTokenKey:    []byte(credential.Token),
			WriteSecretUsernameKey: []byte(credential.Username),
			WriteSecretProviderKey: []byte(credential.Provider),
		},
	}
}

// UpsertSecret creates desired, or updates an existing Secret of the same name
// in place.
//
// An existing Secret is adopted rather than refused: rotating a token is the
// same call again, and the Secret it has to rotate may have been created by the
// bootstrap chart or by the platform UI.
//
// Labels, annotations and data keys are merged, not replaced, so a key someone
// added next to ours (an Argo CD tlsClientCert, say) survives a rotation.
func UpsertSecret(ctx context.Context, client kubernetes.Interface, desired *corev1.Secret) (created bool, err error) {
	secrets := client.CoreV1().Secrets(desired.Namespace)

	_, err = secrets.Get(ctx, desired.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("get secret %q: %w", desired.Name, err)
	}
	if apierrors.IsNotFound(err) {
		if _, err = secrets.Create(ctx, desired, metav1.CreateOptions{}); err == nil {
			return true, nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return false, fmt.Errorf("create secret %q: %w", desired.Name, err)
		}
		// Lost a create race; fall through to the update path.
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := secrets.Get(ctx, desired.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Labels == nil {
			current.Labels = map[string]string{}
		}
		for key, value := range desired.Labels {
			current.Labels[key] = value
		}
		if current.Annotations == nil {
			current.Annotations = map[string]string{}
		}
		for key, value := range desired.Annotations {
			current.Annotations[key] = value
		}
		if current.Data == nil {
			current.Data = map[string][]byte{}
		}
		for key, value := range desired.Data {
			current.Data[key] = value
			// A real API server never returns stringData; clients that keep
			// it, like the fake clientset, would let it shadow the new value.
			delete(current.StringData, key)
		}
		_, err = secrets.Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return false, fmt.Errorf("update secret %q: %w", desired.Name, err)
	}
	return false, nil
}
