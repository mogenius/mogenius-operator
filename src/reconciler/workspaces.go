package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/store"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (d *reconcilerModule) reconcileWorkspaces(ctx context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	namespace := d.config.Get("MO_OWN_NAMESPACE")
	grants, err := store.GetAllGrants(namespace)
	if err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("failed to fetch grants for workspace reconciliation: %w", err)}}
	}

	var results []ReconcileResult
	for _, grant := range grants {
		if grant.Spec.TargetName != obj.GetName() || grant.Spec.TargetType != "workspace" {
			continue
		}
		if op == deleteOperation {
			// Workspace deleted: clean up all grants targeting it.
			results = append(results, d.deleteGrantBindings(ctx, grant)...)
		} else {
			results = append(results, d.reconcileGrantInternal(ctx, grant)...)
		}
	}
	return results
}
