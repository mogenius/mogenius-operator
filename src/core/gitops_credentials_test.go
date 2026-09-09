package core

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"mogenius-operator/src/gitops"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// These tests are the Go port of the bootstrap chart's
// unittests/write-credential-secret_test.yaml, which is the contract with the
// platform API: it reads the write credential by name from the operator's
// namespace, keyed and labelled exactly this way.

const (
	testRepositoryURL = "https://github.com/acme/platform.git"
	testRepository    = "platform"
	testEngineNs      = "flux-system"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testCredentialsRequest is the minimal valid command: repository, url, token.
func testCredentialsRequest() GitOpsCredentialsRequest {
	return GitOpsCredentialsRequest{
		RepositoryName: testRepository,
		RepositoryURL:  testRepositoryURL,
		Token:          "ghp_test",
	}
}

func upsertForTest(t *testing.T, client *fake.Clientset, request GitOpsCredentialsRequest) string {
	t.Helper()
	result, err := UpsertGitOpsCredentials(context.Background(), client, discardLogger(), testNamespace, request)
	require.NoError(t, err)
	return result
}

func writeSecretOf(t *testing.T, client *fake.Clientset, repositoryName string) *corev1.Secret {
	t.Helper()
	secret, err := client.CoreV1().Secrets(testNamespace).Get(context.Background(), gitOpsWriteSecretName(repositoryName), metav1.GetOptions{})
	require.NoError(t, err)
	return secret
}

func readSecretOf(t *testing.T, client *fake.Clientset, namespace string, repositoryName string) *corev1.Secret {
	t.Helper()
	secret, err := client.CoreV1().Secrets(namespace).Get(context.Background(), gitOpsReadSecretName(repositoryName), metav1.GetOptions{})
	require.NoError(t, err)
	return secret
}

// ╭──────────────────────────────╮
// │ names, namespaces, and keys  │
// ╰──────────────────────────────╯

func TestGitOpsSecretNames(t *testing.T) {
	// Both names are contracts: the platform API reads the write credential
	// under this exact name, and the platform repository reconciler looks the
	// read credential up under the -repository suffix.
	assert.Equal(t, "mogenius-gitops-write-platform", gitOpsWriteSecretName("platform"))
	assert.Equal(t, "platform-repository", gitOpsReadSecretName("platform"))
	assert.Equal(t, "mogenius-gitops-write-platform-eu", gitOpsWriteSecretName("platform-eu"))
}

// "creates the write credential when a token is given"
func TestUpsertGitOpsCredentialsCreatesWriteCredential(t *testing.T) {
	client := fake.NewClientset()
	upsertForTest(t, client, testCredentialsRequest())

	secret := writeSecretOf(t, client, testRepository)
	assert.Equal(t, "mogenius-gitops-write-platform", secret.Name)
	// The operator's namespace, not the engine's: getting this wrong produces
	// a Secret nobody looks for.
	assert.Equal(t, testNamespace, secret.Namespace)
	assert.Equal(t, corev1.SecretTypeOpaque, secret.Type)
	assert.Equal(t, []byte("ghp_test"), secret.Data[gitOpsWriteSecretTokenKey])
	assert.Equal(t, []byte("mogenius"), secret.Data[gitOpsWriteSecretUsernameKey])
	assert.Equal(t, []byte(GitOpsProviderGitHub), secret.Data[gitOpsWriteSecretProviderKey])
	// data, never stringData: the platform's secret validation only looks at
	// data and rejects a Secret carrying just stringData as having no keys.
	assert.Empty(t, secret.StringData)
}

// "labels the credential the way the platform recognises it"
func TestUpsertGitOpsCredentialsWriteLabels(t *testing.T) {
	client := fake.NewClientset()
	upsertForTest(t, client, testCredentialsRequest())

	secret := writeSecretOf(t, client, testRepository)
	assert.Equal(t, "mogenius", secret.Labels[gitOpsWriteManagedByLabelKey])
	assert.Equal(t, "gitops-write", secret.Labels[gitOpsComponentLabelKey])
	assert.Equal(t, "mogenius", secret.Labels[gitOpsManagedByLabelKey])
	assert.Equal(t, "platform-config", secret.Labels[gitOpsComponentK8sLabelKey])
	assert.Equal(t, "mogenius", secret.Labels[gitOpsPartOfLabelKey])
}

// "annotates the repository, its url and the write mode"
func TestUpsertGitOpsCredentialsWriteAnnotations(t *testing.T) {
	client := fake.NewClientset()
	upsertForTest(t, client, testCredentialsRequest())

	secret := writeSecretOf(t, client, testRepository)
	assert.Equal(t, testRepository, secret.Annotations[gitOpsWriteRepositoryAnnotation])
	assert.Equal(t, testRepositoryURL, secret.Annotations[gitOpsWriteRepositoryURLAnnotation])
	assert.Equal(t, GitOpsWriteModePullRequest, secret.Annotations[gitOpsWriteModeAnnotation])
}

// "writes directly when asked to"
func TestUpsertGitOpsCredentialsDirectCommitMode(t *testing.T) {
	client := fake.NewClientset()
	request := testCredentialsRequest()
	request.WriteMode = GitOpsWriteModeDirectCommit
	upsertForTest(t, client, request)

	assert.Equal(t, GitOpsWriteModeDirectCommit,
		writeSecretOf(t, client, testRepository).Annotations[gitOpsWriteModeAnnotation])
}

// "follows the repository name into the secret name"
func TestUpsertGitOpsCredentialsFollowsRepositoryName(t *testing.T) {
	client := fake.NewClientset()
	request := testCredentialsRequest()
	request.RepositoryName = "platform-eu"
	upsertForTest(t, client, request)

	secret := writeSecretOf(t, client, "platform-eu")
	assert.Equal(t, "mogenius-gitops-write-platform-eu", secret.Name)
	assert.Equal(t, "platform-eu", secret.Annotations[gitOpsWriteRepositoryAnnotation])
	assert.Equal(t, "platform-eu-repository", readSecretOf(t, client, testEngineNs, "platform-eu").Name)
}

// "honors a custom operator namespace"
func TestUpsertGitOpsCredentialsHonorsOperatorNamespace(t *testing.T) {
	client := fake.NewClientset()
	_, err := UpsertGitOpsCredentials(context.Background(), client, discardLogger(), "mogenius-system", testCredentialsRequest())
	require.NoError(t, err)

	secret, err := client.CoreV1().Secrets("mogenius-system").Get(context.Background(), gitOpsWriteSecretName(testRepository), metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "mogenius-system", secret.Namespace)

	// An empty operator namespace is refused rather than defaulted: the write
	// credential would land nowhere the platform reads it.
	_, err = UpsertGitOpsCredentials(context.Background(), fake.NewClientset(), discardLogger(), "", testCredentialsRequest())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "operator namespace is empty")
}

// "stores a custom username"
func TestUpsertGitOpsCredentialsCustomUsername(t *testing.T) {
	client := fake.NewClientset()
	request := testCredentialsRequest()
	request.Username = "mo-bot"
	upsertForTest(t, client, request)

	assert.Equal(t, []byte("mo-bot"), writeSecretOf(t, client, testRepository).Data[gitOpsWriteSecretUsernameKey])
}

// "is created for Argo CD just the same" — the write credential does not move
// with the engine; only the read credential does.
func TestUpsertGitOpsCredentialsWriteCredentialIsEngineIndependent(t *testing.T) {
	client := fake.NewClientset()
	request := testCredentialsRequest()
	request.Engine = gitops.EngineArgoCD
	upsertForTest(t, client, request)

	secret := writeSecretOf(t, client, testRepository)
	assert.Equal(t, "mogenius-gitops-write-platform", secret.Name)
	assert.Equal(t, testNamespace, secret.Namespace)
}

// ╭──────────────────╮
// │ the write mode   │
// ╰──────────────────╯

func TestResolveGitOpsWriteMode(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		want    string
		wantErr string
	}{
		{name: "empty defaults to the reviewable mode", mode: "", want: GitOpsWriteModePullRequest},
		{name: "pull request", mode: GitOpsWriteModePullRequest, want: GitOpsWriteModePullRequest},
		{name: "direct commit", mode: GitOpsWriteModeDirectCommit, want: GitOpsWriteModeDirectCommit},
		// "refuses an unknown write mode": silently ignored by the platform's
		// reader, which would leave the cluster on a mode nobody chose.
		{name: "unknown mode is refused", mode: "YOLO", wantErr: `writeMode must be PULL_REQUEST or DIRECT_COMMIT, got "YOLO"`},
		{name: "lowercase is not accepted", mode: "pull_request", wantErr: "writeMode must be"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveGitOpsWriteMode(tt.mode)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ╭───────────────────╮
// │ the git provider  │
// ╰───────────────────╯

func TestResolveGitOpsProvider(t *testing.T) {
	tests := []struct {
		name          string
		provider      string
		repositoryURL string
		want          string
		wantErr       string
	}{
		{name: "derives GIT_HUB from a github url", repositoryURL: "https://github.com/acme/platform.git", want: GitOpsProviderGitHub},
		{name: "derives GIT_LAB from a gitlab url", repositoryURL: "https://gitlab.com/acme/platform.git", want: GitOpsProviderGitLab},
		{name: "derives GITEA from a self-hosted gitea url", repositoryURL: "https://gitea.acme.internal/acme/platform.git", want: GitOpsProviderGitea},
		{name: "derives GITEA from codeberg", repositoryURL: "https://codeberg.org/acme/platform.git", want: GitOpsProviderGitea},
		{name: "derives from an scp-style ssh remote", repositoryURL: "git@github.com:acme/platform.git", want: GitOpsProviderGitHub},
		{name: "derives from a self-hosted gitlab host", repositoryURL: "https://gitlab.acme.internal/acme/platform.git", want: GitOpsProviderGitLab},
		{name: "is case insensitive", repositoryURL: "https://GitHub.com/acme/platform.git", want: GitOpsProviderGitHub},
		// "refuses a url whose provider cannot be derived": a Secret with an
		// empty provider is silently skipped by the platform, so failing here
		// beats a first commit weeks later that finds no credential.
		{
			name:          "refuses a url whose provider cannot be derived",
			repositoryURL: "https://git.acme.internal/acme/platform.git",
			wantErr:       "cannot derive the git provider of https://git.acme.internal/acme/platform.git",
		},
		// "takes an explicit provider for a host that says nothing"
		{
			name:          "explicit provider wins for a host that says nothing",
			provider:      GitOpsProviderGitea,
			repositoryURL: "https://git.acme.internal/acme/platform.git",
			want:          GitOpsProviderGitea,
		},
		{
			name:          "explicit provider overrides the derived one",
			provider:      GitOpsProviderGitea,
			repositoryURL: "https://github.com/acme/platform.git",
			want:          GitOpsProviderGitea,
		},
		// "refuses a provider that cannot write files"
		{
			name:          "refuses a provider that cannot write files",
			provider:      "BITBUCKET",
			repositoryURL: testRepositoryURL,
			wantErr:       `provider "BITBUCKET" cannot write files`,
		},
		{
			name:          "refuses a repository path that only looks like a host",
			repositoryURL: "https://git.acme.internal/acme/github.mirror.git",
			wantErr:       "cannot derive the git provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveGitOpsProvider(tt.provider, tt.repositoryURL)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// A missing provider makes the platform silently skip the credential, so the
// command has to fail instead of writing one.
func TestUpsertGitOpsCredentialsRefusesUndeducibleProvider(t *testing.T) {
	client := fake.NewClientset()
	request := testCredentialsRequest()
	request.RepositoryURL = "https://git.acme.internal/acme/platform.git"

	_, err := UpsertGitOpsCredentials(context.Background(), client, discardLogger(), testNamespace, request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot derive the git provider")

	// Nothing at all was written: neither credential is useful without the
	// other, and a half-onboarded repository is harder to diagnose.
	secrets, listErr := client.CoreV1().Secrets(testNamespace).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, listErr)
	assert.Empty(t, secrets.Items)
}

// ╭──────────────────────────╮
// │ the engine's credential  │
// ╰──────────────────────────╯

func TestUpsertGitOpsCredentialsFluxReadCredential(t *testing.T) {
	client := fake.NewClientset()
	upsertForTest(t, client, testCredentialsRequest())

	secret := readSecretOf(t, client, testEngineNs, testRepository)
	assert.Equal(t, "platform-repository", secret.Name)
	// Flux runs in flux-system by default, and its credential goes with it.
	assert.Equal(t, "flux-system", secret.Namespace)
	assert.Equal(t, corev1.SecretTypeOpaque, secret.Type)
	// Plain basic auth: Flux is pointed at the Secret by name and reads no url
	// from it.
	assert.Equal(t, []byte("git"), secret.Data[gitOpsReadSecretUsernameKey])
	assert.Equal(t, []byte("ghp_test"), secret.Data[gitOpsReadSecretPasswordKey])
	assert.NotContains(t, secret.Data, gitOpsReadSecretURLKey)
	assert.NotContains(t, secret.Data, gitOpsReadSecretTypeKey)
	assert.NotContains(t, secret.Labels, argoCDSecretTypeLabelKey, "the Argo CD discovery label must not leak into a Flux secret")
}

func TestUpsertGitOpsCredentialsArgoCDReadCredential(t *testing.T) {
	client := fake.NewClientset()
	request := testCredentialsRequest()
	request.Engine = gitops.EngineArgoCD
	upsertForTest(t, client, request)

	secret := readSecretOf(t, client, "argocd", request.RepositoryName)
	assert.Equal(t, "platform-repository", secret.Name)
	assert.Equal(t, "argocd", secret.Namespace)
	// Argo CD discovers repository credentials by label and expects the
	// repository url inside the Secret.
	assert.Equal(t, argoCDSecretTypeLabelValue, secret.Labels[argoCDSecretTypeLabelKey])
	assert.Equal(t, []byte("git"), secret.Data[gitOpsReadSecretTypeKey])
	assert.Equal(t, []byte(testRepositoryURL), secret.Data[gitOpsReadSecretURLKey])
	// x-token-auth is accepted as the username by GitHub, GitLab and Gitea alike.
	assert.Equal(t, []byte("x-token-auth"), secret.Data[gitOpsReadSecretUsernameKey])
	assert.Equal(t, []byte("ghp_test"), secret.Data[gitOpsReadSecretPasswordKey])
}

func TestUpsertGitOpsCredentialsHonorsExplicitEngineNamespace(t *testing.T) {
	client := fake.NewClientset()
	request := testCredentialsRequest()
	request.Engine = gitops.EngineArgoCD
	request.EngineNamespace = "gitops"
	upsertForTest(t, client, request)

	assert.Equal(t, "gitops", readSecretOf(t, client, "gitops", testRepository).Namespace)
	// The write credential stays in the operator's namespace regardless.
	assert.Equal(t, testNamespace, writeSecretOf(t, client, testRepository).Namespace)
}

func TestResolveGitOpsEngine(t *testing.T) {
	tests := []struct {
		name      string
		engine    string
		want      string
		wantNs    string
		wantError bool
	}{
		{name: "empty defaults to flux", engine: "", want: gitops.EngineFlux, wantNs: "flux-system"},
		{name: "flux", engine: gitops.EngineFlux, want: gitops.EngineFlux, wantNs: "flux-system"},
		// The PlatformConfig spells the same engines fluxcd/argocd.
		{name: "fluxcd spelling", engine: "fluxcd", want: gitops.EngineFlux, wantNs: "flux-system"},
		{name: "argo-cd", engine: gitops.EngineArgoCD, want: gitops.EngineArgoCD, wantNs: "argocd"},
		{name: "argocd spelling", engine: "argocd", want: gitops.EngineArgoCD, wantNs: "argocd"},
		{name: "mixed case", engine: "Argo-CD", want: gitops.EngineArgoCD, wantNs: "argocd"},
		{name: "unknown engine is refused", engine: "jenkins", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveGitOpsEngine(tt.engine)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantNs, defaultGitOpsEngineNamespace(got))
		})
	}
}

// ╭─────────────────────────╮
// │ required input          │
// ╰─────────────────────────╯

func TestResolveGitOpsCredentialsRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*GitOpsCredentialsRequest)
		wantErr string
	}{
		{name: "missing repository name", mutate: func(r *GitOpsCredentialsRequest) { r.RepositoryName = "" }, wantErr: "repositoryName is required"},
		{name: "blank repository name", mutate: func(r *GitOpsCredentialsRequest) { r.RepositoryName = "   " }, wantErr: "repositoryName is required"},
		{name: "missing url", mutate: func(r *GitOpsCredentialsRequest) { r.RepositoryURL = "" }, wantErr: "repositoryUrl is required"},
		{name: "missing token", mutate: func(r *GitOpsCredentialsRequest) { r.Token = "" }, wantErr: "token is required"},
		{
			name:    "repository name that renders an invalid secret name",
			mutate:  func(r *GitOpsCredentialsRequest) { r.RepositoryName = "Platform_EU" },
			wantErr: "renders an invalid secret name",
		},
		{
			name:    "repository name too long for the secret name",
			mutate:  func(r *GitOpsCredentialsRequest) { r.RepositoryName = strings.Repeat("a", 253) },
			wantErr: "renders an invalid secret name",
		},
		{
			name:    "invalid engine namespace",
			mutate:  func(r *GitOpsCredentialsRequest) { r.EngineNamespace = "Flux System" },
			wantErr: "is not a valid namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := testCredentialsRequest()
			tt.mutate(&request)
			_, err := resolveGitOpsCredentials(request)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.NotContains(t, err.Error(), "ghp_test", "errors must not leak the token")
		})
	}
}

func TestResolveGitOpsCredentialsTrimsAndDefaults(t *testing.T) {
	request := testCredentialsRequest()
	request.RepositoryName = "  platform  "
	request.Username = "  "
	request.EngineNamespace = "  "

	resolved, err := resolveGitOpsCredentials(request)
	require.NoError(t, err)
	assert.Equal(t, "platform", resolved.repositoryName)
	assert.Equal(t, gitOpsWriteDefaultUsername, resolved.username)
	assert.Equal(t, "flux-system", resolved.engineNamespace)
	assert.Equal(t, GitOpsWriteModePullRequest, resolved.writeMode)
	assert.Equal(t, GitOpsProviderGitHub, resolved.provider)
}

// ╭──────────────────────────────╮
// │ rotation and idempotency     │
// ╰──────────────────────────────╯

// Rotating a token is the same command again. It must update in place, not
// error on the Secret that is already there.
func TestUpsertGitOpsCredentialsRotatesInPlace(t *testing.T) {
	client := fake.NewClientset()
	upsertForTest(t, client, testCredentialsRequest())

	rotated := testCredentialsRequest()
	rotated.Token = "ghp_rotated"
	rotated.WriteMode = GitOpsWriteModeDirectCommit
	rotated.Username = "mo-bot"
	upsertForTest(t, client, rotated)

	write := writeSecretOf(t, client, testRepository)
	assert.Equal(t, []byte("ghp_rotated"), write.Data[gitOpsWriteSecretTokenKey])
	assert.Equal(t, []byte("mo-bot"), write.Data[gitOpsWriteSecretUsernameKey])
	assert.Equal(t, GitOpsWriteModeDirectCommit, write.Annotations[gitOpsWriteModeAnnotation])

	read := readSecretOf(t, client, testEngineNs, testRepository)
	assert.Equal(t, []byte("ghp_rotated"), read.Data[gitOpsReadSecretPasswordKey])

	// Exactly one Secret per namespace — an upsert must not fan out copies.
	writeSecrets, err := client.CoreV1().Secrets(testNamespace).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, writeSecrets.Items, 1)
	readSecrets, err := client.CoreV1().Secrets(testEngineNs).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, readSecrets.Items, 1)
}

// A cluster onboarded by the bootstrap chart has Secrets labelled
// mogenius-platform-bootstrap. Rotation has to adopt those, not refuse them.
func TestUpsertGitOpsCredentialsAdoptsChartCreatedSecret(t *testing.T) {
	client := fake.NewClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gitOpsWriteSecretName(testRepository),
			Namespace: testNamespace,
			Labels: map[string]string{
				gitOpsWriteManagedByLabelKey: "mogenius",
				gitOpsManagedByLabelKey:      "mogenius-platform-bootstrap",
			},
			Annotations: map[string]string{gitOpsWriteModeAnnotation: GitOpsWriteModePullRequest},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{gitOpsWriteSecretTokenKey: []byte("ghp_old")},
	})

	rotated := testCredentialsRequest()
	rotated.Token = "ghp_new"
	upsertForTest(t, client, rotated)

	secret := writeSecretOf(t, client, testRepository)
	assert.Equal(t, []byte("ghp_new"), secret.Data[gitOpsWriteSecretTokenKey])
	// The chart never wrote these; the upsert has to fill them in.
	assert.Equal(t, gitOpsWriteComponentLabelValue, secret.Labels[gitOpsComponentLabelKey])
	assert.Equal(t, gitOpsManagedByLabelValue, secret.Labels[gitOpsManagedByLabelKey])
	assert.Equal(t, testRepositoryURL, secret.Annotations[gitOpsWriteRepositoryURLAnnotation])
	assert.Equal(t, []byte(GitOpsProviderGitHub), secret.Data[gitOpsWriteSecretProviderKey])
}

// Keys next to the ones we manage survive a rotation: an Argo CD
// tlsClientCertData an operator added by hand must not be dropped.
func TestUpsertGitOpsCredentialsPreservesForeignKeys(t *testing.T) {
	client := fake.NewClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gitOpsReadSecretName(testRepository),
			Namespace: testEngineNs,
		},
		Data: map[string][]byte{"tlsClientCertData": []byte("cert")},
	})

	upsertForTest(t, client, testCredentialsRequest())

	secret := readSecretOf(t, client, testEngineNs, testRepository)
	assert.Equal(t, []byte("cert"), secret.Data["tlsClientCertData"])
	assert.Equal(t, []byte("ghp_test"), secret.Data[gitOpsReadSecretPasswordKey])
}

// A Secret written with stringData would have the old value shadow the new one,
// because stringData wins over data on the API server.
func TestUpsertGitOpsCredentialsClearsShadowingStringData(t *testing.T) {
	client := fake.NewClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gitOpsWriteSecretName(testRepository),
			Namespace: testNamespace,
		},
		StringData: map[string]string{gitOpsWriteSecretTokenKey: "ghp_old"},
	})

	rotated := testCredentialsRequest()
	rotated.Token = "ghp_new"
	upsertForTest(t, client, rotated)

	secret := writeSecretOf(t, client, testRepository)
	assert.NotContains(t, secret.StringData, gitOpsWriteSecretTokenKey)
	assert.Equal(t, []byte("ghp_new"), secret.Data[gitOpsWriteSecretTokenKey])
}

// A concurrent create (two replicas, or a retried command) must land on the
// update path rather than failing the command.
func TestUpsertGitOpsSecretToleratesCreateRace(t *testing.T) {
	client := fake.NewClientset()
	desired := gitOpsWriteCredentialSecret(testNamespace, resolvedGitOpsCredentials{
		repositoryName: testRepository,
		repositoryURL:  testRepositoryURL,
		token:          "ghp_test",
		username:       gitOpsWriteDefaultUsername,
		provider:       GitOpsProviderGitHub,
		writeMode:      GitOpsWriteModePullRequest,
	})

	// Get says NotFound, Create then loses the race with another writer.
	client.PrependReactor("create", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		client.ReactionChain = client.ReactionChain[1:]
		require.NoError(t, client.Tracker().Add(desired.DeepCopy()))
		return true, nil, apierrors.NewAlreadyExists(schema.GroupResource{Resource: "secrets"}, desired.Name)
	})

	created, err := upsertGitOpsSecret(context.Background(), client, desired)
	require.NoError(t, err)
	assert.False(t, created, "the loser of the race did not create it")
	assert.Equal(t, []byte("ghp_test"), writeSecretOf(t, client, testRepository).Data[gitOpsWriteSecretTokenKey])
}

func TestUpsertGitOpsSecretWrapsGetError(t *testing.T) {
	client := fake.NewClientset()
	client.PrependReactor("get", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("api server is down")
	})

	_, err := upsertGitOpsSecret(context.Background(), client, gitOpsWriteCredentialSecret(testNamespace, resolvedGitOpsCredentials{
		repositoryName: testRepository,
		token:          "ghp_test",
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api server is down")
	assert.Contains(t, err.Error(), gitOpsWriteSecretName(testRepository))
}

// A failure on the write credential must be reported, not swallowed — the
// engine could sync while the platform silently could not commit.
func TestUpsertGitOpsCredentialsReportsWriteCredentialFailure(t *testing.T) {
	client := fake.NewClientset()
	client.PrependReactor("create", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		create, ok := action.(k8stesting.CreateAction)
		require.True(t, ok)
		secret, ok := create.GetObject().(*corev1.Secret)
		require.True(t, ok)
		if secret.Name == gitOpsWriteSecretName(testRepository) {
			return true, nil, fmt.Errorf("forbidden")
		}
		return false, nil, nil
	})

	_, err := UpsertGitOpsCredentials(context.Background(), client, discardLogger(), testNamespace, testCredentialsRequest())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upsert repository write credential")
	assert.NotContains(t, err.Error(), "ghp_test", "errors must not leak the token")
}

// The result string names both Secrets, so an onboarding log says where the
// credentials landed.
func TestUpsertGitOpsCredentialsResultNamesBothSecrets(t *testing.T) {
	client := fake.NewClientset()
	result := upsertForTest(t, client, testCredentialsRequest())

	assert.Contains(t, result, "mogenius/mogenius-gitops-write-platform")
	assert.Contains(t, result, "flux-system/platform-repository")
	assert.NotContains(t, result, "ghp_test")
}
