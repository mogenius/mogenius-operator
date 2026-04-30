package gitops

import (
	"context"
	"fmt"
	"mogenius-operator/src/k8sclient"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var argoApplicationGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "applications",
}

const argoNamespace = "argocd"

type argocdInstaller struct {
	clientProvider k8sclient.K8sClientProvider
}

func (a *argocdInstaller) Install(component string, artifact GitOpsArtifact) error {
	app := buildArgoApplication(component, artifact)
	if err := applyUnstructured(a.clientProvider, argoApplicationGVR, argoNamespace, app); err != nil {
		return fmt.Errorf("apply argocd application %s: %w", component, err)
	}

	if len(artifact.ExtraObjects) > 0 {
		moacApp := buildArgoMoacApplication(component, artifact)
		if err := applyUnstructured(a.clientProvider, argoApplicationGVR, argoNamespace, moacApp); err != nil {
			return fmt.Errorf("apply argocd moac application %s-resources: %w", component, err)
		}
	}

	return nil
}

func (a *argocdInstaller) UnInstall(component string) error {
	ctx := context.Background()
	client := a.clientProvider.DynamicClient().Resource(argoApplicationGVR).Namespace(argoNamespace)

	for _, name := range []string{component, component + "-resources"} {
		if err := client.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete argocd application %s: %w", name, err)
		}
	}
	return nil
}

func buildArgoApplication(component string, artifact GitOpsArtifact) *unstructured.Unstructured {
	helm := map[string]interface{}{}
	if artifact.Values != "" {
		helm["values"] = artifact.Values
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"name":      component,
				"namespace": argoNamespace,
				"labels":    defaultLabels(component),
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					"repoURL":        artifact.HelmChart.Repository,
					"chart":          artifact.HelmChart.Chart,
					"targetRevision": artifact.HelmChart.Version,
					"helm":           helm,
				},
				"destination": map[string]interface{}{
					"name":      "in-cluster",
					"namespace": artifact.Namespace,
				},
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{
						"prune":    true,
						"selfHeal": true,
					},
					"syncOptions": []interface{}{"CreateNamespace=true"},
				},
			},
		},
	}
}

func buildArgoMoacApplication(component string, artifact GitOpsArtifact) *unstructured.Unstructured {
	name := component + "-resources"
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": argoNamespace,
				"labels":    defaultLabels(component),
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					"repoURL":        moacRepository,
					"chart":          moacChart,
					"targetRevision": moacVersion,
					"helm": map[string]interface{}{
						"valuesObject": map[string]interface{}{
							"rawResources": artifact.ExtraObjects,
						},
					},
				},
				"destination": map[string]interface{}{
					"name":      "in-cluster",
					"namespace": artifact.Namespace,
				},
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{
						"prune":    true,
						"selfHeal": true,
					},
					"syncOptions": []interface{}{"CreateNamespace=true"},
				},
			},
		},
	}
}
