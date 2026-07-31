package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/utils"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func (d *reconcilerModule) reconcileUIConfigs(ctx context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	if op == deleteOperation {
		return nil
	}
	return d.verifyUIConfigName(ctx, obj)
}

// expectedUIConfigName derives the deterministic metadata.name for a resource
// type: "<kind>.<group>" (lowercase; core resources without a group are just
// "<kind>"). Must stay in sync with MoKubernetesDtoUtils.uiConfigNameForReference
// in the client SDK, which the platform uses for its direct lookups.
func expectedUIConfigName(reference v1alpha1.CrdReference) string {
	group := ""
	if idx := strings.Index(reference.ApiVersion, "/"); idx >= 0 {
		group = reference.ApiVersion[:idx]
	}
	name := strings.ToLower(reference.Kind)
	if group != "" {
		name += "." + strings.ToLower(group)
	}
	return name
}

// verifyUIConfigName checks that metadata.name follows the deterministic naming
// scheme and reports the result through the NameValid status condition. A
// mismatching config is never found by the platform's name-based lookups, so it
// silently does not apply — the condition makes that visible.
func (d *reconcilerModule) verifyUIConfigName(ctx context.Context, obj *unstructured.Unstructured) []ReconcileResult {
	var uiConfig v1alpha1.UIConfig
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &uiConfig); err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("failed to parse UIConfig: %w", err)}}
	}
	results := []ReconcileResult{}

	expected := expectedUIConfigName(uiConfig.Spec.Reference)
	nameCondition := metav1.Condition{
		Type:               v1alpha1.UIConfigConditionNameValid,
		Status:             metav1.ConditionTrue,
		Reason:             "NameMatchesReference",
		Message:            "metadata.name matches the deterministic name derived from spec.reference",
		ObservedGeneration: uiConfig.Generation,
	}
	if uiConfig.Name != expected {
		nameCondition.Status = metav1.ConditionFalse
		nameCondition.Reason = "NameMismatch"
		nameCondition.Message = fmt.Sprintf("metadata.name must be %q (derived from spec.reference %s/%s); the platform will not find this config under %q", expected, uiConfig.Spec.Reference.ApiVersion, uiConfig.Spec.Reference.Kind, uiConfig.Name)
		results = append(results, ReconcileResult{Err: fmt.Errorf("UIConfig %q does not follow the deterministic naming scheme, expected %q", uiConfig.Name, expected)})
	}

	// Cluster-scoped resource, hence the empty namespace.
	if err := d.setStatusConditions(ctx, utils.UIConfigResource, "", uiConfig.Name, uiConfig.Status.Conditions, nameCondition); err != nil {
		results = append(results, ReconcileResult{Err: fmt.Errorf("failed to update UIConfig status: %w", err), IsWarning: true})
	}
	return results
}
