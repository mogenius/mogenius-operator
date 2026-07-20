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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func (d *reconcilerModule) reconcileWorkspaceDashboards(ctx context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	var results []ReconcileResult

	if op != deleteOperation {
		results = append(results, d.verifyWorkspaceDashboardIntegrity(ctx, obj)...)
	}

	// A deleted dashboard must not leave dangling references behind: clear
	// spec.dashboardRef on every workspace that pointed at it.
	if op == deleteOperation {
		d.clearWorkspaceDashboardRefs(ctx, obj)
	}

	// A dashboard appearing or disappearing flips the DashboardRefValid
	// condition of every workspace referencing it, but those workspaces get no
	// event of their own — without a requeue their condition would stay stale
	// until the next background sweep (up to 15 minutes). On delete this also
	// surfaces any workspace whose reference could not be cleared above.
	// Updates are skipped: they can't change whether the dashboard exists.
	if op == createOperation || op == deleteOperation {
		d.requeueReferencingWorkspaces(obj)
	}
	return results
}

// clearWorkspaceDashboardRefs removes spec.dashboardRef from every workspace
// in the deleted dashboard's namespace that still references it. Errors are
// logged instead of returned: results recorded for a deleted object would
// linger in the reconciler status forever, and a workspace whose reference
// could not be cleared is visible through its DashboardRefValid condition.
func (d *reconcilerModule) clearWorkspaceDashboardRefs(ctx context.Context, dashboard *unstructured.Unstructured) {
	name := dashboard.GetName()
	namespace := dashboard.GetNamespace()

	gv, err := schema.ParseGroupVersion(utils.WorkspaceResource.ApiVersion)
	if err != nil {
		d.logger.Error("clear dashboard refs: failed to parse group version", "error", err)
		return
	}
	client := d.clientProvider.DynamicClient().Resource(gv.WithResource(utils.WorkspaceResource.Plural)).Namespace(namespace)

	// List live instead of via the watcher-fed store: a lagging store would
	// silently skip a workspace and leave its dangling reference in place.
	workspaces, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		d.logger.Error("clear dashboard refs: failed to list workspaces", "dashboard", name, "error", err)
		return
	}

	// Merge-patching the reference to null removes the field entirely.
	patch := []byte(`{"spec":{"dashboardRef":null}}`)
	for _, workspace := range workspaces.Items {
		ref, _, _ := unstructured.NestedString(workspace.Object, "spec", "dashboardRef")
		if ref != name {
			continue
		}
		if _, err := client.Patch(ctx, workspace.GetName(), types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
			d.logger.Error("clear dashboard refs: failed to clear workspace reference", "workspace", workspace.GetName(), "dashboard", name, "error", err)
			continue
		}
		d.logger.Info("cleared dashboardRef after dashboard deletion", "workspace", workspace.GetName(), "dashboard", name)
	}
}

func (d *reconcilerModule) requeueReferencingWorkspaces(dashboard *unstructured.Unstructured) {
	if d.requeue == nil {
		return
	}
	name := dashboard.GetName()
	namespace := dashboard.GetNamespace()
	d.requeue(utils.WorkspaceResource, func(workspace *unstructured.Unstructured) bool {
		if workspace.GetNamespace() != namespace {
			return false
		}
		ref, _, _ := unstructured.NestedString(workspace.Object, "spec", "dashboardRef")
		return ref == name
	})
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
	// several tables.
	references := []v1alpha1.CrdReference{}
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