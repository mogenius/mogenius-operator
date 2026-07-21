package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/ai"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/store"
	"mogenius-operator/src/utils"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const aiModelReadyCondition = "Ready"

// reconcileAiModels validates AiModel CRs and reports the result as a "Ready"
// status condition, so declaratively managed model configs (kubectl/GitOps)
// get feedback without the UI: `kubectl get aimodels -n mogenius` shows READY
// and REASON columns. Resolution itself stays fail-closed at run time; the
// condition is purely informational.
func (d *reconcilerModule) reconcileAiModels(ctx context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	if op == deleteOperation {
		// The deleted model may have been the effective default; re-evaluate
		// the remaining default-flagged models so a stale DuplicateDefault
		// condition doesn't linger until the next background sweep.
		d.requeueDefaultAiModels(obj.GetNamespace(), obj.GetName())
		return nil
	}

	var model v1alpha1.AiModel
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &model); err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("failed to parse AiModel: %w", err)}}
	}

	conditionStatus, reason, message := d.evaluateAiModel(ctx, &model)

	// Usage reset request: a changed "mogenius.com/reset-usage-at" annotation
	// resets today's recorded usage exactly once per distinct value (same
	// Flux-style pattern as the agent run trigger). Handled independently of
	// the Ready condition — usage may need clearing even on a broken spec.
	resetHandled := false
	if requested := model.Annotations[v1alpha1.AiModelResetUsageAtAnnotation]; requested != "" && requested != model.Status.LastUsageResetAt {
		cleared, err := d.aiManager.ResetTokenUsageForModel(model.Name)
		if err != nil {
			// Leave LastUsageResetAt unchanged so the next reconcile retries.
			d.logger.Error("Failed to reset AiModel token usage", "aimodel", model.Name, "error", err)
		} else {
			model.Status.LastUsageResetAt = requested
			resetHandled = true
			d.logger.Info("AiModel token usage reset", "aimodel", model.Name, "clearedTokens", cleared)
			d.emitAiModelEvent(ctx, &model, "UsageReset", fmt.Sprintf("Cleared %d tokens of today's recorded usage", cleared))
		}
	}

	current := apimeta.FindStatusCondition(model.Status.Conditions, aiModelReadyCondition)
	upToDate := current != nil &&
		current.Status == conditionStatus &&
		current.Reason == reason &&
		current.Message == message &&
		model.Status.ObservedGeneration == model.Generation
	if !upToDate || resetHandled {
		apimeta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
			Type:               aiModelReadyCondition,
			Status:             conditionStatus,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: model.Generation,
		})
		model.Status.ObservedGeneration = model.Generation
		if _, err := d.clientProvider.MogeniusClientSet().MogeniusV1alpha1.UpdateAiModelStatus(&model); err != nil {
			return []ReconcileResult{{Err: fmt.Errorf("failed to update status of aimodel %q: %w", model.Name, err)}}
		}
	}
	if !upToDate {
		// This model's state changed, which may flip the default election for
		// its peers (e.g. default flag toggled). Requeueing only on an actual
		// condition transition (never on usage resets) keeps mutual requeues
		// between default-flagged models from ping-ponging: the second pass
		// finds everything up-to-date and stops.
		d.requeueDefaultAiModels(model.Namespace, model.Name)
	}

	// Surface user configuration problems as warnings in the reconciler
	// status API as well — the condition alone is easy to miss.
	if conditionStatus == metav1.ConditionFalse {
		return []ReconcileResult{{Err: fmt.Errorf("aimodel %q is not ready: %s: %s", model.Name, reason, message), IsWarning: true}}
	}
	return nil
}

// evaluateAiModel computes the Ready condition for an AiModel. Fail reasons
// are CamelCase identifiers per the metav1.Condition convention.
func (d *reconcilerModule) evaluateAiModel(ctx context.Context, model *v1alpha1.AiModel) (metav1.ConditionStatus, string, string) {
	ownNamespace := d.config.Get("MO_OWN_NAMESPACE")
	if model.Namespace != ownNamespace {
		return metav1.ConditionFalse, "IgnoredNamespace", fmt.Sprintf("aimodels are only processed in namespace %q", ownNamespace)
	}

	if err := ai.ValidateAiModelSpec(model.Spec); err != nil {
		return metav1.ConditionFalse, "InvalidSpec", err.Error()
	}

	if ref := model.Spec.ApiKeySecretRef; ref != nil && ref.Name != "" {
		// Store cache first, then a direct API read — mirrors how the key is
		// resolved at run time, so the condition matches actual behavior.
		secret := store.GetSecret(model.Namespace, ref.Name)
		if secret == nil {
			var err error
			secret, err = d.clientProvider.K8sClientSet().CoreV1().Secrets(model.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
			if err != nil || secret == nil {
				return metav1.ConditionFalse, "SecretNotFound", fmt.Sprintf("spec.apiKeySecretRef references secret %q which does not exist", ref.Name)
			}
		}
		key := ref.Key
		if key == "" {
			key = ai.DefaultApiKeySecretKey
		}
		if data, exists := secret.Data[key]; !exists || len(data) == 0 {
			return metav1.ConditionFalse, "KeyNotFound", fmt.Sprintf("secret %q has no data key %q", ref.Name, key)
		}
	}

	if model.Spec.Default {
		// The API write path rejects a second default, but kubectl/GitOps can
		// still create one. Deterministically only the election losers are
		// flagged, so exactly one model stays Ready as the effective default.
		if models, err := store.GetAllAiModels(ownNamespace); err == nil {
			if winner := ai.PickDefaultAiModel(models); winner != nil && winner.Name != model.Name {
				return metav1.ConditionFalse, "DuplicateDefault",
					fmt.Sprintf("AiModel %q is also marked as default and wins the election (oldest first); unset spec.default on one of them", winner.Name)
			}
		}
		return metav1.ConditionTrue, "Valid", "spec is valid; model is the cluster default"
	}
	return metav1.ConditionTrue, "Valid", "spec is valid"
}

// emitAiModelEvent writes a best-effort Kubernetes Event on the AiModel so
// kubectl users get visible feedback (kubectl describe aimodel / get events)
// without a status field. Failures are logged, never propagated.
func (d *reconcilerModule) emitAiModelEvent(ctx context.Context, model *v1alpha1.AiModel, reason, message string) {
	now := metav1.Now()
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: model.Name + ".",
			Namespace:    model.Namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			APIVersion: "mogenius.com/v1alpha1",
			Kind:       "AiModel",
			Namespace:  model.Namespace,
			Name:       model.Name,
			UID:        model.UID,
		},
		Reason:         reason,
		Message:        message,
		Type:           corev1.EventTypeNormal,
		Source:         corev1.EventSource{Component: "mogenius-k8s-manager"},
		FirstTimestamp: now,
		LastTimestamp:  now,
		Count:          1,
	}
	if _, err := d.clientProvider.K8sClientSet().CoreV1().Events(model.Namespace).Create(ctx, event, metav1.CreateOptions{}); err != nil {
		d.logger.Warn("Failed to emit AiModel event", "aimodel", model.Name, "reason", reason, "error", err)
	}
}

// requeueDefaultAiModels re-reconciles every other default-flagged AiModel in
// the namespace — their DuplicateDefault condition depends on the default
// election, which changes with this model's create/update/delete, but they
// get no watch event of their own.
func (d *reconcilerModule) requeueDefaultAiModels(namespace string, excludeName string) {
	if d.requeue == nil {
		return
	}
	d.requeue(utils.AiModelResource, func(model *unstructured.Unstructured) bool {
		if model.GetNamespace() != namespace || model.GetName() == excludeName {
			return false
		}
		isDefault, _, _ := unstructured.NestedBool(model.Object, "spec", "default")
		return isDefault
	})
}
