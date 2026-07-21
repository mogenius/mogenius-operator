package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"mogenius-operator/src/ai"
	"mogenius-operator/src/crds/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

// ╭─────────────────────────────────╮
// │ AiModel: managed API key secret │
// ╰─────────────────────────────────╯
//
// The create/update aimodel handlers accept an optional plaintext apiKey. When
// set, the operator materializes it as a Secret in its own namespace and wires
// spec.apiKeySecretRef to it, so callers don't have to provision the Secret
// themselves. The explicit apiKeySecretRef path (GitOps, keys shared between
// models) stays untouched. Managed secrets carry an OwnerReference to their
// AiModel, so deleting the model garbage-collects the secret.
//
// The plaintext key must never leave this file except inside Secret.Data: not
// in error messages, not in log attributes, not in audit objects.

const (
	aiModelSecretManagedByLabelKey   = "app.kubernetes.io/managed-by"
	aiModelSecretManagedByLabelValue = "mogenius-k8s-manager"
	aiModelSecretModelLabelKey       = "mogenius.com/aimodel"
)

// truncateWithHash shortens s to at most maxLen by cutting it and appending a
// 10-hex-char sha256 suffix, keeping the result deterministic and
// collision-safe. s is returned unchanged when it already fits.
func truncateWithHash(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	hash := sha256.Sum256([]byte(s))
	suffix := hex.EncodeToString(hash[:])[:10]
	return s[:maxLen-len(suffix)-1] + "-" + suffix
}

// managedAiModelSecretName returns the name of the Secret the operator manages
// for the given model: "aimodel-<name>-api-key", hash-truncated if the model
// name would push it past the DNS-1123 subdomain limit.
func managedAiModelSecretName(modelName string) string {
	const prefix, suffix = "aimodel-", "-api-key"
	middle := truncateWithHash(modelName, validation.DNS1123SubdomainMaxLength-len(prefix)-len(suffix))
	return prefix + middle + suffix
}

// applyManagedApiKeyRef enforces the apiKey/apiKeySecretRef exclusivity rule
// and wires spec.apiKeySecretRef to the managed secret. With an empty apiKey
// the spec passes through untouched. A ref that already points at the managed
// secret is tolerated, because clients round-trip the spec they fetched via
// GET when updating; only a ref to a different secret conflicts.
func applyManagedApiKeyRef(modelName string, spec *v1alpha1.AiModelSpec, apiKey string) error {
	if apiKey == "" {
		return nil
	}
	managedName := managedAiModelSecretName(modelName)
	if spec.ApiKeySecretRef != nil && spec.ApiKeySecretRef.Name != "" && spec.ApiKeySecretRef.Name != managedName {
		return fmt.Errorf("apiKey and spec.apiKeySecretRef (%q) are mutually exclusive: pass the key or a secret reference, not both", spec.ApiKeySecretRef.Name)
	}
	spec.ApiKeySecretRef = &v1alpha1.SecretKeyRef{
		Name: managedName,
		Key:  ai.DefaultApiKeySecretKey,
	}
	return nil
}

func aiModelOwnerReference(model *v1alpha1.AiModel) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "mogenius.com/v1alpha1",
		Kind:       "AiModel",
		Name:       model.Name,
		UID:        model.UID,
	}
}

func hasAiModelOwnerReference(secret *corev1.Secret, model *v1alpha1.AiModel) bool {
	for _, ref := range secret.OwnerReferences {
		if ref.UID == model.UID {
			return true
		}
	}
	return false
}

// isManagedAiModelSecret reports whether the operator owns this secret and may
// therefore rewrite or delete it.
func isManagedAiModelSecret(secret *corev1.Secret) bool {
	return secret.Labels[aiModelSecretManagedByLabelKey] == aiModelSecretManagedByLabelValue
}

// upsertManagedAiModelSecret creates or updates the managed secret for
// modelName, storing apiKey under the default data key. A same-name secret
// that lacks the managed-by label is refused, never overwritten. When owner is
// non-nil its OwnerReference is (re-)asserted; nil is used on create, where
// the CR (and its UID) doesn't exist yet. createdFresh reports whether this
// call created the secret, so a failed CR create can roll back exactly what it
// provisioned.
func upsertManagedAiModelSecret(ctx context.Context, client kubernetes.Interface, namespace string, modelName string, apiKey string, owner *v1alpha1.AiModel) (createdFresh bool, err error) {
	secretName := managedAiModelSecretName(modelName)
	secrets := client.CoreV1().Secrets(namespace)

	_, err = secrets.Get(ctx, secretName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("get secret %q: %w", secretName, err)
	}
	if apierrors.IsNotFound(err) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
				Labels: map[string]string{
					aiModelSecretManagedByLabelKey: aiModelSecretManagedByLabelValue,
					aiModelSecretModelLabelKey:     truncateWithHash(modelName, validation.LabelValueMaxLength),
				},
			},
			Data: map[string][]byte{ai.DefaultApiKeySecretKey: []byte(apiKey)},
		}
		if owner != nil {
			secret.OwnerReferences = []metav1.OwnerReference{aiModelOwnerReference(owner)}
		}
		_, err = secrets.Create(ctx, secret, metav1.CreateOptions{})
		if err == nil {
			return true, nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return false, fmt.Errorf("create secret %q: %w", secretName, err)
		}
		// Lost a create race; fall through to the update path.
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := secrets.Get(ctx, secretName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if !isManagedAiModelSecret(current) {
			return fmt.Errorf("secret %q already exists and is not managed by mogenius; pick a different model name or provision the secret yourself via spec.apiKeySecretRef", secretName)
		}
		if current.Data == nil {
			current.Data = map[string][]byte{}
		}
		current.Data[ai.DefaultApiKeySecretKey] = []byte(apiKey)
		current.Labels[aiModelSecretModelLabelKey] = truncateWithHash(modelName, validation.LabelValueMaxLength)
		if owner != nil && !hasAiModelOwnerReference(current, owner) {
			current.OwnerReferences = append(current.OwnerReferences, aiModelOwnerReference(owner))
		}
		_, err = secrets.Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return false, fmt.Errorf("update secret %q: %w", secretName, err)
	}
	return false, nil
}

// setAiModelOwnerReference adds the model's OwnerReference to its managed
// secret after the CR create resolved the UID.
func setAiModelOwnerReference(ctx context.Context, client kubernetes.Interface, namespace string, secretName string, model *v1alpha1.AiModel) error {
	secrets := client.CoreV1().Secrets(namespace)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := secrets.Get(ctx, secretName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if hasAiModelOwnerReference(current, model) {
			return nil
		}
		current.OwnerReferences = append(current.OwnerReferences, aiModelOwnerReference(model))
		_, err = secrets.Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
}

// deleteManagedAiModelSecret removes the managed secret for modelName. A
// missing secret is fine; an unmanaged same-name secret is left untouched.
func deleteManagedAiModelSecret(ctx context.Context, client kubernetes.Interface, namespace string, modelName string) error {
	secretName := managedAiModelSecretName(modelName)
	secrets := client.CoreV1().Secrets(namespace)

	current, err := secrets.Get(ctx, secretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get secret %q: %w", secretName, err)
	}
	if !isManagedAiModelSecret(current) {
		return fmt.Errorf("secret %q is not managed by mogenius, refusing to delete it", secretName)
	}
	err = secrets.Delete(ctx, secretName, metav1.DeleteOptions{
		Preconditions: &metav1.Preconditions{UID: &current.UID},
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete secret %q: %w", secretName, err)
	}
	return nil
}

// aiModelCrdOps is the minimal AiModel CR surface the managed-secret
// orchestration needs; production code satisfies it with an adapter over
// MogeniusV1alpha1 (see workspacemanager.go), tests with a stub.
type aiModelCrdOps interface {
	Create(namespace string, name string, spec v1alpha1.AiModelSpec) (*v1alpha1.AiModel, error)
	Update(namespace string, name string, spec v1alpha1.AiModelSpec) (*v1alpha1.AiModel, error)
	Get(namespace string, name string) (*v1alpha1.AiModel, error)
}

// createAiModelWithManagedSecret creates the AiModel CR, provisioning the
// managed API key secret first when apiKey is set — the secret must exist
// before the CR, because both spec validation and the reconciler's Ready
// condition resolve the reference. The OwnerReference is patched onto the
// secret afterwards (the CR's UID only exists after the create).
func createAiModelWithManagedSecret(ctx context.Context, k8s kubernetes.Interface, crdOps aiModelCrdOps, logger *slog.Logger, namespace string, name string, spec v1alpha1.AiModelSpec, apiKey string) (*v1alpha1.AiModel, error) {
	if apiKey == "" {
		return crdOps.Create(namespace, name, spec)
	}

	secretName := managedAiModelSecretName(name)
	createdFresh, err := upsertManagedAiModelSecret(ctx, k8s, namespace, name, apiKey, nil)
	if err != nil {
		return nil, fmt.Errorf("provision api key secret: %w", err)
	}

	created, err := crdOps.Create(namespace, name, spec)
	if err != nil {
		rollbackManagedAiModelSecret(ctx, k8s, crdOps, logger, namespace, name, secretName, createdFresh, err)
		return nil, err
	}

	if ownerErr := setAiModelOwnerReference(ctx, k8s, namespace, secretName, created); ownerErr != nil {
		// The model is fully functional without the ownerRef; the secret just
		// won't be garbage-collected on delete. The next apiKey update
		// re-asserts it, so don't fail the create over this.
		logger.Warn("aimodel created, but setting the owner reference on its api key secret failed; the secret will not be garbage-collected with the model",
			"model", name, "secret", secretName, "error", ownerErr)
	}
	return created, nil
}

// rollbackManagedAiModelSecret is the best-effort cleanup after a failed CR
// create: delete the secret only if this request created it and no existing
// model uses it. A pre-existing managed secret is kept — its key value was
// already rotated by the upsert and may belong to an earlier model generation.
func rollbackManagedAiModelSecret(ctx context.Context, k8s kubernetes.Interface, crdOps aiModelCrdOps, logger *slog.Logger, namespace string, name string, secretName string, createdFresh bool, createErr error) {
	if !createdFresh {
		logger.Warn("aimodel create failed after its managed api key secret was updated; the new key value stays in place",
			"model", name, "secret", secretName, "error", createErr)
		return
	}
	if apierrors.IsAlreadyExists(createErr) {
		// A concurrent create won the race. If the winner references the
		// managed secret, both racers wrote the same secret — keep it.
		existing, err := crdOps.Get(namespace, name)
		if err == nil && existing != nil && existing.Spec.ApiKeySecretRef != nil && existing.Spec.ApiKeySecretRef.Name == secretName {
			return
		}
	}
	if err := deleteManagedAiModelSecret(ctx, k8s, namespace, name); err != nil {
		logger.Warn("failed to clean up api key secret after aimodel create failed",
			"model", name, "secret", secretName, "error", err)
	}
}

// updateAiModelWithManagedSecret updates the AiModel CR, rotating (or first
// provisioning) the managed API key secret when apiKey is set. The existing CR
// is fetched up front so the secret gets its OwnerReference in one step. If
// the CR update fails after the secret write, the new key value stays — with
// the model already pointing at the managed secret the rotation effectively
// landed, and otherwise the secret is unused and garbage-collected with the
// model.
func updateAiModelWithManagedSecret(ctx context.Context, k8s kubernetes.Interface, crdOps aiModelCrdOps, namespace string, name string, spec v1alpha1.AiModelSpec, apiKey string) (*v1alpha1.AiModel, error) {
	if apiKey == "" {
		return crdOps.Update(namespace, name, spec)
	}

	existing, err := crdOps.Get(namespace, name)
	if err != nil {
		return nil, err
	}
	if _, err := upsertManagedAiModelSecret(ctx, k8s, namespace, name, apiKey, existing); err != nil {
		return nil, fmt.Errorf("provision api key secret: %w", err)
	}
	return crdOps.Update(namespace, name, spec)
}
