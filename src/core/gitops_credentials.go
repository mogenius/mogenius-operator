package core

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"mogenius-operator/src/gitops"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
)

// ╭──────────────────────────────────────╮
// │ GitOps repository credential secrets │
// ╰──────────────────────────────────────╯
//
// The two Secrets a mogenius platform repository needs, previously rendered by
// the mogenius-platform-bootstrap Helm chart and now created here from a
// command the platform API pushes over the websocket connection.
//
// They are deliberately separate objects, because they are read by different
// parties and only one of them may write:
//
//   - the read credential (<repository>-repository, in the GitOps engine's
//     namespace) is the engine's. Argo CD or Flux pulls platformconfig.yaml out
//     of the repository with it. Its shape is the engine's, not ours.
//   - the write credential (mogenius-gitops-write-<repository>, in the
//     operator's namespace) is the platform API's. Committing platformconfig.yaml
//     back needs a git provider API token, which the engine's credential cannot
//     do — for Flux it may not even be a token.
//
// The operator only ever materializes them. Committing to git stays the API's
// job: the token reaches exactly one cluster, and the platform never holds it.
//
// Name, keys, labels and annotations of the write credential are the API's
// format, not ours — it looks the credential up by name in the operator's
// namespace, so a different name produces a Secret nobody reads, and an empty
// provider is silently skipped by the reader. Both are therefore validated
// here rather than deferred to a first commit weeks later.
//
// The token must never leave this file except inside Secret.Data: not in error
// messages, not in log attributes, not in audit objects.

// The write credential's contract and the shared labels live in the gitops
// package, which the platform reconciler writes the same Secret through.
const (
	gitOpsWriteSecretPrefix      = gitops.WriteSecretPrefix
	gitOpsRepositorySecretSuffix = gitops.RepositorySecretSuffix
)

// Data keys of the two Secrets.
const (
	gitOpsWriteSecretTokenKey    = gitops.WriteSecretTokenKey
	gitOpsWriteSecretUsernameKey = gitops.WriteSecretUsernameKey
	gitOpsWriteSecretProviderKey = gitops.WriteSecretProviderKey

	gitOpsReadSecretTypeKey     = "type"
	gitOpsReadSecretURLKey      = "url"
	gitOpsReadSecretUsernameKey = "username"
	gitOpsReadSecretPasswordKey = "password"
)

const (
	gitOpsWriteRepositoryAnnotation    = gitops.WriteRepositoryAnnotation
	gitOpsWriteRepositoryURLAnnotation = gitops.WriteRepositoryURLAnnotation
	gitOpsWriteModeAnnotation          = gitops.WriteModeAnnotation
)

const (
	gitOpsWriteManagedByLabelKey   = gitops.WriteManagedByLabelKey
	gitOpsWriteManagedByLabelValue = gitops.WriteManagedByLabelValue

	gitOpsComponentLabelKey        = gitops.ComponentLabelKey
	gitOpsWriteComponentLabelValue = gitops.WriteComponentLabelValue

	gitOpsManagedByLabelKey    = gitops.ManagedByLabelKey
	gitOpsManagedByLabelValue  = gitops.ManagedByLabelValue
	gitOpsPartOfLabelKey       = gitops.PartOfLabelKey
	gitOpsPartOfLabelValue     = gitops.PartOfLabelValue
	gitOpsComponentK8sLabelKey = gitops.ComponentK8sLabelKey
	gitOpsComponentK8sValue    = gitops.ComponentK8sValue

	argoCDSecretTypeLabelKey   = gitops.ArgoCDSecretTypeLabelKey
	argoCDSecretTypeLabelValue = gitops.ArgoCDSecretTypeLabelValue
)

const (
	GitOpsWriteModePullRequest  = gitops.WriteModePullRequest
	GitOpsWriteModeDirectCommit = gitops.WriteModeDirectCommit
)

const (
	GitOpsProviderGitHub = gitops.ProviderGitHub
	GitOpsProviderGitLab = gitops.ProviderGitLab
	GitOpsProviderGitea  = gitops.ProviderGitea
)

const gitOpsWriteDefaultUsername = gitops.DefaultWriteUsername

// Default namespaces of the engines, used when the command does not name one.
const (
	gitOpsFluxDefaultNamespace   = "flux-system"
	gitOpsArgoCDDefaultNamespace = "argocd"
)

// Usernames of the read credential. x-token-auth is accepted as the username by
// GitHub, GitLab and Gitea alike; Flux's basic-auth Secret conventionally
// carries git.
const (
	gitOpsArgoCDReadUsername = "x-token-auth"
	gitOpsFluxReadUsername   = "git"
)

// GitOpsCredentialsRequest is the platform API's command payload: the
// repository and the token to materialize credentials for.
//
// Everything but the repository, its URL and the token is optional and
// defaulted the way the bootstrap chart defaulted it, so the API only has to
// send what deviates.
type GitOpsCredentialsRequest struct {
	// RepositoryName is the repository as spec.gitOps.repositories[] names it;
	// both Secret names are derived from it.
	RepositoryName string `json:"repositoryName" validate:"required"`
	// RepositoryURL is the clone URL. The git provider is derived from it
	// unless Provider says which.
	RepositoryURL string `json:"repositoryUrl" validate:"required"`
	// Token is the git provider token. Redacted in the audit log by the
	// "token" field name — do not rename without updating
	// sensitiveAuditPayloadKeys (store).
	Token string `json:"token" validate:"required"`
	// Username is stored next to the token in the write credential. Defaults
	// to mogenius.
	Username string `json:"username,omitempty"`
	// Provider overrides the URL-derived provider, for a self-hosted host
	// whose name says nothing.
	Provider string `json:"provider,omitempty" validate:"omitempty,oneof=GIT_HUB GIT_LAB GITEA"`
	// WriteMode is how the platform commits: PULL_REQUEST or DIRECT_COMMIT.
	// Defaults to PULL_REQUEST.
	WriteMode string `json:"writeMode,omitempty" validate:"omitempty,oneof=PULL_REQUEST DIRECT_COMMIT"`
	// Engine is the GitOps engine reading the repository, "flux" or "argo-cd".
	// Defaults to flux, matching the engine the charts install by default.
	Engine string `json:"engine,omitempty"`
	// EngineNamespace is where the engine runs and where its read credential
	// goes. Defaults to flux-system or argocd.
	EngineNamespace string `json:"engineNamespace,omitempty"`
}

// resolvedGitOpsCredentials is a request with every default applied and every
// value validated — the shape the two upserts work from.
type resolvedGitOpsCredentials struct {
	repositoryName  string
	repositoryURL   string
	token           string
	username        string
	provider        string
	writeMode       string
	engine          string
	engineNamespace string
}

// gitOpsWriteSecretName returns the name the platform API reads the write
// credential under.
func gitOpsWriteSecretName(repositoryName string) string {
	return gitOpsWriteSecretPrefix + repositoryName
}

// gitOpsReadSecretName returns the name the GitOps engine's credential is
// looked up under.
func gitOpsReadSecretName(repositoryName string) string {
	return repositoryName + gitOpsRepositorySecretSuffix
}

// resolveGitOpsWriteMode defaults an empty mode and rejects an unknown one. An
// unknown value is silently ignored by the platform's reader, which would leave
// the cluster on a write mode nobody chose.
func resolveGitOpsWriteMode(mode string) (string, error) {
	switch mode {
	case "":
		return GitOpsWriteModePullRequest, nil
	case GitOpsWriteModePullRequest, GitOpsWriteModeDirectCommit:
		return mode, nil
	default:
		return "", fmt.Errorf("writeMode must be %s or %s, got %q", GitOpsWriteModePullRequest, GitOpsWriteModeDirectCommit, mode)
	}
}

// resolveGitOpsEngine normalizes the engine name. Empty means Flux, the engine
// the charts install by default, so an unset field lands where the bootstrap
// did. The alternative spellings are accepted because the PlatformConfig spells
// the same engines "fluxcd" and "argocd" in spec.gitOps.
func resolveGitOpsEngine(engine string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "", gitops.EngineFlux, "fluxcd":
		return gitops.EngineFlux, nil
	case gitops.EngineArgoCD, "argocd", "argo":
		return gitops.EngineArgoCD, nil
	default:
		return "", fmt.Errorf("engine must be %s or %s, got %q", gitops.EngineFlux, gitops.EngineArgoCD, engine)
	}
}

// defaultGitOpsEngineNamespace is where each engine runs by convention.
func defaultGitOpsEngineNamespace(engine string) string {
	if engine == gitops.EngineArgoCD {
		return gitOpsArgoCDDefaultNamespace
	}
	return gitOpsFluxDefaultNamespace
}

// deriveGitOpsProvider maps a repository URL onto the platform's provider
// spelling, mirroring the host list the API itself matches on (github.,
// gitlab., gitea., codeberg.org).
//
// The host is matched when the URL has one, and the whole string otherwise, so
// an scp-style SSH remote (git@github.com:acme/platform.git) — which parses
// without a host — is still recognised.
//
// A URL that says nothing is an error, not an empty provider: a write
// credential without a provider is skipped by the platform, so the alternative
// is a cluster that looks onboarded and a first commit that cannot find a
// credential.
func deriveGitOpsProvider(repositoryURL string) (string, error) {
	haystack := strings.ToLower(repositoryURL)
	if parsed, err := url.Parse(repositoryURL); err == nil && parsed.Host != "" {
		haystack = strings.ToLower(parsed.Host)
	}

	switch {
	case strings.Contains(haystack, "github."):
		return GitOpsProviderGitHub, nil
	case strings.Contains(haystack, "gitlab."):
		return GitOpsProviderGitLab, nil
	case strings.Contains(haystack, "gitea."), strings.Contains(haystack, "codeberg.org"):
		return GitOpsProviderGitea, nil
	default:
		return "", fmt.Errorf("cannot derive the git provider of %s: set provider to %s, %s or %s",
			repositoryURL, GitOpsProviderGitHub, GitOpsProviderGitLab, GitOpsProviderGitea)
	}
}

// resolveGitOpsProvider prefers an explicit provider — the self-hosted
// installation whose host says nothing — and derives one otherwise.
func resolveGitOpsProvider(provider string, repositoryURL string) (string, error) {
	switch provider {
	case "":
		return deriveGitOpsProvider(repositoryURL)
	case GitOpsProviderGitHub, GitOpsProviderGitLab, GitOpsProviderGitea:
		return provider, nil
	default:
		return "", fmt.Errorf("provider %q cannot write files: set it to one of %s, %s, %s",
			provider, GitOpsProviderGitHub, GitOpsProviderGitLab, GitOpsProviderGitea)
	}
}

// resolveGitOpsCredentials applies every default and validates every field,
// including the two Secret names: the platform reads the write credential by
// name, so a repository name that renders an invalid one has to fail here
// rather than at the API server.
func resolveGitOpsCredentials(request GitOpsCredentialsRequest) (resolvedGitOpsCredentials, error) {
	var resolved resolvedGitOpsCredentials

	name := strings.TrimSpace(request.RepositoryName)
	if name == "" {
		return resolved, fmt.Errorf("repositoryName is required")
	}
	if request.RepositoryURL == "" {
		return resolved, fmt.Errorf("repositoryUrl is required")
	}
	if request.Token == "" {
		return resolved, fmt.Errorf("token is required")
	}

	for _, secretName := range []string{gitOpsWriteSecretName(name), gitOpsReadSecretName(name)} {
		if problems := validation.IsDNS1123Subdomain(secretName); len(problems) > 0 {
			return resolved, fmt.Errorf("repositoryName %q renders an invalid secret name %q: %s",
				name, secretName, strings.Join(problems, "; "))
		}
	}

	writeMode, err := resolveGitOpsWriteMode(request.WriteMode)
	if err != nil {
		return resolved, err
	}
	provider, err := resolveGitOpsProvider(request.Provider, request.RepositoryURL)
	if err != nil {
		return resolved, err
	}
	engine, err := resolveGitOpsEngine(request.Engine)
	if err != nil {
		return resolved, err
	}

	engineNamespace := strings.TrimSpace(request.EngineNamespace)
	if engineNamespace == "" {
		engineNamespace = defaultGitOpsEngineNamespace(engine)
	}
	if problems := validation.IsDNS1123Label(engineNamespace); len(problems) > 0 {
		return resolved, fmt.Errorf("engineNamespace %q is not a valid namespace: %s", engineNamespace, strings.Join(problems, "; "))
	}

	username := strings.TrimSpace(request.Username)
	if username == "" {
		username = gitOpsWriteDefaultUsername
	}

	return resolvedGitOpsCredentials{
		repositoryName:  name,
		repositoryURL:   request.RepositoryURL,
		token:           request.Token,
		username:        username,
		provider:        provider,
		writeMode:       writeMode,
		engine:          engine,
		engineNamespace: engineNamespace,
	}, nil
}

// gitOpsWriteCredentialSecret is the desired write credential: the Secret the
// platform API commits platformconfig.yaml with.
func gitOpsWriteCredentialSecret(namespace string, resolved resolvedGitOpsCredentials) *corev1.Secret {
	return gitops.WriteCredentialSecret(gitops.WriteCredential{
		Namespace:      namespace,
		RepositoryName: resolved.repositoryName,
		RepositoryURL:  resolved.repositoryURL,
		Token:          resolved.token,
		Username:       resolved.username,
		Provider:       resolved.provider,
		WriteMode:      resolved.writeMode,
	})
}

// gitOpsReadCredentialSecret is the desired read credential: the Secret the
// GitOps engine syncs the repository with.
//
// Its shape is the engine's, not ours. Argo CD discovers repository credentials
// by label and expects the repository URL inside the Secret; Flux expects plain
// basic-auth keys and is pointed at the Secret by name from the GitRepository.
func gitOpsReadCredentialSecret(resolved resolvedGitOpsCredentials) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gitOpsReadSecretName(resolved.repositoryName),
			Namespace: resolved.engineNamespace,
			Labels: map[string]string{
				gitOpsManagedByLabelKey:    gitOpsManagedByLabelValue,
				gitOpsComponentK8sLabelKey: gitOpsComponentK8sValue,
				gitOpsPartOfLabelKey:       gitOpsPartOfLabelValue,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			gitOpsReadSecretPasswordKey: []byte(resolved.token),
		},
	}

	if resolved.engine == gitops.EngineArgoCD {
		secret.Labels[argoCDSecretTypeLabelKey] = argoCDSecretTypeLabelValue
		secret.Data[gitOpsReadSecretTypeKey] = []byte("git")
		secret.Data[gitOpsReadSecretURLKey] = []byte(resolved.repositoryURL)
		secret.Data[gitOpsReadSecretUsernameKey] = []byte(gitOpsArgoCDReadUsername)
		return secret
	}

	secret.Data[gitOpsReadSecretUsernameKey] = []byte(gitOpsFluxReadUsername)
	return secret
}

// upsertGitOpsSecret creates desired, or adopts and updates an existing Secret
// of the same name in place: the Secret a rotation has to update may have been
// created by the bootstrap chart or by the platform UI.
func upsertGitOpsSecret(ctx context.Context, client kubernetes.Interface, desired *corev1.Secret) (created bool, err error) {
	return gitops.UpsertSecret(ctx, client, desired)
}

// UpsertGitOpsCredentials materializes both credentials for one platform
// repository and is idempotent: calling it again with a new token rotates both
// Secrets in place.
//
// The read credential goes first. It is what lets the engine sync the
// repository at all, and the write credential is only useful once there is a
// PlatformConfig in the cluster to commit changes to.
func UpsertGitOpsCredentials(
	ctx context.Context,
	client kubernetes.Interface,
	logger *slog.Logger,
	operatorNamespace string,
	request GitOpsCredentialsRequest,
) (string, error) {
	resolved, err := resolveGitOpsCredentials(request)
	if err != nil {
		return "", err
	}
	if operatorNamespace == "" {
		return "", fmt.Errorf("operator namespace is empty: cannot place the write credential where the platform reads it")
	}

	readSecret := gitOpsReadCredentialSecret(resolved)
	writeSecret := gitOpsWriteCredentialSecret(operatorNamespace, resolved)

	for _, secret := range []*corev1.Secret{readSecret, writeSecret} {
		if err := refuseGitHubAppCredential(ctx, client, secret); err != nil {
			return "", err
		}
	}

	readCreated, err := upsertGitOpsSecret(ctx, client, readSecret)
	if err != nil {
		return "", fmt.Errorf("upsert repository read credential: %w", err)
	}

	writeCreated, err := upsertGitOpsSecret(ctx, client, writeSecret)
	if err != nil {
		return "", fmt.Errorf("upsert repository write credential: %w", err)
	}

	logger.Info("materialized gitops repository credentials",
		"repository", resolved.repositoryName,
		"engine", resolved.engine,
		"readSecret", readSecret.Name,
		"readSecretNamespace", readSecret.Namespace,
		"readSecretCreated", readCreated,
		"writeSecret", writeSecret.Name,
		"writeSecretNamespace", writeSecret.Namespace,
		"writeSecretCreated", writeCreated,
		"provider", resolved.provider,
		"writeMode", resolved.writeMode)

	return fmt.Sprintf("gitops credentials for repository %q stored: %s/%s and %s/%s",
		resolved.repositoryName,
		readSecret.Namespace, readSecret.Name,
		writeSecret.Namespace, writeSecret.Name), nil
}

// refuseGitHubAppCredential fails when the Secret desired would land on is
// derived from a GitHub App. The PlatformConfig owns those: a token written
// over them is replaced on the next reconcile, and until then the engine and
// the platform authenticate with a credential nobody declared.
func refuseGitHubAppCredential(ctx context.Context, client kubernetes.Interface, desired *corev1.Secret) error {
	current, err := client.CoreV1().Secrets(desired.Namespace).Get(ctx, desired.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get secret %q: %w", desired.Name, err)
	}
	if _, ok := current.Labels[gitops.GitHubAppLabelKey]; ok {
		return fmt.Errorf("secret %s/%s is derived from the repository's githubApp in the PlatformConfig; remove githubApp there before storing a token", desired.Namespace, desired.Name)
	}
	return nil
}
