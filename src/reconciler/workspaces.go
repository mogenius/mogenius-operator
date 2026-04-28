package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"

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

	for _, resource := range workspace.Spec.Resources {
		switch resource.Type {
		case "namespace":
			namespace := resource.Id
			if namespace == "" {
				resourceErr := ReconcileResult{}
				resourceErr.Err = fmt.Errorf("Workspace contains a resource of type 'namespace' which does not specifiy the namespace name in resource.Id")
				results = append(results, resourceErr)
			}
			_, err := GetNamespace(namespace, &d.valkeyClient, d.logger)
			if err != nil {
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
			_, err := GetNamespace(namespace, &d.valkeyClient, d.logger)
			if err != nil {
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
	return results
}
