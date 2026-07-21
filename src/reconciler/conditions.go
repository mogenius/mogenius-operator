package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-operator/src/utils"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// setStatusConditions patches a resource's status subresource when any of the
// given conditions differs from the stored ones. Writing only on change keeps
// the watch/reconcile cycle from looping on its own status updates.
func (d *reconcilerModule) setStatusConditions(ctx context.Context, resource utils.ResourceDescriptor, namespace string, name string, current []metav1.Condition, newConditions ...metav1.Condition) error {
	conditions := append([]metav1.Condition{}, current...)
	changed := false
	for _, condition := range newConditions {
		if meta.SetStatusCondition(&conditions, condition) {
			changed = true
		}
	}
	if !changed {
		return nil
	}

	patch, err := json.Marshal(map[string]any{"status": map[string]any{"conditions": conditions}})
	if err != nil {
		return fmt.Errorf("marshal status patch: %w", err)
	}

	gv, err := schema.ParseGroupVersion(resource.ApiVersion)
	if err != nil {
		return fmt.Errorf("parse group version: %w", err)
	}
	_, err = d.clientProvider.DynamicClient().
		Resource(gv.WithResource(resource.Plural)).
		Namespace(namespace).
		Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{}, "status")
	if err != nil {
		return fmt.Errorf("patch %s status: %w", resource.Kind, err)
	}
	return nil
}