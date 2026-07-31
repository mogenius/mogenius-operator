package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"mogenius-operator/src/ai"
	"mogenius-operator/src/crds/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const testNamespace = "mogenius"

// fakeAiModelCrdOps stubs the CR surface with injectable behavior.
type fakeAiModelCrdOps struct {
	createErr error
	updateErr error
	getModel  *v1alpha1.AiModel
	getErr    error

	createCalls int
	updateCalls int
}

func (self *fakeAiModelCrdOps) Create(namespace string, name string, spec v1alpha1.AiModelSpec) (*v1alpha1.AiModel, error) {
	self.createCalls++
	if self.createErr != nil {
		return nil, self.createErr
	}
	return aiModelFixture(name, spec), nil
}

func (self *fakeAiModelCrdOps) Update(namespace string, name string, spec v1alpha1.AiModelSpec) (*v1alpha1.AiModel, error) {
	self.updateCalls++
	if self.updateErr != nil {
		return nil, self.updateErr
	}
	return aiModelFixture(name, spec), nil
}

func (self *fakeAiModelCrdOps) Get(namespace string, name string) (*v1alpha1.AiModel, error) {
	return self.getModel, self.getErr
}

func aiModelFixture(name string, spec v1alpha1.AiModelSpec) *v1alpha1.AiModel {
	return &v1alpha1.AiModel{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace, UID: types.UID("uid-" + name)},
		Spec:       spec,
	}
}

func anthropicSpec(refName string) v1alpha1.AiModelSpec {
	spec := v1alpha1.AiModelSpec{Sdk: "anthropic", Model: "claude-sonnet-5"}
	if refName != "" {
		spec.ApiKeySecretRef = &v1alpha1.SecretKeyRef{Name: refName, Key: ai.DefaultApiKeySecretKey}
	}
	return spec
}

func managedSecretFixture(modelName string, apiKey string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managedAiModelSecretName(modelName),
			Namespace: testNamespace,
			Labels: map[string]string{
				aiModelSecretManagedByLabelKey: aiModelSecretManagedByLabelValue,
				aiModelSecretModelLabelKey:     modelName,
			},
		},
		Data: map[string][]byte{ai.DefaultApiKeySecretKey: []byte(apiKey)},
	}
}

func getSecret(t *testing.T, client *fake.Clientset, modelName string) *corev1.Secret {
	t.Helper()
	secret, err := client.CoreV1().Secrets(testNamespace).Get(context.Background(), managedAiModelSecretName(modelName), metav1.GetOptions{})
	require.NoError(t, err)
	return secret
}

func TestManagedAiModelSecretName(t *testing.T) {
	assert.Equal(t, "aimodel-claude-api-key", managedAiModelSecretName("claude"))

	longName := strings.Repeat("a", 253)
	result := managedAiModelSecretName(longName)
	assert.LessOrEqual(t, len(result), validation.DNS1123SubdomainMaxLength)
	assert.Empty(t, validation.IsDNS1123Subdomain(result))
	assert.Equal(t, result, managedAiModelSecretName(longName), "must be deterministic")
	assert.NotEqual(t, result, managedAiModelSecretName(strings.Repeat("b", 253)), "distinct long names must not collide")
}

func TestApplyManagedApiKeyRef(t *testing.T) {
	managedName := managedAiModelSecretName("claude")

	tests := []struct {
		name        string
		spec        v1alpha1.AiModelSpec
		apiKey      string
		wantErr     bool
		wantRefName string
	}{
		{name: "no apiKey leaves spec untouched", spec: anthropicSpec("shared-secret"), apiKey: "", wantRefName: "shared-secret"},
		{name: "no apiKey and no ref stays empty", spec: anthropicSpec(""), apiKey: "", wantRefName: ""},
		{name: "apiKey wires managed ref", spec: anthropicSpec(""), apiKey: "sk-123", wantRefName: managedName},
		{name: "apiKey with round-tripped managed ref is fine", spec: anthropicSpec(managedName), apiKey: "sk-123", wantRefName: managedName},
		{name: "apiKey with foreign ref conflicts", spec: anthropicSpec("shared-secret"), apiKey: "sk-123", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := applyManagedApiKeyRef("claude", &tt.spec, tt.apiKey)
			if tt.wantErr {
				require.Error(t, err)
				assert.NotContains(t, err.Error(), tt.apiKey, "error must not leak the key")
				return
			}
			require.NoError(t, err)
			if tt.wantRefName == "" {
				assert.Nil(t, tt.spec.ApiKeySecretRef)
			} else {
				require.NotNil(t, tt.spec.ApiKeySecretRef)
				assert.Equal(t, tt.wantRefName, tt.spec.ApiKeySecretRef.Name)
				assert.Equal(t, ai.DefaultApiKeySecretKey, tt.spec.ApiKeySecretRef.Key)
			}
		})
	}
}

func TestApplyManagedApiKeyRefValidatesForAnthropicWithKeyOnly(t *testing.T) {
	// The whole point of the feature: apiKey alone must yield a spec that
	// passes ValidateAiModelSpec for SDKs requiring a key.
	spec := anthropicSpec("")
	require.NoError(t, applyManagedApiKeyRef("claude", &spec, "sk-123"))
	assert.NoError(t, ai.ValidateAiModelSpec(spec))

	// Ollama with apiKey is not special-cased and stays valid too.
	ollama := v1alpha1.AiModelSpec{Sdk: "ollama", Model: "llama3.1:8b", ApiUrl: "http://ollama:11434"}
	require.NoError(t, applyManagedApiKeyRef("local", &ollama, "irrelevant"))
	assert.NoError(t, ai.ValidateAiModelSpec(ollama))
}

func TestUpsertManagedAiModelSecretCreatesFresh(t *testing.T) {
	client := fake.NewClientset()

	createdFresh, err := upsertManagedAiModelSecret(context.Background(), client, testNamespace, "claude", "sk-123", nil)
	require.NoError(t, err)
	assert.True(t, createdFresh)

	secret := getSecret(t, client, "claude")
	assert.Equal(t, []byte("sk-123"), secret.Data[ai.DefaultApiKeySecretKey])
	assert.Equal(t, aiModelSecretManagedByLabelValue, secret.Labels[aiModelSecretManagedByLabelKey])
	assert.Equal(t, "claude", secret.Labels[aiModelSecretModelLabelKey])
	assert.Empty(t, secret.OwnerReferences, "no owner before the CR exists")
}

func TestUpsertManagedAiModelSecretCreatesWithOwner(t *testing.T) {
	client := fake.NewClientset()
	owner := aiModelFixture("claude", anthropicSpec(""))

	_, err := upsertManagedAiModelSecret(context.Background(), client, testNamespace, "claude", "sk-123", owner)
	require.NoError(t, err)

	secret := getSecret(t, client, "claude")
	require.Len(t, secret.OwnerReferences, 1)
	assert.Equal(t, owner.UID, secret.OwnerReferences[0].UID)
	assert.Equal(t, "AiModel", secret.OwnerReferences[0].Kind)
}

func TestUpsertManagedAiModelSecretRotatesAndAddsOwner(t *testing.T) {
	client := fake.NewClientset(managedSecretFixture("claude", "sk-old"))
	owner := aiModelFixture("claude", anthropicSpec(""))

	createdFresh, err := upsertManagedAiModelSecret(context.Background(), client, testNamespace, "claude", "sk-new", owner)
	require.NoError(t, err)
	assert.False(t, createdFresh)

	secret := getSecret(t, client, "claude")
	assert.Equal(t, []byte("sk-new"), secret.Data[ai.DefaultApiKeySecretKey])
	require.Len(t, secret.OwnerReferences, 1)
	assert.Equal(t, owner.UID, secret.OwnerReferences[0].UID)

	// Idempotent: a second rotation must not duplicate the ownerRef.
	_, err = upsertManagedAiModelSecret(context.Background(), client, testNamespace, "claude", "sk-newer", owner)
	require.NoError(t, err)
	assert.Len(t, getSecret(t, client, "claude").OwnerReferences, 1)
}

func TestUpsertManagedAiModelSecretRefusesUnmanagedSecret(t *testing.T) {
	unmanaged := managedSecretFixture("claude", "user-owned")
	unmanaged.Labels = nil
	client := fake.NewClientset(unmanaged)

	_, err := upsertManagedAiModelSecret(context.Background(), client, testNamespace, "claude", "sk-123", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not managed by mogenius")
	assert.NotContains(t, err.Error(), "sk-123", "error must not leak the key")

	secret := getSecret(t, client, "claude")
	assert.Equal(t, []byte("user-owned"), secret.Data[ai.DefaultApiKeySecretKey], "unmanaged secret must stay untouched")
}

func TestUpsertManagedAiModelSecretSurvivesCreateRace(t *testing.T) {
	client := fake.NewClientset()
	raced := false
	client.PrependReactor("create", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if raced {
			return false, nil, nil
		}
		raced = true
		// Simulate a concurrent create winning the race: materialize the
		// competitor's secret, then fail this create with AlreadyExists.
		competitor := managedSecretFixture("claude", "sk-competitor")
		_ = client.Tracker().Add(competitor)
		return true, nil, apierrors.NewAlreadyExists(schema.GroupResource{Resource: "secrets"}, competitor.Name)
	})

	createdFresh, err := upsertManagedAiModelSecret(context.Background(), client, testNamespace, "claude", "sk-123", nil)
	require.NoError(t, err)
	assert.False(t, createdFresh)
	assert.Equal(t, []byte("sk-123"), getSecret(t, client, "claude").Data[ai.DefaultApiKeySecretKey])
}

func TestUpsertManagedAiModelSecretRetriesOnConflict(t *testing.T) {
	client := fake.NewClientset(managedSecretFixture("claude", "sk-old"))
	conflicted := false
	client.PrependReactor("update", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if conflicted {
			return false, nil, nil
		}
		conflicted = true
		return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "secrets"}, managedAiModelSecretName("claude"), fmt.Errorf("conflict"))
	})

	_, err := upsertManagedAiModelSecret(context.Background(), client, testNamespace, "claude", "sk-new", nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("sk-new"), getSecret(t, client, "claude").Data[ai.DefaultApiKeySecretKey])
}

func TestDeleteManagedAiModelSecret(t *testing.T) {
	t.Run("deletes managed secret", func(t *testing.T) {
		client := fake.NewClientset(managedSecretFixture("claude", "sk-123"))
		require.NoError(t, deleteManagedAiModelSecret(context.Background(), client, testNamespace, "claude"))
		_, err := client.CoreV1().Secrets(testNamespace).Get(context.Background(), managedAiModelSecretName("claude"), metav1.GetOptions{})
		assert.True(t, apierrors.IsNotFound(err))
	})

	t.Run("refuses unmanaged secret", func(t *testing.T) {
		unmanaged := managedSecretFixture("claude", "user-owned")
		unmanaged.Labels = map[string]string{"app.kubernetes.io/managed-by": "someone-else"}
		client := fake.NewClientset(unmanaged)
		require.Error(t, deleteManagedAiModelSecret(context.Background(), client, testNamespace, "claude"))
		_, err := client.CoreV1().Secrets(testNamespace).Get(context.Background(), managedAiModelSecretName("claude"), metav1.GetOptions{})
		assert.NoError(t, err, "unmanaged secret must survive")
	})

	t.Run("missing secret is fine", func(t *testing.T) {
		client := fake.NewClientset()
		assert.NoError(t, deleteManagedAiModelSecret(context.Background(), client, testNamespace, "claude"))
	})
}

func TestCreateAiModelWithoutApiKeySkipsSecrets(t *testing.T) {
	client := fake.NewClientset()
	crdOps := &fakeAiModelCrdOps{}

	_, err := createAiModelWithManagedSecret(context.Background(), client, crdOps, slog.Default(), testNamespace, "claude", anthropicSpec("shared-secret"), "")
	require.NoError(t, err)
	assert.Equal(t, 1, crdOps.createCalls)
	assert.Empty(t, client.Actions(), "no secret API calls without apiKey")
}

func TestCreateAiModelProvisionsSecretBeforeCrAndSetsOwner(t *testing.T) {
	client := fake.NewClientset()
	crdOps := &fakeAiModelCrdOps{}
	spec := anthropicSpec(managedAiModelSecretName("claude"))

	created, err := createAiModelWithManagedSecret(context.Background(), client, crdOps, slog.Default(), testNamespace, "claude", spec, "sk-123")
	require.NoError(t, err)
	require.NotNil(t, created)

	secret := getSecret(t, client, "claude")
	assert.Equal(t, []byte("sk-123"), secret.Data[ai.DefaultApiKeySecretKey])
	require.Len(t, secret.OwnerReferences, 1)
	assert.Equal(t, created.UID, secret.OwnerReferences[0].UID)
}

func TestCreateAiModelRollsBackFreshSecretOnCrFailure(t *testing.T) {
	client := fake.NewClientset()
	crdOps := &fakeAiModelCrdOps{createErr: fmt.Errorf("webhook denied")}

	_, err := createAiModelWithManagedSecret(context.Background(), client, crdOps, slog.Default(), testNamespace, "claude", anthropicSpec(""), "sk-123")
	require.Error(t, err)

	_, getErr := client.CoreV1().Secrets(testNamespace).Get(context.Background(), managedAiModelSecretName("claude"), metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(getErr), "fresh secret must be rolled back")
}

func TestCreateAiModelKeepsPreexistingSecretOnCrFailure(t *testing.T) {
	client := fake.NewClientset(managedSecretFixture("claude", "sk-old"))
	crdOps := &fakeAiModelCrdOps{createErr: fmt.Errorf("webhook denied")}

	_, err := createAiModelWithManagedSecret(context.Background(), client, crdOps, slog.Default(), testNamespace, "claude", anthropicSpec(""), "sk-new")
	require.Error(t, err)

	secret := getSecret(t, client, "claude")
	assert.Equal(t, []byte("sk-new"), secret.Data[ai.DefaultApiKeySecretKey], "pre-existing managed secret is kept (value already rotated)")
}

func TestCreateAiModelKeepsSecretWhenRaceWinnerUsesIt(t *testing.T) {
	managedName := managedAiModelSecretName("claude")
	client := fake.NewClientset()
	crdOps := &fakeAiModelCrdOps{
		createErr: fmt.Errorf("RESTClient: %w", apierrors.NewAlreadyExists(schema.GroupResource{Group: "mogenius.com", Resource: "aimodels"}, "claude")),
		getModel:  aiModelFixture("claude", anthropicSpec(managedName)),
	}

	_, err := createAiModelWithManagedSecret(context.Background(), client, crdOps, slog.Default(), testNamespace, "claude", anthropicSpec(managedName), "sk-123")
	require.Error(t, err)

	_, getErr := client.CoreV1().Secrets(testNamespace).Get(context.Background(), managedName, metav1.GetOptions{})
	assert.NoError(t, getErr, "race winner references the managed secret, so it must be kept")
}

func TestCreateAiModelDeletesSecretWhenRaceWinnerUsesForeignRef(t *testing.T) {
	client := fake.NewClientset()
	crdOps := &fakeAiModelCrdOps{
		createErr: fmt.Errorf("RESTClient: %w", apierrors.NewAlreadyExists(schema.GroupResource{Group: "mogenius.com", Resource: "aimodels"}, "claude")),
		getModel:  aiModelFixture("claude", anthropicSpec("shared-secret")),
	}

	_, err := createAiModelWithManagedSecret(context.Background(), client, crdOps, slog.Default(), testNamespace, "claude", anthropicSpec(""), "sk-123")
	require.Error(t, err)

	_, getErr := client.CoreV1().Secrets(testNamespace).Get(context.Background(), managedAiModelSecretName("claude"), metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(getErr))
}

func TestCreateAiModelSucceedsWhenOwnerRefPatchFails(t *testing.T) {
	client := fake.NewClientset()
	client.PrependReactor("update", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("connection refused")
	})
	crdOps := &fakeAiModelCrdOps{}

	created, err := createAiModelWithManagedSecret(context.Background(), client, crdOps, slog.Default(), testNamespace, "claude", anthropicSpec(""), "sk-123")
	require.NoError(t, err, "a failed ownerRef patch must not fail the create")
	assert.NotNil(t, created)
}

func TestUpdateAiModelWithoutApiKeySkipsSecrets(t *testing.T) {
	client := fake.NewClientset()
	crdOps := &fakeAiModelCrdOps{}

	_, err := updateAiModelWithManagedSecret(context.Background(), client, crdOps, testNamespace, "claude", anthropicSpec("shared-secret"), "")
	require.NoError(t, err)
	assert.Equal(t, 1, crdOps.updateCalls)
	assert.Empty(t, client.Actions())
}

func TestUpdateAiModelFailsWhenModelMissing(t *testing.T) {
	client := fake.NewClientset()
	crdOps := &fakeAiModelCrdOps{getErr: fmt.Errorf("store: aimodel %s/claude not found", testNamespace)}

	_, err := updateAiModelWithManagedSecret(context.Background(), client, crdOps, testNamespace, "claude", anthropicSpec(""), "sk-123")
	require.Error(t, err)
	assert.Equal(t, 0, crdOps.updateCalls)
	assert.Empty(t, client.Actions(), "no secret writes for a missing model")
}

func TestUpdateAiModelProvisionsMissingManagedSecretWithOwner(t *testing.T) {
	// Model was created via GitOps or with a foreign ref; the first apiKey
	// update materializes the managed secret including the ownerRef.
	client := fake.NewClientset()
	existing := aiModelFixture("claude", anthropicSpec("shared-secret"))
	crdOps := &fakeAiModelCrdOps{getModel: existing}

	_, err := updateAiModelWithManagedSecret(context.Background(), client, crdOps, testNamespace, "claude", anthropicSpec(managedAiModelSecretName("claude")), "sk-123")
	require.NoError(t, err)
	assert.Equal(t, 1, crdOps.updateCalls)

	secret := getSecret(t, client, "claude")
	assert.Equal(t, []byte("sk-123"), secret.Data[ai.DefaultApiKeySecretKey])
	require.Len(t, secret.OwnerReferences, 1)
	assert.Equal(t, existing.UID, secret.OwnerReferences[0].UID)
}

func TestUpdateAiModelRotatesKey(t *testing.T) {
	client := fake.NewClientset(managedSecretFixture("claude", "sk-old"))
	crdOps := &fakeAiModelCrdOps{getModel: aiModelFixture("claude", anthropicSpec(managedAiModelSecretName("claude")))}

	_, err := updateAiModelWithManagedSecret(context.Background(), client, crdOps, testNamespace, "claude", anthropicSpec(managedAiModelSecretName("claude")), "sk-new")
	require.NoError(t, err)
	assert.Equal(t, []byte("sk-new"), getSecret(t, client, "claude").Data[ai.DefaultApiKeySecretKey])
}

func TestUpdateAiModelRefusesUnmanagedSecretBeforeCrWrite(t *testing.T) {
	unmanaged := managedSecretFixture("claude", "user-owned")
	unmanaged.Labels = nil
	client := fake.NewClientset(unmanaged)
	crdOps := &fakeAiModelCrdOps{getModel: aiModelFixture("claude", anthropicSpec(""))}

	_, err := updateAiModelWithManagedSecret(context.Background(), client, crdOps, testNamespace, "claude", anthropicSpec(""), "sk-123")
	require.Error(t, err)
	assert.Equal(t, 0, crdOps.updateCalls, "CR update must not run when the secret is refused")
}
