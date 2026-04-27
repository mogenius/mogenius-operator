package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/store"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (d *reconcilerModule) reconcileClusterRoles(ctx context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	// Determine the logical role name from label, falling back to object name.
	roleName := obj.GetName()
	if labelVal, ok := obj.GetLabels()[labelRoleName]; ok && labelVal != "" {
		roleName = labelVal
	}

	namespace := d.config.Get("MO_OWN_NAMESPACE")
	grants, err := store.GetAllGrants(namespace)
	if err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("failed to fetch grants for clusterrole reconciliation: %w", err)}}
	}

	var results []ReconcileResult
	for _, grant := range grants {
		if grant.Spec.Role == roleName || grant.Spec.Role == obj.GetName() {
			results = append(results, d.reconcileGrantInternal(ctx, grant)...)
		}
	}
	return results
}
