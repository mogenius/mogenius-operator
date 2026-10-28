// Package githubapp mints GitHub App installation access tokens.
package githubapp

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DefaultAPIBaseURL is the REST API root of github.com.
const DefaultAPIBaseURL = "https://api.github.com"

const (
	// GitHub rejects app JWTs valid for longer than ten minutes.
	jwtLifetime = 9 * time.Minute
	// Backdated to tolerate clock skew between the cluster and GitHub.
	jwtClockSkew = 60 * time.Second
)

// InstallationToken is a short-lived token scoped to one app installation.
type InstallationToken struct {
	Token     string
	ExpiresAt time.Time
}

// TokenRequest describes the installation token to mint.
type TokenRequest struct {
	AppID          int64
	InstallationID int64
	// APIBaseURL is the REST API root; empty means github.com.
	APIBaseURL    string
	PrivateKeyPEM []byte
	// Repositories narrows the token to these repository names, without owner.
	// Empty grants every repository the installation can access.
	Repositories []string
}

// MintInstallationToken exchanges an app JWT for an installation access token.
// The token never appears in returned errors.
func MintInstallationToken(ctx context.Context, client *http.Client, request TokenRequest) (InstallationToken, error) {
	key, err := ParsePrivateKey(request.PrivateKeyPEM)
	if err != nil {
		return InstallationToken{}, err
	}
	jwt, err := appJWT(request.AppID, key, time.Now())
	if err != nil {
		return InstallationToken{}, err
	}

	var body io.Reader
	if len(request.Repositories) > 0 {
		payload, err := json.Marshal(map[string][]string{"repositories": request.Repositories})
		if err != nil {
			return InstallationToken{}, fmt.Errorf("encode token request: %w", err)
		}
		body = bytes.NewReader(payload)
	}

	baseURL := strings.TrimSuffix(request.APIBaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultAPIBaseURL
	}
	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", baseURL, request.InstallationID)

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return InstallationToken{}, fmt.Errorf("build token request: %w", err)
	}
	httpRequest.Header.Set("Accept", "application/vnd.github+json")
	httpRequest.Header.Set("Authorization", "Bearer "+jwt)
	httpRequest.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		httpRequest.Header.Set("Content-Type", "application/json")
	}

	response, err := client.Do(httpRequest)
	if err != nil {
		return InstallationToken{}, fmt.Errorf("request installation token: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusCreated {
		// The error body carries GitHub's message, never a token.
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return InstallationToken{}, fmt.Errorf("request installation token: %s: %s", response.Status, strings.TrimSpace(string(detail)))
	}

	var decoded struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return InstallationToken{}, fmt.Errorf("decode installation token response: %w", err)
	}
	if decoded.Token == "" {
		return InstallationToken{}, errors.New("installation token response carries no token")
	}
	return InstallationToken{Token: decoded.Token, ExpiresAt: decoded.ExpiresAt}, nil
}

// ParsePrivateKey reads an RSA key in PKCS#1, the format GitHub issues, or
// PKCS#8.
func ParsePrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("github app private key is not PEM encoded")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("github app private key is neither PKCS#1 nor PKCS#8")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("github app private key is not an RSA key")
	}
	return key, nil
}

func appJWT(appID int64, key *rsa.PrivateKey, now time.Time) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	claims, err := json.Marshal(map[string]any{
		"iat": now.Add(-jwtClockSkew).Unix(),
		"exp": now.Add(jwtLifetime).Unix(),
		"iss": strconv.FormatInt(appID, 10),
	})
	if err != nil {
		return "", err
	}

	encoding := base64.RawURLEncoding
	signingInput := encoding.EncodeToString(header) + "." + encoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign github app jwt: %w", err)
	}
	return signingInput + "." + encoding.EncodeToString(signature), nil
}

// RepositoryName is the repository part of an HTTPS or SCP-style clone URL,
// empty when the URL has none.
func RepositoryName(url string) string {
	trimmed := strings.TrimSuffix(strings.TrimSuffix(url, "/"), ".git")
	index := strings.LastIndexAny(trimmed, "/:")
	if index < 0 || index == len(trimmed)-1 {
		return ""
	}
	return trimmed[index+1:]
}
