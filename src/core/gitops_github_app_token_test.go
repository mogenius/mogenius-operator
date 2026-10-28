package core

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mogenius-operator/src/gitops"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// githubAppTokenServer is a GitHub that records the repositories each token
// was scoped to.
func githubAppTokenServer(t *testing.T, scopes *[][]string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Repositories []string `json:"repositories"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		*scopes = append(*scopes, body.Repositories)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "ghs_test", "expires_at": time.Date(2026, 10, 28, 13, 0, 0, 0, time.UTC)})
	}))
	t.Cleanup(server.Close)
	return server
}

func githubAppCredential(t *testing.T, baseURL string, labels map[string]string, extra map[string]string) *corev1.Secret {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	data := map[string][]byte{
		gitops.GitHubAppIDKey:                      []byte("7"),
		gitops.GitHubAppInstallationIDKey:          []byte("42"),
		gitops.GitHubAppPrivateKeyKey:              pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}),
		gitops.ArgoCDGitHubAppEnterpriseBaseURLKey: []byte(baseURL),
	}
	for k, v := range extra {
		data[k] = []byte(v)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "platform-repository", Namespace: "argocd", Labels: labels},
		Data:       data,
	}
}

var derivedLabels = map[string]string{gitops.GitHubAppLabelKey: gitops.GitHubAppLabelValue}

func mintForTest(t *testing.T, secret *corev1.Secret, request GitHubAppTokenRequest) (GitHubAppTokenResult, error) {
	t.Helper()
	request.Namespace, request.Name = secret.Namespace, secret.Name
	return MintGitHubAppToken(context.Background(), fake.NewClientset(secret), http.DefaultClient, discardLogger(), request)
}

func TestMintGitHubAppTokenScopesToTheRequestedRepository(t *testing.T) {
	var scopes [][]string
	server := githubAppTokenServer(t, &scopes)

	result, err := mintForTest(t, githubAppCredential(t, server.URL, derivedLabels, nil),
		GitHubAppTokenRequest{RepositoryURL: "https://github.com/acme/platform.git"})

	require.NoError(t, err)
	assert.Equal(t, "ghs_test", result.Token)
	assert.Equal(t, [][]string{{"platform"}}, scopes)
}

// An Argo CD repository Secret is bound to its url: a token for another
// repository of the same installation is refused.
func TestMintGitHubAppTokenHoldsAnArgoRepositoryToItsURL(t *testing.T) {
	var scopes [][]string
	server := githubAppTokenServer(t, &scopes)
	secret := githubAppCredential(t, server.URL,
		map[string]string{gitops.ArgoCDSecretTypeLabelKey: gitops.ArgoCDSecretTypeLabelValue},
		map[string]string{"url": "https://github.com/acme/apps.git"})

	_, err := mintForTest(t, secret, GitHubAppTokenRequest{})
	require.NoError(t, err)

	_, err = mintForTest(t, secret, GitHubAppTokenRequest{RepositoryURL: "https://github.com/acme/other.git"})
	require.Error(t, err)
	assert.Equal(t, [][]string{{"apps"}}, scopes)
}

func TestMintGitHubAppTokenRefusesSecretsThatAreNoRepositoryCredential(t *testing.T) {
	var scopes [][]string
	server := githubAppTokenServer(t, &scopes)

	_, err := mintForTest(t, githubAppCredential(t, server.URL, nil, nil),
		GitHubAppTokenRequest{RepositoryURL: "https://github.com/acme/platform.git"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a repository credential")
	assert.Empty(t, scopes)
}

func TestMintGitHubAppTokenNeedsARepositoryToScopeTo(t *testing.T) {
	var scopes [][]string
	server := githubAppTokenServer(t, &scopes)

	_, err := mintForTest(t, githubAppCredential(t, server.URL, derivedLabels, nil), GitHubAppTokenRequest{})

	require.Error(t, err)
	assert.Empty(t, scopes)
}
