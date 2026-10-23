package reconciler

import (
	"context"
	"mogenius-operator/src/utils"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// reconcilePlatformPatch hands every change of a PlatformPatch to the
// PlatformConfigs referencing it. A patch is only read while its PlatformConfig
// reconciles, and that PlatformConfig gets no event of its own when the patch
// changes, so without a requeue the new values wait for the background sweep.
// Deletes requeue too: the release has to drop what the patch contributed.
// Background sweeps are skipped, they already reconcile every PlatformConfig.
func (d *reconcilerModule) reconcilePlatformPatch(_ context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	if op == backgroundOperation || d.requeue == nil {
		return nil
	}
	name := obj.GetName()
	d.requeue(utils.PlatformConfigResource, func(platformConfig *unstructured.Unstructured) bool {
		spec, _, _ := unstructured.NestedMap(platformConfig.Object, "spec")
		return referencesPlatformPatch(spec, name)
	})
	return nil
}

// referencesPlatformPatch reports whether any `patches` list below node names
// the given PlatformPatch. The spec is walked generically so components added
// to PlatformConfigSpec are covered without being listed here.
func referencesPlatformPatch(node any, name string) bool {
	switch v := node.(type) {
	case map[string]any:
		for key, child := range v {
			if key == "patches" && patchListNames(child, name) {
				return true
			}
			if referencesPlatformPatch(child, name) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if referencesPlatformPatch(child, name) {
				return true
			}
		}
	}
	return false
}

func patchListNames(list any, name string) bool {
	refs, ok := list.([]any)
	if !ok {
		return false
	}
	for _, ref := range refs {
		if m, ok := ref.(map[string]any); ok && m["name"] == name {
			return true
		}
	}
	return false
}
