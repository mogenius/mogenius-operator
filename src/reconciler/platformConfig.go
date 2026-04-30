package reconciler

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func (d *reconcilerModule) reconcilePlatformConfig(ctx context.Context, obj *unstructured.Unstructured, op operation) []ReconcileResult {
	var platformConfig v1alpha1.PlatformConfig
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &platformConfig); err != nil {
		return []ReconcileResult{{Err: fmt.Errorf("failed to parse PlatformConfig: %w", err)}}
	}

	results := []ReconcileResult{}

	installer := gitops.NewGitOpsInstaller(platformConfig.Spec.GitOps.Engine)

	results = append(results, d.reconcileCertManager(ctx, platformConfig.Spec, installer))

	return results
}
