package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mogenius-operator/src/githubapp"
	"mogenius-operator/src/gitops"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ╭─────────────────────────────────────╮
// │ GitHub App installation tokens      │
// ╰─────────────────────────────────────╯
//
// The platform API commits to a repository that authenticates as a GitHub App
// with a token it requests here, right before the commit. The private key
// stays in the cluster and no token is stored anywhere: each one lives for an
// hour and is scoped to the one repository the commit targets.
//
// The token must never leave this file except in the returned result: not in
// error messages, not in log attributes, not in audit objects.

var githubAppTokenHTTPClient = &http.Client{Timeout: 15 * time.Second}

// GitHubAppTokenRequest names a GitHub App credential Secret in the cluster:
// the one PlatformConfig status.repositories[].writeCredential reports, or an
// Argo CD repository registered with GitHub App credentials.
type GitHubAppTokenRequest struct {
	Namespace string `json:"namespace" validate:"required"`
	Name      string `json:"name" validate:"required"`
	// RepositoryURL is the repository the token is scoped to. Required unless
	// the Secret carries its url, as Argo CD repository Secrets do; when both
	// are present they have to match.
	RepositoryURL string `json:"repositoryUrl,omitempty"`
}

// GitHubAppTokenResult is a minted installation token.
type GitHubAppTokenResult struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// MintGitHubAppToken mints an installation token from a GitHub App credential
// Secret.
//
// Only Secrets that are repository credentials qualify: ones the operator
// derived from a PlatformConfig githubApp, or Argo CD repository Secrets. Any
// other Secret in the cluster is refused, so the command cannot be pointed at
// unrelated key material.
func MintGitHubAppToken(
	ctx context.Context,
	client kubernetes.Interface,
	httpClient *http.Client,
	logger *slog.Logger,
	request GitHubAppTokenRequest,
) (GitHubAppTokenResult, error) {
	secret, err := client.CoreV1().Secrets(request.Namespace).Get(ctx, request.Name, metav1.GetOptions{})
	if err != nil {
		return GitHubAppTokenResult{}, fmt.Errorf("read github app credential %s/%s: %w", request.Namespace, request.Name, err)
	}
	_, derived := secret.Labels[gitops.GitHubAppLabelKey]
	argoRepository := secret.Labels[gitops.ArgoCDSecretTypeLabelKey] == gitops.ArgoCDSecretTypeLabelValue
	if !derived && !argoRepository {
		return GitHubAppTokenResult{}, fmt.Errorf("secret %s/%s is not a repository credential", request.Namespace, request.Name)
	}

	appID, err := githubAppNumber(secret.Data, gitops.GitHubAppIDKey)
	if err != nil {
		return GitHubAppTokenResult{}, fmt.Errorf("secret %s/%s: %w", request.Namespace, request.Name, err)
	}
	installationID, err := githubAppNumber(secret.Data, gitops.GitHubAppInstallationIDKey)
	if err != nil {
		return GitHubAppTokenResult{}, fmt.Errorf("secret %s/%s: %w", request.Namespace, request.Name, err)
	}
	privateKey := secret.Data[gitops.GitHubAppPrivateKeyKey]
	if len(privateKey) == 0 {
		return GitHubAppTokenResult{}, fmt.Errorf("secret %s/%s has no %s", request.Namespace, request.Name, gitops.GitHubAppPrivateKeyKey)
	}

	repositoryURL, err := githubAppTokenRepositoryURL(string(secret.Data[gitOpsReadSecretURLKey]), request.RepositoryURL)
	if err != nil {
		return GitHubAppTokenResult{}, fmt.Errorf("secret %s/%s: %w", request.Namespace, request.Name, err)
	}
	repositoryName := githubapp.RepositoryName(repositoryURL)
	if repositoryName == "" {
		return GitHubAppTokenResult{}, fmt.Errorf("cannot tell the repository from url %q", repositoryURL)
	}

	baseURL := string(secret.Data[gitops.ArgoCDGitHubAppEnterpriseBaseURLKey])
	if baseURL == "" {
		baseURL = string(secret.Data[gitops.FluxGitHubAppBaseURLKey])
	}

	token, err := githubapp.MintInstallationToken(ctx, httpClient, githubapp.TokenRequest{
		AppID:          appID,
		InstallationID: installationID,
		APIBaseURL:     baseURL,
		PrivateKeyPEM:  privateKey,
		Repositories:   []string{repositoryName},
	})
	if err != nil {
		return GitHubAppTokenResult{}, fmt.Errorf("mint github app installation token: %w", err)
	}

	logger.Info("minted github app installation token",
		"secret", request.Name,
		"namespace", request.Namespace,
		"repository", repositoryName,
		"expiresAt", token.ExpiresAt)
	return GitHubAppTokenResult{Token: token.Token, ExpiresAt: token.ExpiresAt}, nil
}

func githubAppNumber(data map[string][]byte, key string) (int64, error) {
	value := strings.TrimSpace(string(data[key]))
	if value == "" {
		return 0, fmt.Errorf("no %s", key)
	}
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s is not a number", key)
	}
	return number, nil
}

// githubAppTokenRepositoryURL picks the repository a token is scoped to. A
// Secret bound to one repository only mints for that one.
func githubAppTokenRepositoryURL(secretURL, requestURL string) (string, error) {
	switch {
	case secretURL == "" && requestURL == "":
		return "", errors.New("no repository url to scope the token to; pass repositoryUrl")
	case secretURL == "":
		return requestURL, nil
	case requestURL == "" || normalizeRepositoryURL(requestURL) == normalizeRepositoryURL(secretURL):
		return secretURL, nil
	default:
		return "", fmt.Errorf("repositoryUrl %q does not match the credential's url %q", requestURL, secretURL)
	}
}

func normalizeRepositoryURL(url string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(url), "/"), ".git"))
}
