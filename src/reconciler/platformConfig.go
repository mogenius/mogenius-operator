package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

var platformConfigGVR = schema.GroupVersionResource{
	Group:    "mogenius.com",
	Version:  "v1alpha1",
	Resource: "platformconfigs",
}

const componentCertManager = "cert-manager"

func (d *reconcilerModule) reconcilePlatformConfig(ctx context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	var platformConfig v1alpha1.PlatformConfig
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &platformConfig); err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("failed to parse PlatformConfig: %w", err)}}
	}

	installer := gitops.NewGitOpsInstaller(platformConfig.Spec.GitOps.Engine, d.clientProvider)

	type componentResult struct {
		name   string
		result *ReconcileResult
	}
	components := []componentResult{
		{componentCertManager, d.reconcileCertManager(ctx, platformConfig.Spec, installer)},
	}

	statuses := make([]v1alpha1.PlatformComponentStatus, 0, len(components))
	results := make([]ReconcileResult, 0)
	now := metav1.Now()

	for _, c := range components {
		status := v1alpha1.PlatformComponentStatus{
			Name:     c.name,
			Ready:    c.result == nil,
			LastSync: now,
		}
		if c.result != nil {
			results = append(results, *c.result)
			if c.result.Err != nil {
				status.Message = c.result.Err.Error()
			}
		}
		statuses = append(statuses, status)
	}

	if err := d.updatePlatformConfigStatus(ctx, obj.GetName(), statuses); err != nil {
		d.logger.Warn("failed to update PlatformConfig status", "name", obj.GetName(), "error", err)
	}

	return results
}

func (d *reconcilerModule) updatePlatformConfigStatus(ctx context.Context, name string, components []v1alpha1.PlatformComponentStatus) error {
	patch := map[string]interface{}{
		"status": map[string]interface{}{
			"components": components,
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal status patch: %w", err)
	}

	_, err = d.clientProvider.DynamicClient().Resource(platformConfigGVR).Patch(
		ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status",
	)
	return err
}
