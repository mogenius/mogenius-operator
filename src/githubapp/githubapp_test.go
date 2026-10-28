package githubapp

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testKey(t *testing.T) (*rsa.PrivateKey, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return key, pemBytes
}

func TestMintInstallationTokenSendsASignedJWTAndScopesRepositories(t *testing.T) {
	key, pemBytes := testKey(t)
	expiresAt := time.Date(2026, 10, 28, 13, 0, 0, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/app/installations/42/access_tokens", r.URL.Path)

		jwt := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		parts := strings.Split(jwt, ".")
		require.Len(t, parts, 3)
		signature, err := base64.RawURLEncoding.DecodeString(parts[2])
		require.NoError(t, err)
		digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		assert.NoError(t, rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], signature))

		claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
		require.NoError(t, err)
		var claims map[string]any
		require.NoError(t, json.Unmarshal(claimsJSON, &claims))
		assert.Equal(t, "7", claims["iss"])
		assert.Less(t, claims["exp"].(float64)-claims["iat"].(float64), (10*time.Minute).Seconds()+1)

		var body map[string][]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, []string{"platform"}, body["repositories"])

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "ghs_test", "expires_at": expiresAt})
	}))
	defer server.Close()

	token, err := MintInstallationToken(t.Context(), server.Client(), TokenRequest{
		AppID:          7,
		InstallationID: 42,
		APIBaseURL:     server.URL + "/",
		PrivateKeyPEM:  pemBytes,
		Repositories:   []string{"platform"},
	})

	require.NoError(t, err)
	assert.Equal(t, "ghs_test", token.Token)
	assert.True(t, expiresAt.Equal(token.ExpiresAt))
}

func TestMintInstallationTokenReportsGitHubsRefusal(t *testing.T) {
	_, pemBytes := testKey(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer server.Close()

	_, err := MintInstallationToken(t.Context(), server.Client(), TokenRequest{
		AppID: 7, InstallationID: 42, APIBaseURL: server.URL, PrivateKeyPEM: pemBytes,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "Not Found")
}

func TestParsePrivateKeyAcceptsPKCS8(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	parsed, err := ParsePrivateKey(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	require.NoError(t, err)
	assert.True(t, key.Equal(parsed))
}

func TestParsePrivateKeyRejectsGarbage(t *testing.T) {
	_, err := ParsePrivateKey([]byte("not a key"))
	assert.Error(t, err)
}

func TestRepositoryName(t *testing.T) {
	assert.Equal(t, "platform", RepositoryName("https://github.com/acme/platform.git"))
	assert.Equal(t, "platform", RepositoryName("https://github.com/acme/platform/"))
	assert.Equal(t, "platform", RepositoryName("git@github.com:acme/platform.git"))
	assert.Equal(t, "", RepositoryName("platform"))
}
