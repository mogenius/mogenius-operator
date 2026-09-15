package reconciler

import (
	"context"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/utils"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func (d *reconcilerModule) reconcileNamespaces(_ context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	var results []ReconcileResult

	if op == updateOperation || op == backgroundOperation {
		// only trigger workspace reconciler if namespace gets created or deleted
		// workspaces have their own reconciler that would handle update through update or background reconciles
		return results
	}

	namespaceName := obj.GetName()
	d.requeue(utils.WorkspaceResource, func(obj *unstructured.Unstructured) bool {
		var workspace v1alpha1.Workspace
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &workspace); err != nil {
			return false
		}

		for _, resource := range workspace.Spec.Resources {
			if resource.Type == "namespace" && resource.Id == namespaceName {
				return true
			}
		}

		return false
	})
	return results
}
