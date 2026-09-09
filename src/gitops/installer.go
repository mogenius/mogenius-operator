package gitops

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-operator/src/k8sclient"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	// moacRepository is the OCI Helm registry that hosts the moac chart.
	moacRepository = "https://helm.mogenius.com/public"
	// moacChart deploys raw extra Kubernetes objects via the rawResources values key.
	moacChart   = "moac"
	moacVersion = "1.2.3"
)

// GitOps engine identities. These match the platform SDK's GitOpsEngineEnum and
// are reported verbatim in PlatformConfig.status.gitOpsStatus.engine. They are
// distinct from the component names used to address platform-defaults files.
const (
	EngineArgoCD = "argo-cd"
	EngineFlux   = "flux"
)

type GitOpsArtifact struct {
	Namespace    string
	Values       map[string]any
	HelmChart    HelmChartReference
	ExtraObjects []any

	ArgoCD *ArgoCDSettings
	FluxCD *FluxCDSettings
}

type ArgoCDSettings struct {
	Project string
}

type FluxCDSettings struct {
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

// ManagedByLabel and ManagedByValue mark every object this operator creates
// through an installer. UnInstall requires them before it deletes anything.
const (
	ManagedByLabel = "app.kubernetes.io/managed-by"
	ManagedByValue = "mogenius-operator"
)

// noopInstaller is returned when no engine is configured so that component
// reconcilers can call Install/UnInstall without panicking.
type noopInstaller struct{}

func (n *noopInstaller) Install(_ string, _ GitOpsArtifact) error { return nil }
func (n *noopInstaller) UnInstall(_ string) error                 { return nil }

// NewGitOpsInstaller returns an installer for the given engine type.
// namespace is where the engine's own CRDs (Applications, HelmReleases, …) live.
// ownerRefs are set on every resource created by the installer.
// logger reports what UnInstall decided not to delete, which is otherwise
// invisible: a skipped delete looks exactly like a successful one.
func NewGitOpsInstaller(engine, namespace string, clientProvider k8sclient.K8sClientProvider, ownerRefs []metav1.OwnerReference, logger *slog.Logger) GitOpsInstaller {
	switch engine {
	case EngineArgoCD:
		return &argocdInstaller{clientProvider: clientProvider, namespace: namespace, ownerRefs: ownerRefs, logger: logger}
	case EngineFlux:
		return &fluxInstaller{clientProvider: clientProvider, namespace: namespace, ownerRefs: ownerRefs, logger: logger}
	default:
		return &noopInstaller{}
	}
}

func defaultLabels(component string) map[string]string {
	return map[string]string{
		ManagedByLabel:                ManagedByValue,
		"app.kubernetes.io/component": component,
	}
}

// deleteIfManaged deletes one object, but only when it carries this operator's
// managed-by label.
//
// Deleting by name alone made every disable a hazard. A component someone else
// installed under the same name became collateral damage, and on a cluster
// where mogenius never installed the component at all a single stray
// enabled:false took out the user's own release — which is how a Traefik that
// predated mogenius ended up deleted.
//
// A skipped delete is not an error: the spec asked for the component to be
// gone, and as far as this operator is concerned it is. Only louder, because
// the object staying behind is otherwise indistinguishable from a delete that
// worked.
func deleteIfManaged(ctx context.Context, logger *slog.Logger, client dynamic.ResourceInterface, kind string, name string) error {
	object, err := client.Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s %s: %w", kind, name, err)
	}

	if managedBy := object.GetLabels()[ManagedByLabel]; managedBy != ManagedByValue {
		if logger != nil {
			logger.Warn("not deleting a resource this operator did not install",
				"kind", kind, "name", name, "namespace", object.GetNamespace(), "managedBy", managedBy)
		}
		return nil
	}

	if err := client.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete %s %s: %w", kind, name, err)
	}
	return nil
}

// applyUnstructured creates or updates a namespaced resource via the dynamic client.
// Apply creates the object, or updates it in place when it already exists.
//
// Exported for the reconciler: objects that belong to an engine mogenius does
// not own cannot ship as extra objects of a release it installed, because
// there is no such release -- they have to be applied directly.
func Apply(cp k8sclient.K8sClientProvider, gvr schema.GroupVersionResource, namespace string, obj *unstructured.Unstructured) error {
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
