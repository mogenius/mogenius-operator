package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/store"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (d *reconcilerModule) reconcileNamespaces(ctx context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	// A new namespace may unblock previously-pending grants; re-reconcile all.
	namespace := d.config.Get("MO_OWN_NAMESPACE")
	grants, err := store.GetAllGrants(namespace)
	if err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("failed to fetch grants for namespace reconciliation: %w", err)}}
	}

	var results []ReconcileResult
	for _, grant := range grants {
		results = append(results, d.reconcileGrantInternal(ctx, grant)...)
	}
	return results
}
