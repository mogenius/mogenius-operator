package core

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

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
	k8stesting "k8s.io/client-go/testing"
)

const legacyTestNamespace = "mogenius"

var (
	legacyTestAiModelGVR        = schema.GroupVersionResource{Group: "mogenius.com", Version: "v1alpha1", Resource: "aimodels"}
	legacyTestMcpServerGVR      = schema.GroupVersionResource{Group: "mogenius.com", Version: "v1alpha1", Resource: "mcpservers"}
	legacyTestPlatformConfigGVR = schema.GroupVersionResource{Group: "mogenius.com", Version: "v1alpha1", Resource: "platformconfigs"}
)

func legacyTestSecret() *corev1.Secret {
	return &corev1.Secret{
		Name: LEGACY_AI_CONFIG_SECRET_NAME, Namespace: legacyTestNamespace, UID: "legacy-uid",
		Data: map[string][]byte{"API_KEY": []byte("sk-test"), "AI_CONFIG_GITHUB_PAT": []byte("pat")},
	}
}

func legacyTestObject(kind, name, namespace string, spec map[string]any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "mogenius.com/v1alpha1",
		"kind":       kind,
		"metadata":   map[string]any{"name": name},
		"spec":       spec,
	}}
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	return obj
}

func legacyTestDynamicClient(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		legacyTestAiModelGVR:        "AiModelList",
		legacyTestMcpServerGVR:      "McpServerList",
		legacyTestPlatformConfigGVR: "PlatformConfigList",
	}, objects...)
}

func legacySecretExists(t *testing.T, clientset *fake.Clientset) bool {
	t.Helper()
	_, err := clientset.CoreV1().Secrets(legacyTestNamespace).Get(context.Background(), LEGACY_AI_CONFIG_SECRET_NAME, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false
	}
	require.NoError(t, err)
	return true
}

func TestRemoveLegacyAiConfigSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		withSecret  bool
		objects     []runtime.Object
		wantRemoved bool
	}{
		{
			name:        "no legacy secret: nothing to do",
			withSecret:  false,
			wantRemoved: false,
		},
		{
			name:        "unreferenced legacy secret is removed",
			withSecret:  true,
			wantRemoved: true,
		},
		{
			name:       "kept while an AiModel uses it as apiKeySecretRef",
			withSecret: true,
			objects: []runtime.Object{legacyTestObject("AiModel", "claude", legacyTestNamespace, map[string]any{
				"sdk":             "anthropic",
				"apiKeySecretRef": map[string]any{"name": LEGACY_AI_CONFIG_SECRET_NAME, "key": "API_KEY"},
			})},
			wantRemoved: false,
		},
		{
			name:       "kept while an McpServer header reads it via valueFrom",
			withSecret: true,
			objects: []runtime.Object{legacyTestObject("McpServer", "github", legacyTestNamespace, map[string]any{
				"headers": []any{map[string]any{
					"name":      "Authorization",
					"valueFrom": map[string]any{"name": LEGACY_AI_CONFIG_SECRET_NAME, "key": "AI_CONFIG_GITHUB_PAT"},
				}},
			})},
			wantRemoved: false,
		},
		{
			name:       "kept while the PlatformConfig names it as a secret",
			withSecret: true,
			objects: []runtime.Object{legacyTestObject("PlatformConfig", "platform", "", map[string]any{
				"repository": map[string]any{"secretRef": map[string]any{"secret": map[string]any{"name": LEGACY_AI_CONFIG_SECRET_NAME}}},
			})},
			wantRemoved: false,
		},
		{
			name:       "removed when objects only reference other secrets",
			withSecret: true,
			objects: []runtime.Object{legacyTestObject("AiModel", "claude", legacyTestNamespace, map[string]any{
				"apiKeySecretRef": map[string]any{"name": "aimodel-claude-api-key"},
			})},
			wantRemoved: true,
		},
		{
			name:       "an AiModel in another namespace does not keep it",
			withSecret: true,
			objects: []runtime.Object{legacyTestObject("AiModel", "claude", "elsewhere", map[string]any{
				"apiKeySecretRef": map[string]any{"name": LEGACY_AI_CONFIG_SECRET_NAME},
			})},
			wantRemoved: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			clientset := fake.NewSimpleClientset()
			if tt.withSecret {
				_, err := clientset.CoreV1().Secrets(legacyTestNamespace).Create(context.Background(), legacyTestSecret(), metav1.CreateOptions{})
				require.NoError(t, err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))

			err := removeLegacyAiConfigSecret(context.Background(), logger, legacyTestNamespace, clientset, legacyTestDynamicClient(tt.objects...))

			require.NoError(t, err)
			if tt.withSecret {
				assert.Equal(t, !tt.wantRemoved, legacySecretExists(t, clientset))
			}
		})
	}
}

// A CRD that is not served yet has no objects and therefore no references.
func TestRemoveLegacyAiConfigSecretTreatsUnservedCrdAsUnreferenced(t *testing.T) {
	t.Parallel()

	clientset := fake.NewSimpleClientset(legacyTestSecret())
	dynamicClient := legacyTestDynamicClient()
	dynamicClient.PrependReactor("list", "mcpservers", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(legacyTestMcpServerGVR.GroupResource(), "")
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := removeLegacyAiConfigSecret(context.Background(), logger, legacyTestNamespace, clientset, dynamicClient)

	require.NoError(t, err)
	assert.False(t, legacySecretExists(t, clientset))
}

// When the references cannot be read the secret must stay: deleting a key a
// model still uses would break it silently.
func TestRemoveLegacyAiConfigSecretKeepsSecretWhenReferencesUnreadable(t *testing.T) {
	t.Parallel()

	clientset := fake.NewSimpleClientset(legacyTestSecret())
	dynamicClient := legacyTestDynamicClient()
	dynamicClient.PrependReactor("list", "aimodels", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("api server hiccup")
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := removeLegacyAiConfigSecret(context.Background(), logger, legacyTestNamespace, clientset, dynamicClient)

	assert.Error(t, err)
	assert.True(t, legacySecretExists(t, clientset))
}

func TestNamesSecretOnlyMatchesReferenceFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec map[string]any
		want bool
	}{
		{"plain name field is not a reference", map[string]any{"name": LEGACY_AI_CONFIG_SECRET_NAME}, false},
		{"apiKeySecretRef", map[string]any{"apiKeySecretRef": map[string]any{"name": LEGACY_AI_CONFIG_SECRET_NAME}}, true},
		{"secretRef nested deep", map[string]any{"a": map[string]any{"b": map[string]any{"secretRef": map[string]any{"name": LEGACY_AI_CONFIG_SECRET_NAME}}}}, true},
		{"valueFrom in a list", map[string]any{"env": []any{map[string]any{"valueFrom": map[string]any{"name": LEGACY_AI_CONFIG_SECRET_NAME}}}}, true},
		{"other secret", map[string]any{"secretRef": map[string]any{"name": "other"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, namesSecret(tt.spec, LEGACY_AI_CONFIG_SECRET_NAME))
		})
	}
}
