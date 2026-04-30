package gitops

import (
	"context"
	"mogenius-operator/src/k8sclient"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// moacRepository is the OCI Helm registry that hosts the moac chart.
	moacRepository = "oci://ghcr.io/mogenius/helm-charts"
	// moacChart deploys raw extra Kubernetes objects via the rawResources values key.
	moacChart   = "moac"
	moacVersion = "0.1.0"
)

type GitOpsArtifact struct {
	Namespace    string
	Values       string
	HelmChart    HelmChartReference
	ExtraObjects []any
}

type HelmChartReference struct {
	Repository string
	Chart      string
	Name       string
	Version    string
}

type GitOpsInstaller interface {
	Install(string, GitOpsArtifact) error
	UnInstall(string) error
}

func NewGitOpsInstaller(engine string, clientProvider k8sclient.K8sClientProvider) GitOpsInstaller {
	switch engine {
	case "argocd":
		return &argocdInstaller{clientProvider: clientProvider}
	case "flux":
		return &fluxInstaller{clientProvider: clientProvider}
	default:
		return nil
	}
}

func defaultLabels(component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "mogenius-operator",
		"app.kubernetes.io/component":  component,
	}
}

// applyUnstructured creates or updates a namespaced resource via the dynamic client.
func applyUnstructured(cp k8sclient.K8sClientProvider, gvr schema.GroupVersionResource, namespace string, obj *unstructured.Unstructured) error {
	ctx := context.Background()
	client := cp.DynamicClient().Resource(gvr).Namespace(namespace)

	_, err := client.Create(ctx, obj, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return err
	}

	existing, err := client.Get(ctx, obj.GetName(), metav1.GetOptions{})
	if err != nil {
		return err
	}
	obj.SetResourceVersion(existing.GetResourceVersion())
	_, err = client.Update(ctx, obj, metav1.UpdateOptions{})
	return err
}
