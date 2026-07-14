package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/store"
	"mogenius-operator/src/utils"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func (d *reconcilerModule) reconcileWorkspaces(ctx context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	var results []ReconcileResult

	if op != deleteOperation {
		results = append(results, d.verifyWorkspaceIntegrity(ctx, obj)...)
	}
	return results
}

func (d *reconcilerModule) verifyWorkspaceIntegrity(ctx context.Context, obj *unstructured.Unstructured) []ReconcileResult {
	var workspace v1alpha1.Workspace
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &workspace); err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("failed to parse Workspace: %w", err)}}
	}
	results := []ReconcileResult{}

	dashboardRefCondition := metav1.Condition{
		Type:               v1alpha1.WorkspaceConditionDashboardRefValid,
		Status:             metav1.ConditionTrue,
		Reason:             "NoReference",
		Message:            "No WorkspaceDashboard referenced",
		ObservedGeneration: workspace.Generation,
	}
	if ref := workspace.Spec.DashboardRef; ref != "" {
		dashboard, err := store.GetWorkspaceDashboard(obj.GetNamespace(), ref)
		switch {
		case err != nil:
			// A store lookup failure says nothing about whether the dashboard
			// exists — report Unknown instead of a false NotFound.
			dashboardRefCondition.Status = metav1.ConditionUnknown
			dashboardRefCondition.Reason = "LookupFailed"
			dashboardRefCondition.Message = fmt.Sprintf("Failed to look up WorkspaceDashboard %q: %v", ref, err)
			results = append(results, ReconcileResult{Err: fmt.Errorf("failed to look up WorkspaceDashboard %q: %w", ref, err), IsWarning: true})
		case dashboard == nil:
			dashboardRefCondition.Status = metav1.ConditionFalse
			dashboardRefCondition.Reason = "NotFound"
			dashboardRefCondition.Message = fmt.Sprintf("WorkspaceDashboard %q does not exist", ref)
			resultErr := ReconcileResult{}
			resultErr.Err = fmt.Errorf("Workspace references a WorkspaceDashboard which does not exist: %q", ref)
			results = append(results, resultErr)
		default:
			dashboardRefCondition.Reason = "Found"
			dashboardRefCondition.Message = fmt.Sprintf("WorkspaceDashboard %q exists", ref)
		}
	}

	// Everything the resource loop below appends is a resource integrity
	// problem; remember the offset so the ResourcesValid condition can be
	// derived from those results without duplicating the checks.
	resourceCheckStart := len(results)

	// Workspaces frequently reference the same namespace from several resource
	// entries (e.g. multiple helm releases deployed into one namespace). Memo
	// the lookup result per namespace string so we hit Valkey at most once per
	// distinct namespace within this workspace. The sentinel struct{} for nil
	// errors lets us distinguish "not yet looked up" from "looked up, no error".
	type lookupResult struct{ err error }
	namespaceLookups := map[string]lookupResult{}
	checkNamespace := func(ns string) error {
		if cached, ok := namespaceLookups[ns]; ok {
			return cached.err
		}
		_, err := GetNamespace(ns, &d.valkeyClient, d.logger)
		namespaceLookups[ns] = lookupResult{err: err}
		return err
	}

	for _, resource := range workspace.Spec.Resources {
		switch resource.Type {
		case "namespace":
			namespace := resource.Id
			if namespace == "" {
				resourceErr := ReconcileResult{}
				resourceErr.Err = fmt.Errorf("Workspace contains a resource of type 'namespace' which does not specifiy the namespace name in resource.Id")
				results = append(results, resourceErr)
			}
			if err := checkNamespace(namespace); err != nil {
				resourceErr := ReconcileResult{}
				resourceErr.Err = fmt.Errorf("Workspace contains a resource of type 'namespace' pointing to a namespace which does not exist: %#v, %w", namespace, err)
				results = append(results, resourceErr)
			}
		case "helm":
			namespace := resource.Namespace
			if namespace == "" {
				resourceErr := ReconcileResult{}
				resourceErr.Err = fmt.Errorf("Workspace contains a resource of type 'helm' which does not specifiy the namespace name in resource.Namespace")
				results = append(results, resourceErr)
			}
			if err := checkNamespace(namespace); err != nil {
				resourceErr := ReconcileResult{}
				resourceErr.Err = fmt.Errorf("Workspace contains a resource of type 'helm' pointing to a namespace which does not exist: %#v, %w", namespace, err)
				results = append(results, resourceErr)
			}
		case "argocd":
			// No integrity checks for ArgoCD resources yet, as they don't have any cluster-level representation.
			continue
		default:
			resourceErr := ReconcileResult{}
			resourceErr.Err = fmt.Errorf("Workspace contains a resource with the invalid type: %#v", resource.Type)
			results = append(results, resourceErr)
		}
	}

	resourcesCondition := metav1.Condition{
		Type:               v1alpha1.WorkspaceConditionResourcesValid,
		Status:             metav1.ConditionTrue,
		Reason:             "AllResourcesFound",
		Message:            "All referenced resources exist in the cluster",
		ObservedGeneration: workspace.Generation,
	}
	if problems := results[resourceCheckStart:]; len(problems) > 0 {
		messages := make([]string, 0, len(problems))
		for _, problem := range problems {
			messages = append(messages, problem.Err.Error())
		}
		resourcesCondition.Status = metav1.ConditionFalse
		resourcesCondition.Reason = "ResourcesNotFound"
		resourcesCondition.Message = strings.Join(messages, "; ")
	}

	if err := d.setStatusConditions(ctx, utils.WorkspaceResource, workspace.Namespace, workspace.Name, workspace.Status.Conditions, resourcesCondition, dashboardRefCondition); err != nil {
		results = append(results, ReconcileResult{Err: fmt.Errorf("failed to update Workspace status: %w", err), IsWarning: true})
	}
	return results
}

