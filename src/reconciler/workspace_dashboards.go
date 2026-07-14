package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/utils"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func (d *reconcilerModule) reconcileWorkspaceDashboards(ctx context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	var results []ReconcileResult

	if op != deleteOperation {
		results = append(results, d.verifyWorkspaceDashboardIntegrity(ctx, obj)...)
	}
	return results
}

func (d *reconcilerModule) verifyWorkspaceDashboardIntegrity(ctx context.Context, obj *unstructured.Unstructured) []ReconcileResult {
	var dashboard v1alpha1.WorkspaceDashboard
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &dashboard); err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("failed to parse WorkspaceDashboard: %w", err)}}
	}
	results := []ReconcileResult{}

	available, err := kubernetes.GetAvailableResources()
	if err != nil {
		return append(results, ReconcileResult{Err: fmt.Errorf("failed to discover available resources: %w", err), IsWarning: true})
	}
	availableKinds := make(map[string]struct{}, len(available))
	for _, resource := range available {
		availableKinds[resource.ApiVersion+"/"+resource.Kind] = struct{}{}
	}

	// Collect referenced kinds that are not served by the cluster, deduped so
	// the condition message stays readable for dashboards reusing a kind in
	// several components.
	references := append([]v1alpha1.CrdReference{}, dashboard.Spec.ResourceOverview.Resources...)
	for _, table := range dashboard.Spec.ResourceTables {
		references = append(references, table.Resources...)
	}
	missing := []string{}
	seen := map[string]struct{}{}
	for _, reference := range references {
		key := reference.ApiVersion + "/" + reference.Kind
		if _, ok := availableKinds[key]; ok {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		missing = append(missing, key)
	}

	resourcesCondition := metav1.Condition{
		Type:               v1alpha1.WorkspaceDashboardConditionResourcesValid,
		Status:             metav1.ConditionTrue,
		Reason:             "AllResourcesFound",
		Message:            "All referenced resource types exist in the cluster",
		ObservedGeneration: dashboard.Generation,
	}
	if len(missing) > 0 {
		resourcesCondition.Status = metav1.ConditionFalse
		resourcesCondition.Reason = "ResourcesNotFound"
		resourcesCondition.Message = fmt.Sprintf("Referenced resource types do not exist in the cluster: %s", strings.Join(missing, ", "))
		resultErr := ReconcileResult{}
		resultErr.Err = fmt.Errorf("WorkspaceDashboard references resource types which do not exist in the cluster: %s", strings.Join(missing, ", "))
		results = append(results, resultErr)
	}
	if err := d.setStatusConditions(ctx, utils.WorkspaceDashboardResource, dashboard.Namespace, dashboard.Name, dashboard.Status.Conditions, resourcesCondition); err != nil {
		results = append(results, ReconcileResult{Err: fmt.Errorf("failed to update WorkspaceDashboard status: %w", err), IsWarning: true})
	}
	return results
}