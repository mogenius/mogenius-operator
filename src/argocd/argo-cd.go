package argocd

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeyclient"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ARGO_CD_CONFIGMAP_NAME   = "argo-cd-config" // mogenius argo-cd configmap name
	ARGO_CD_USER_SECRET_NAME = "argo-cd-secret" // mogenius argo-cd configmap name
	ARGO_CD_SERVER_URL       = "https://argo-cd-argocd-server.%s.svc.cluster.local:443"
)

type Argocd interface {
	ArgoCdCreateApiToken(data ArgoCdCreateApiTokenRequest) (bool, error)
	ArgoCdApplicationRefresh(data ArgoCdApplicationRefreshRequest) (bool, error)
}

type argocd struct {
	logger         *slog.Logger
	config         cfg.ConfigModule
	valkeyClient   valkeyclient.ValkeyClient
	clientProvider k8sclient.K8sClientProvider
}

func NewArgoCd(logManager logging.SlogManager, configModule cfg.ConfigModule, clientProviderModule k8sclient.K8sClientProvider, valkey valkeyclient.ValkeyClient) Argocd {
	argocd := argocd{
		logger:         logManager.CreateLogger("argocd"),
		config:         configModule,
		clientProvider: clientProviderModule,
		valkeyClient:   valkey,
	}
	return &argocd
}

type ArgoCdCreateApiTokenRequest struct {
	Username string `json:"username" validate:"required"`
}

type ArgoCdApplicationRefreshRequest struct {
	Username        string `json:"username" validate:"required"`
	ApplicationName string `json:"applicationName" validate:"required"`
}

// ArgoSessionResponse represents the token returned by /api/v1/session
type ArgoSessionResponse struct {
	Token string `json:"token"`
}

// ArgoCreateTokenResponse represents the token returned by /api/v1/account/{account}/token
type ArgoCreateTokenResponse struct {
	Token string `json:"token"`
}

func (self *argocd) getArgoCdConfig() (*corev1.ConfigMap, error) {
	// Check if argo-cd-config ConfigMap exists in the MO_OWN_NAMESPACE
	argoCdConfigUnstructured, err := store.GetResource(self.valkeyClient, utils.ConfigMapResource.Group, utils.ConfigMapResource.Kind, self.config.Get("MO_OWN_NAMESPACE"), ARGO_CD_CONFIGMAP_NAME, self.logger)
	if err != nil {
		return nil, err
	}
	var argoCdConfig corev1.ConfigMap
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(argoCdConfigUnstructured.Object, &argoCdConfig)
	if err != nil {
		return nil, err
	}
	return &argoCdConfig, nil
}

func (self *argocd) getArgoCdSecret() (*corev1.Secret, error) {
	argoCdSecretUnstructured, err := store.GetResource(self.valkeyClient, utils.SecretResource.Group, utils.SecretResource.Kind, self.config.Get("MO_OWN_NAMESPACE"), ARGO_CD_USER_SECRET_NAME, self.logger)
	if err != nil {
		return nil, err
	}
	var argoCdSecret corev1.Secret
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(argoCdSecretUnstructured.Object, &argoCdSecret)
	if err != nil {
		return nil, err
	}
	return &argoCdSecret, nil
}

func (self *argocd) ArgoCdCreateApiToken(data ArgoCdCreateApiTokenRequest) (bool, error) {
	// Check if argo-cd-config ConfigMap exists in the MO_OWN_NAMESPACE
	argoCdConfig, err := self.getArgoCdConfig()
	if err != nil {
		return false, err
	}
	if argoCdConfig.Data == nil {
		return false, fmt.Errorf("argo-cd-config ConfigMap data is nil")
	}
	if ns, ok := argoCdConfig.Data["namespaceName"]; !ok || ns == "" {
		return false, fmt.Errorf("namespaceName key not found in argo-cd-config ConfigMap")
	}

	argoCdSecret, err := self.getArgoCdSecret()
	if err != nil {
		return false, fmt.Errorf("failed to get argo-cd-user-secret Secret: %w", err)
	}
	if argoCdSecret.Data == nil {
		return false, fmt.Errorf("argo-cd-user-secret Secret data is nil")
	}
	// accounts.mogenius.password
	if pw, ok := argoCdSecret.Data[fmt.Sprintf("accounts.%s.password", data.Username)]; !ok || pw == nil {
		return false, fmt.Errorf("accounts.%s.password key not found in argo-cd-user-secret Secret", data.Username)
	}
	// base64 decode password
	password := string(argoCdSecret.Data[fmt.Sprintf("accounts.%s.password", data.Username)])
	argoURL := fmt.Sprintf("https://argo-cd-argocd-server.%s.svc.cluster.local:443", argoCdConfig.Data["namespaceName"])
	// argoURL := "http://localhost:8080"
	token, err := self.createArgoToken(argoURL, data.Username, password, data.Username)
	if err != nil {
		return false, err
	}

	// add token to argoCdSecret.Data
	argoCdSecret.Data[fmt.Sprintf("accounts.%s.token", data.Username)] = []byte(token)
	dynamicClient := self.clientProvider.DynamicClient()

	argoCdSecretObjMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&argoCdSecret)
	if err != nil {
		log.Fatalf("Failed to convert Secret to unstructured: %v", err)
	}
	_, err = dynamicClient.Resource(kubernetes.CreateGroupVersionResource("v1", "", "secrets")).Namespace(self.config.Get("MO_OWN_NAMESPACE")).Update(context.Background(), &unstructured.Unstructured{Object: argoCdSecretObjMap}, metav1.UpdateOptions{})
	if err != nil {
		return false, err
	}

	return true, nil
}

func (self *argocd) ArgoCdApplicationRefresh(data ArgoCdApplicationRefreshRequest) (bool, error) {
	// Check if argo-cd-config ConfigMap exists in the MO_OWN_NAMESPACE
	argoCdConfig, err := self.getArgoCdConfig()
	if err != nil {
		return false, err
	}
	if argoCdConfig.Data == nil {
		return false, fmt.Errorf("argo-cd-config ConfigMap data is nil")
	}
	if ns, ok := argoCdConfig.Data["namespaceName"]; !ok || ns == "" {
		return false, fmt.Errorf("namespaceName key not found in argo-cd-config ConfigMap")
	}

	argoCdSecret, err := self.getArgoCdSecret()
	if err != nil {
		return false, fmt.Errorf("argo-cd-user-secret Secret data is nil")
	}
	// accounts.mogenius.token
	if pw, ok := argoCdSecret.Data[fmt.Sprintf("accounts.%s.token", data.Username)]; !ok || pw == nil {
		return false, fmt.Errorf("accounts.%s.token key not found in argo-cd-user-secret Secret", data.Username)
	}
	// base64 decode password
	token := string(argoCdSecret.Data[fmt.Sprintf("accounts.%s.token", data.Username)])
	argoURL := fmt.Sprintf("https://argo-cd-argocd-server.%s.svc.cluster.local:443", argoCdConfig.Data["namespaceName"])
	// argoURL := "http://localhost:8080"
	_, err = self.refreshApplication(argoURL, data.ApplicationName, token)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (self *argocd) createArgoToken(argoURL, username, password, account string) (string, error) {
	// Login to get session token
	loginBody := map[string]string{"username": username, "password": password}
	loginJSON, _ := json.Marshal(loginBody)

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // ⚠️ skip TLS verification for self-signed certs
		},
	}

	resp, err := httpClient.Post(fmt.Sprintf("%s/api/v1/session", argoURL), "application/json", bytes.NewReader(loginJSON))
	if err != nil {
		return "", fmt.Errorf("failed to call login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login failed: %s (%s)", resp.Status, string(b))
	}

	var session ArgoSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return "", fmt.Errorf("cannot decode login response: %w", err)
	}

	// Create a long‑lived token (API key)
	tokenBody := map[string]string{} // empty = no expiration
	tokenJSON, _ := json.Marshal(tokenBody)

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/api/v1/account/%s/token", argoURL, account),
		bytes.NewReader(tokenJSON),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // ⚠️ skip TLS verification for self-signed certs
		},
	}
	resp2, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create token: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp2.Body)
		return "", fmt.Errorf("token creation failed: %s (%s)", resp2.Status, string(b))
	}

	var tokenRes ArgoCreateTokenResponse
	if err := json.NewDecoder(resp2.Body).Decode(&tokenRes); err != nil {
		return "", fmt.Errorf("cannot decode token response: %w", err)
	}

	return tokenRes.Token, nil
}

func (self *argocd) refreshApplication(argoURL, applicationName, token string) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/applications/%s?refresh=normal", argoURL, applicationName)

	// Insecure http client (skip TLS verification) — only for internal/self-signed setups
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("refresh failed: %s – %s", resp.Status, string(body))
	}

	return true, nil
}
