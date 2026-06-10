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
	moacRepository = "https://helm.mogenius.com/public"
	// moacChart deploys raw extra Kubernetes objects via the rawResources values key.
	moacChart   = "moac"
	moacVersion = "1.2.3"
)

type GitOpsArtifact struct {
	Namespace    string
	Values       map[string]any
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

// noopInstaller is returned when no engine is configured so that component
// reconcilers can call Install/UnInstall without panicking.
type noopInstaller struct{}

func (n *noopInstaller) Install(_ string, _ GitOpsArtifact) error { return nil }
func (n *noopInstaller) UnInstall(_ string) error                 { return nil }

// NewGitOpsInstaller returns an installer for the given engine type.
// namespace is where the engine's own CRDs (Applications, HelmReleases, …) live.
// ownerRefs are set on every resource created by the installer.
func NewGitOpsInstaller(engine, namespace string, clientProvider k8sclient.K8sClientProvider, ownerRefs []metav1.OwnerReference) GitOpsInstaller {
	switch engine {
	case "argocd":
		return &argocdInstaller{clientProvider: clientProvider, namespace: namespace, ownerRefs: ownerRefs}
	case "flux":
		return &fluxInstaller{clientProvider: clientProvider, namespace: namespace, ownerRefs: ownerRefs}
	default:
		return &noopInstaller{}
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
