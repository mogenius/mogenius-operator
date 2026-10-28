package reconciler

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"log/slog"
	"testing"

	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"mogenius-operator/src/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	testOperatorNamespace = "mogenius"
	testArgoNamespace     = "argocd"
)

var externalSecretGVR = schema.GroupVersionResource{Group: "external-secrets.io", Version: "v1", Resource: "externalsecrets"}

func githubAppRepo() v1alpha1.GitOpsRepositoryConfig {
	return v1alpha1.GitOpsRepositoryConfig{
		Name: "platform",
		URL:  "https://github.com/acme/platform.git",
		GitHubApp: &v1alpha1.GitHubAppConfig{
			AppID:          7,
			InstallationID: 42,
			PrivateKey: v1alpha1.GenericSecretReference{
				Secret: &v1alpha1.SecretReference{Name: "github-app"},
			},
		},
	}
}

// githubAppHarness wires githubAppCredentials to fake clients.
type githubAppHarness struct {
	credentials githubAppCredentials
	client      *fake.Clientset
	dynamic     *dynamicfake.FakeDynamicClient
	applied     []map[string]any
}

func newGitHubAppHarness(t *testing.T, objects ...runtime.Object) *githubAppHarness {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	h := &githubAppHarness{}
	objects = append(objects, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "github-app", Namespace: testOperatorNamespace},
		Data:       map[string][]byte{"privateKey": pemBytes},
	})
	h.client = fake.NewClientset(objects...)
	h.dynamic = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{externalSecretGVR: "ExternalSecretList"})

	h.credentials = githubAppCredentials{
		client:            h.client,
		dynamic:           h.dynamic,
		logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		operatorNamespace: testOperatorNamespace,
		engine:            gitOpsEngineArgoCD,
		engineNamespace:   testArgoNamespace,
		applyObject: func(_ context.Context, _ utils.ResourceDescriptor, _ string, object map[string]any) error {
			h.applied = append(h.applied, object)
			return nil
		},
	}
	return h
}

func (h *githubAppHarness) repo() v1alpha1.GitOpsRepositoryConfig {
	return githubAppRepo()
}

func (h *githubAppHarness) reconcile(t *testing.T, repos ...v1alpha1.GitOpsRepositoryConfig) error {
	t.Helper()
	return h.credentials.reconcile(context.Background(), &v1alpha1.GitOpsConfig{Repositories: repos})
}

func (h *githubAppHarness) secret(t *testing.T, namespace, name string) *corev1.Secret {
	t.Helper()
	secret, err := h.client.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)
	return secret
}

func (h *githubAppHarness) secretGone(t *testing.T, namespace, name string) bool {
	t.Helper()
	_, err := h.client.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	return apierrors.IsNotFound(err)
}

func TestGitHubAppCredentialsMaterializeTheReadCredential(t *testing.T) {
	h := newGitHubAppHarness(t)

	require.NoError(t, h.reconcile(t, h.repo()))

	read := h.secret(t, testArgoNamespace, "platform-repository")
	assert.Equal(t, "repository", read.Labels[gitops.ArgoCDSecretTypeLabelKey])
	assert.Equal(t, gitops.GitHubAppLabelValue, read.Labels[gitops.GitHubAppLabelKey])
	assert.Equal(t, "7", string(read.Data["githubAppID"]))
	assert.NotEmpty(t, read.Data["githubAppPrivateKey"])
	assert.True(t, h.secretGone(t, testOperatorNamespace, "mogenius-gitops-write-platform"), "no token is stored")
}

// A token credential under the same name keeps unrelated keys, loses the
// username/password Argo CD would prefer over the app.
func TestGitHubAppCredentialsReplaceTokenKeysOfAPlatformReadCredential(t *testing.T) {
	h := newGitHubAppHarness(t, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "platform-repository",
			Namespace: testArgoNamespace,
			Labels:    map[string]string{gitops.ManagedByLabelKey: gitops.ManagedByLabelValue},
		},
		Data: map[string][]byte{
			"username":          []byte("x-token-auth"),
			"password":          []byte("ghp_old"),
			"tlsClientCertData": []byte("cert"),
		},
	})

	require.NoError(t, h.reconcile(t, h.repo()))

	read := h.secret(t, testArgoNamespace, "platform-repository")
	assert.NotContains(t, read.Data, "username")
	assert.NotContains(t, read.Data, "password")
	assert.Equal(t, "cert", string(read.Data["tlsClientCertData"]))
	assert.NotEmpty(t, read.Data["githubAppPrivateKey"])
}

func TestGitHubAppCredentialsLeaveAForeignReadCredentialAlone(t *testing.T) {
	h := newGitHubAppHarness(t, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "platform-repository", Namespace: testArgoNamespace},
		Data:       map[string][]byte{"password": []byte("mine")},
	})

	err := h.reconcile(t, h.repo())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not created by mogenius")
	assert.Equal(t, "mine", string(h.secret(t, testArgoNamespace, "platform-repository").Data["password"]))
}

func TestGitHubAppCredentialsPruneWhenTheAppIsRemoved(t *testing.T) {
	unrelated := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: testArgoNamespace}}
	storedToken := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:      "mogenius-gitops-write-platform",
		Namespace: testOperatorNamespace,
		Labels:    map[string]string{gitops.GitHubAppLabelKey: gitops.GitHubAppLabelValue},
	}}
	h := newGitHubAppHarness(t, unrelated, storedToken)
	require.NoError(t, h.reconcile(t, h.repo()))

	plain := h.repo()
	plain.GitHubApp = nil
	require.NoError(t, h.reconcile(t, plain))

	assert.True(t, h.secretGone(t, testArgoNamespace, "platform-repository"))
	assert.True(t, h.secretGone(t, testOperatorNamespace, "mogenius-gitops-write-platform"))
	assert.False(t, h.secretGone(t, testArgoNamespace, "unrelated"))
	assert.False(t, h.secretGone(t, testOperatorNamespace, "github-app"), "the source of the private key is the user's")
}

func TestGitHubAppCredentialsFromAVaultRenderALabelledExternalSecret(t *testing.T) {
	h := newGitHubAppHarness(t)
	repo := h.repo()
	repo.GitHubApp.PrivateKey = v1alpha1.GenericSecretReference{
		ExternalSecret: &v1alpha1.ExternalSecret{Vault: "vault", Path: "github/app"},
	}

	require.NoError(t, h.reconcile(t, repo))
	require.Len(t, h.applied, 1)
	object := &unstructured.Unstructured{Object: h.applied[0]}
	assert.Equal(t, "platform-repository", object.GetName())
	assert.Equal(t, gitops.GitHubAppLabelValue, object.GetLabels()[gitops.GitHubAppLabelKey])
	data, _, _ := unstructured.NestedStringMap(object.Object, "spec", "target", "template", "data")
	assert.Equal(t, "{{ .privateKey }}", data["githubAppPrivateKey"])
	assert.Equal(t, "42", data["githubAppInstallationID"])
}

// Moving the key from a vault to a plain Secret deletes the ExternalSecret,
// orphaning its target so the Secret written next is not garbage-collected.
func TestGitHubAppCredentialsPruneTheExternalSecretOnASwitchToAPlainSecret(t *testing.T) {
	h := newGitHubAppHarness(t)
	stale := &unstructured.Unstructured{}
	stale.SetAPIVersion("external-secrets.io/v1")
	stale.SetKind("ExternalSecret")
	stale.SetName("platform-repository")
	stale.SetNamespace(testArgoNamespace)
	stale.SetLabels(map[string]string{gitops.GitHubAppLabelKey: gitops.GitHubAppLabelValue})
	_, err := h.dynamic.Resource(externalSecretGVR).Namespace(testArgoNamespace).Create(context.Background(), stale, metav1.CreateOptions{})
	require.NoError(t, err)

	require.NoError(t, h.reconcile(t, h.repo()))

	_, err = h.dynamic.Resource(externalSecretGVR).Namespace(testArgoNamespace).Get(context.Background(), "platform-repository", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err))
	assert.False(t, h.secretGone(t, testArgoNamespace, "platform-repository"))
}

func TestGitHubAppReadCredentialForArgoCDIsDiscoverableByURL(t *testing.T) {
	repo := githubAppRepo()
	repo.GitHubApp.EnterpriseBaseURL = "https://ghe.acme.io/api/v3"

	labels, data := githubAppReadCredential(repo, gitOpsEngineArgoCD)

	assert.Equal(t, gitops.ArgoCDSecretTypeLabelValue, labels[gitops.ArgoCDSecretTypeLabelKey])
	assert.Equal(t, map[string]string{
		"type":                       "git",
		"url":                        "https://github.com/acme/platform.git",
		"githubAppID":                "7",
		"githubAppInstallationID":    "42",
		"githubAppEnterpriseBaseUrl": "https://ghe.acme.io/api/v3",
	}, data)
}

func TestGitHubAppReadCredentialForFluxUsesFluxKeys(t *testing.T) {
	repo := githubAppRepo()
	repo.GitHubApp.EnterpriseBaseURL = "https://ghe.acme.io/api/v3"

	labels, data := githubAppReadCredential(repo, gitOpsEngineFlux)

	_, argoLabel := labels[gitops.ArgoCDSecretTypeLabelKey]
	assert.False(t, argoLabel)
	assert.Equal(t, map[string]string{
		"githubAppID":             "7",
		"githubAppInstallationID": "42",
		"githubAppBaseURL":        "https://ghe.acme.io/api/v3",
	}, data)
}

func TestFluxGitRepositoryObjectSelectsTheGitHubProvider(t *testing.T) {
	object := fluxGitRepositoryObject("platform", githubAppRepo(), "flux-system", "platform-repository")

	spec := object["spec"].(map[string]any)
	assert.Equal(t, "github", spec["provider"])
	assert.Equal(t, map[string]any{"name": "platform-repository"}, spec["secretRef"])
}

func gitRepositoryCRD(providerEnum ...any) *unstructured.Unstructured {
	spec := map[string]any{}
	if providerEnum != nil {
		spec["provider"] = map[string]any{"enum": providerEnum}
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{"versions": []any{map[string]any{
			"name": "v1",
			"schema": map[string]any{"openAPIV3Schema": map[string]any{
				"properties": map[string]any{"spec": map[string]any{"properties": spec}},
			}},
		}}},
	}}
}

func TestGitRepositoryCRDGitHubProviderSupport(t *testing.T) {
	assert.Equal(t, githubProviderSupported, gitRepositoryCRDGitHubProviderSupport(gitRepositoryCRD("generic", "azure", "github")))
	assert.Equal(t, githubProviderUnsupported, gitRepositoryCRDGitHubProviderSupport(gitRepositoryCRD("generic", "azure")))
	assert.Equal(t, githubProviderUnsupported, gitRepositoryCRDGitHubProviderSupport(gitRepositoryCRD()))
}

func TestDeclaredComponentsReportsGitHubAppCredentials(t *testing.T) {
	spec := v1alpha1.PlatformConfigSpec{GitOps: &v1alpha1.GitOpsConfig{
		Repositories: []v1alpha1.GitOpsRepositoryConfig{githubAppRepo()},
	}}

	assert.True(t, declaredComponents(spec)[componentGitHubAppCredentials].declared)

	spec.GitOps.Repositories[0].GitHubApp = nil
	assert.False(t, declaredComponents(spec)[componentGitHubAppCredentials].declared)
}
