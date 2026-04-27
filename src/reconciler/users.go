package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/store"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (d *reconcilerModule) reconcileUsers(ctx context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	namespace := d.config.Get("MO_OWN_NAMESPACE")
	grants, err := store.GetAllGrants(namespace)
	if err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("failed to fetch grants for user reconciliation: %w", err)}}
	}

	var results []ReconcileResult
	for _, grant := range grants {
		if grant.Spec.Grantee != obj.GetName() {
			continue
		}
		if op == deleteOperation {
			// User deleted: clean up all their RoleBindings.
			results = append(results, d.deleteGrantBindings(ctx, grant)...)
		} else {
			results = append(results, d.reconcileGrantInternal(ctx, grant)...)
		}
	}
	return results
}
