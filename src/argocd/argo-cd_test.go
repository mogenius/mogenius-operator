package argocd

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func platformConfigWithGitOpsStatus(engine string, installed bool, namespace, releaseName string) unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "mogenius.com/v1alpha1",
		"kind":       "PlatformConfig",
		"metadata":   map[string]any{"name": "platform"},
		"status": map[string]any{
			"gitOpsStatus": map[string]any{
				"engine":      engine,
				"installed":   installed,
				"namespace":   namespace,
				"releaseName": releaseName,
			},
		},
	}}
}

func TestArgoCdConfigFromPlatformConfigList(t *testing.T) {
	config, err := argoCdConfigFromPlatformConfigList([]unstructured.Unstructured{
		platformConfigWithGitOpsStatus("argo-cd", true, "argocd", "argocd"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Data["namespaceName"] != "argocd" || config.Data["releaseName"] != "argocd" {
		t.Errorf("unexpected config data: %+v", config.Data)
	}
}

func TestArgoCdConfigFromPlatformConfigListRejectsUnusable(t *testing.T) {
	cases := map[string]unstructured.Unstructured{
		"flux engine":       platformConfigWithGitOpsStatus("flux", true, "flux-system", "flux-operator"),
		"not installed":     platformConfigWithGitOpsStatus("argo-cd", false, "argocd", "argocd"),
		"missing namespace": platformConfigWithGitOpsStatus("argo-cd", true, "", "argocd"),
		"no status":         {Object: map[string]any{"metadata": map[string]any{"name": "platform"}}},
	}
	for name, item := range cases {
		if _, err := argoCdConfigFromPlatformConfigList([]unstructured.Unstructured{item}); err == nil {
			t.Errorf("%s: expected an error, got a config", name)
		}
	}
}
