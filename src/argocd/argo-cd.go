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
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/store"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ARGO_CD_CONFIGMAP_NAME   = "argo-cd-config" // mogenius argo-cd configmap name
	ARGO_CD_USER_SECRET_NAME = "argo-cd-secret" // mogenius argo-cd configmap name
	ARGO_CD_SERVER_URL       = "https://%s.%s.svc.cluster.local:443"
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
	argoCdConfig   *corev1.ConfigMap
	argoURL        *string
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

type ArgoSessionResponse struct {
	Token string `json:"token"`
}

type ArgoCreateTokenResponse struct {
	Token string `json:"token"`
}

func (self *argocd) ArgoCdCreateApiToken(data ArgoCdCreateApiTokenRequest) (bool, error) {
	err := self.initArgoCdConfig()
	if err != nil {
		return false, err
	}
	if self.argoCdConfig.Data == nil {
		return false, fmt.Errorf("argo-cd-config ConfigMap data is nil")
	}
	if ns, ok := self.argoCdConfig.Data["namespaceName"]; !ok || ns == "" {
		return false, fmt.Errorf("namespaceName key not found in argo-cd-config ConfigMap")
	}

	argoCdSecret, err := self.getArgoCdSecret()
	if err != nil {
		return false, fmt.Errorf("failed to get argo-cd-user-secret Secret: %w", err)
	}
	if argoCdSecret.Data == nil {
		return false, fmt.Errorf("argo-cd-user-secret Secret data is nil")
	}
	if pw, ok := argoCdSecret.Data[fmt.Sprintf("accounts.%s.password", data.Username)]; !ok || pw == nil {
		return false, fmt.Errorf("accounts.%s.password key not found in argo-cd-user-secret Secret", data.Username)
	}
	password := string(argoCdSecret.Data[fmt.Sprintf("accounts.%s.password", data.Username)])

	err = self.initArgoServerUrl()
	if err != nil {
		return false, err
	}

	token, err := self.createArgoToken(data.Username, password, data.Username)
	if err != nil {
		return false, err
	}

	argoCdSecret.Data[fmt.Sprintf("accounts.%s.token", data.Username)] = []byte(token)
	dynamicClient := self.clientProvider.DynamicClient()

	argoCdSecretObjMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&argoCdSecret)
	if err != nil {
		log.Fatalf("Failed to convert Secret to unstructured: %v", err)
	}
	_, err = dynamicClient.Resource(kubernetes.CreateGroupVersionResource(utils.SecretResource.ApiVersion, utils.SecretResource.Plural)).Namespace(self.config.Get("MO_OWN_NAMESPACE")).Update(context.Background(), &unstructured.Unstructured{Object: argoCdSecretObjMap}, metav1.UpdateOptions{})
	if err != nil {
		return false, err
	}

	return true, nil
}

func (self *argocd) ArgoCdApplicationRefresh(data ArgoCdApplicationRefreshRequest) (bool, error) {
	err := self.initArgoCdConfig()
	if err != nil {
		return false, err
	}
	if self.argoCdConfig.Data == nil {
		return false, fmt.Errorf("argo-cd-config ConfigMap data is nil")
	}
	if ns, ok := self.argoCdConfig.Data["namespaceName"]; !ok || ns == "" {
		return false, fmt.Errorf("namespaceName key not found in argo-cd-config ConfigMap")
	}

	argoCdSecret, err := self.getArgoCdSecret()
	if err != nil {
		return false, fmt.Errorf("argo-cd-user-secret Secret data is nil")
	}
	if pw, ok := argoCdSecret.Data[fmt.Sprintf("accounts.%s.token", data.Username)]; !ok || pw == nil {
		return false, fmt.Errorf("accounts.%s.token key not found in argo-cd-user-secret Secret", data.Username)
	}
	token := string(argoCdSecret.Data[fmt.Sprintf("accounts.%s.token", data.Username)])

	err = self.initArgoServerUrl()
	if err != nil {
		return false, err
	}

	_, err = self.refreshApplication(data.ApplicationName, token)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (self *argocd) createArgoToken(username, password, account string) (string, error) {
	loginBody := map[string]string{"username": username, "password": password}
	loginJSON, _ := json.Marshal(loginBody)

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // skip TLS verification for self-signed certs
		},
	}

	responseSession, err := httpClient.Post(fmt.Sprintf("%s/api/v1/session", *self.argoURL), "application/json", bytes.NewReader(loginJSON))
	if err != nil {
		return "", fmt.Errorf("failed to call login: %w", err)
	}
	defer responseSession.Body.Close()

	if responseSession.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(responseSession.Body)
		return "", fmt.Errorf("login failed: %s (%s)", responseSession.Status, string(b))
	}

	var session ArgoSessionResponse
	if err := json.NewDecoder(responseSession.Body).Decode(&session); err != nil {
		return "", fmt.Errorf("cannot decode login response: %w", err)
	}

	tokenBody := map[string]string{} // empty = no expiration
	tokenJSON, _ := json.Marshal(tokenBody)

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/api/v1/account/%s/token", *self.argoURL, account),
		bytes.NewReader(tokenJSON),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // skip TLS verification for self-signed certs
		},
	}
	responseApiToken, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create token: %w", err)
	}
	defer func() {
		err := responseApiToken.Body.Close()
		if err != nil {
			self.logger.Warn("failed to close response body", slog.String("error", err.Error()))
		}
	}()

	if responseApiToken.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(responseApiToken.Body)
		return "", fmt.Errorf("token creation failed: %s (%s)", responseApiToken.Status, string(b))
	}

	var tokenRes ArgoCreateTokenResponse
	if err := json.NewDecoder(responseApiToken.Body).Decode(&tokenRes); err != nil {
		return "", fmt.Errorf("cannot decode token response: %w", err)
	}

	return tokenRes.Token, nil
}

func (self *argocd) refreshApplication(applicationName, token string) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/applications/%s?refresh=normal", *self.argoURL, applicationName)
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
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			self.logger.Warn("failed to close response body", slog.String("error", err.Error()))
		}
	}()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("refresh failed: %s – %s", resp.Status, string(body))
	}

	return true, nil
}

func (self *argocd) initArgoCdConfig() error {
	// Check if argo-cd-config ConfigMap exists in the MO_OWN_NAMESPACE
	argoCdConfigUnstructured, err := store.GetResource(self.valkeyClient, utils.ConfigMapResource.ApiVersion, utils.ConfigMapResource.Kind, self.config.Get("MO_OWN_NAMESPACE"), ARGO_CD_CONFIGMAP_NAME, self.logger)
	if err != nil {
		return err
	}
	var argoCdConfig corev1.ConfigMap
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(argoCdConfigUnstructured.Object, &argoCdConfig)
	if err != nil {
		return err
	}
	self.argoCdConfig = &argoCdConfig
	return nil
}

func (self *argocd) getArgoCdSecret() (*corev1.Secret, error) {
	argoCdSecretUnstructured, err := store.GetResource(self.valkeyClient, utils.SecretResource.ApiVersion, utils.SecretResource.Kind, self.config.Get("MO_OWN_NAMESPACE"), ARGO_CD_USER_SECRET_NAME, self.logger)
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

func (self *argocd) initArgoServerUrl() error {
	// get argo-cd server deployment
	whitelist := []*utils.ResourceDescriptor{&utils.DeploymentResource}
	blacklist := []*utils.ResourceDescriptor{}
	deploymentWorkloads, err := kubernetes.GetUnstructuredNamespaceResourceList(self.argoCdConfig.Data["namespaceName"], whitelist, blacklist)
	if err != nil {
		return err
	}
	// find argo-cd-server deployment
	var argoCdServerDeployment *unstructured.Unstructured
	for _, workload := range deploymentWorkloads {
		labels := workload.GetLabels()
		if labels != nil && labels["app.kubernetes.io/part-of"] == "argocd" && labels["app.kubernetes.io/component"] == "server" && labels["app.kubernetes.io/instance"] == self.argoCdConfig.Data["releaseName"] {
			argoCdServerDeployment = &workload
			break
		}
	}
	if argoCdServerDeployment == nil {
		return fmt.Errorf("argo-cd-server deployment not found in namespace %s", self.argoCdConfig.Data["namespaceName"])
	}

	self.argoURL = utils.Pointer(fmt.Sprintf(ARGO_CD_SERVER_URL, argoCdServerDeployment.GetName(), self.argoCdConfig.Data["namespaceName"]))
	return nil
}
