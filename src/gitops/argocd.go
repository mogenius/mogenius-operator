package gitops

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-operator/src/k8sclient"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var argoApplicationGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "applications",
}

type argocdInstaller struct {
	clientProvider k8sclient.K8sClientProvider
	namespace      string
	ownerRefs      []metav1.OwnerReference
	logger         *slog.Logger
}

func (a *argocdInstaller) Install(component string, artifact GitOpsArtifact) error {
	app := buildArgoApplication(component, artifact, a.namespace)
	app.SetOwnerReferences(a.ownerRefs)
	if err := Apply(a.clientProvider, argoApplicationGVR, a.namespace, app); err != nil {
		return fmt.Errorf("apply argocd application %s: %w", component, err)
	}

	if len(artifact.ExtraObjects) > 0 {
		moacApp := buildArgoMoacApplication(component, artifact, a.namespace)
		moacApp.SetOwnerReferences(a.ownerRefs)
		if err := Apply(a.clientProvider, argoApplicationGVR, a.namespace, moacApp); err != nil {
			return fmt.Errorf("apply argocd moac application %s-resources: %w", component, err)
		}
	}

	return nil
}

func (a *argocdInstaller) UnInstall(component string) error {
	ctx := context.Background()
	client := a.clientProvider.DynamicClient().Resource(argoApplicationGVR).Namespace(a.namespace)

	for _, name := range []string{component, component + "-resources"} {
		if err := deleteIfManaged(ctx, a.logger, client, "argocd application", name); err != nil {
			return err
		}
	}
	return nil
}

func buildArgoApplication(component string, artifact GitOpsArtifact, namespace string) *unstructured.Unstructured {
	helm := map[string]any{
		"releaseName": artifact.HelmChart.Name,
	}
	if len(artifact.Values) > 0 {
		helm["valuesObject"] = artifact.Values
	}

	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]any{
				"name":       component,
				"namespace":  namespace,
				"labels":     defaultLabels(component),
				"finalizers": []any{"resources-finalizer.argocd.argoproj.io"}, // ensure resources are deleted when app is deleted
			},
			"spec": map[string]any{
				"revisionHistoryLimit": 1,
				"project":              getArgoProject(artifact),
				"source": map[string]any{
					"repoURL":        artifact.HelmChart.Repository,
					"chart":          artifact.HelmChart.Chart,
					"targetRevision": artifact.HelmChart.Version,
					"helm":           helm,
				},
				"destination": map[string]any{
					"name":      "in-cluster",
					"namespace": artifact.Namespace,
				},
				"syncPolicy": map[string]any{
					"automated": map[string]any{
						"prune":    true,
						"selfHeal": true,
					},
					"syncOptions": []any{"CreateNamespace=true", "ServerSideApply=true"},
				},
			},
		},
	}
}

func buildArgoMoacApplication(component string, artifact GitOpsArtifact, namespace string) *unstructured.Unstructured {
	name := component + "-resources"
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]any{
				"name":       name,
				"namespace":  namespace,
				"labels":     defaultLabels(component),
				"finalizers": []any{"resources-finalizer.argocd.argoproj.io"}, // ensure resources are deleted when app is deleted
			},
			"spec": map[string]any{
				"revisionHistoryLimit": 1,
				"project":              getArgoProject(artifact),
				"source": map[string]any{
					"repoURL":        moacRepository,
					"chart":          moacChart,
					"targetRevision": moacVersion,
					"helm": map[string]any{
						"releaseName": artifact.HelmChart.Name + "-resources",
						"valuesObject": map[string]any{
							"rawResources": artifact.ExtraObjects,
						},
					},
				},
				"destination": map[string]any{
					"name":      "in-cluster",
					"namespace": artifact.Namespace,
				},
				"syncPolicy": map[string]any{
					"automated": map[string]any{
						"prune":    true,
						"selfHeal": true,
					},
					"syncOptions": []any{"CreateNamespace=true", "ServerSideApply=true"},
				},
			},
		},
	}
}

func getArgoProject(artifact GitOpsArtifact) string {
	if artifact.ArgoCD == nil || artifact.ArgoCD.Project == "" {
		return "default"
	}
	return artifact.ArgoCD.Project
}
