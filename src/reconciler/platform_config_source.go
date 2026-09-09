package reconciler

import (
	"context"
	"strings"

	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Who owns the PlatformConfig's spec.
const (
	configSourceCluster = "cluster"
	configSourceGit     = "git"
)

// Provenance markers the engines stamp on every object they apply. Reading them
// off the PlatformConfig itself is what makes the answer evidence rather than
// inference: the object either was applied by an engine or it was not.
const (
	fluxKustomizationNameLabel      = "kustomize.toolkit.fluxcd.io/name"
	fluxKustomizationNamespaceLabel = "kustomize.toolkit.fluxcd.io/namespace"
	argoTrackingIDAnnotation        = "argocd.argoproj.io/tracking-id"
	argoInstanceLabel               = "app.kubernetes.io/instance"
)

// detectConfigSource reads who applied the PlatformConfig off the object.
//
// engineNamespace is where an Argo CD Application would live: Argo's markers
// name the Application but not its namespace, while Flux's name both.
func detectConfigSource(labels map[string]string, annotations map[string]string, engineNamespace string) *v1alpha1.PlatformConfigSource {
	if name := labels[fluxKustomizationNameLabel]; name != "" {
		namespace := labels[fluxKustomizationNamespaceLabel]
		if namespace == "" {
			namespace = engineNamespace
		}
		return &v1alpha1.PlatformConfigSource{
			Source:   configSourceGit,
			SyncedBy: namespace + "/" + name,
		}
	}

	if name := argoApplicationName(labels, annotations); name != "" {
		return &v1alpha1.PlatformConfigSource{
			Source:   configSourceGit,
			SyncedBy: engineNamespace + "/" + name,
		}
	}

	return &v1alpha1.PlatformConfigSource{Source: configSourceCluster}
}

// argoApplicationName pulls the Application name out of Argo CD's tracking
// markers. The tracking id is "<app>:<group>/<kind>:<namespace>/<name>", so the
// application is the segment before the first colon; the instance label carries
// the same name on installations configured to track by label.
func argoApplicationName(labels map[string]string, annotations map[string]string) string {
	if trackingID := annotations[argoTrackingIDAnnotation]; trackingID != "" {
		if app, _, found := strings.Cut(trackingID, ":"); found && app != "" {
			return app
		}
		return trackingID
	}
	return labels[argoInstanceLabel]
}

// resolveSyncedRevision fills in the git revision the engine last applied.
//
// Best effort: the revision is reported by the sync object, and a cluster where
// that object is gone or unreadable still has a valid source of "git". Losing
// the revision only costs the UI a detail, while failing the reconcile over it
// would stop the platform.
func (d *reconcilerModule) resolveSyncedRevision(ctx context.Context, engine string, source *v1alpha1.PlatformConfigSource) {
	if source == nil || source.Source != configSourceGit || source.SyncedBy == "" {
		return
	}

	namespace, name, found := strings.Cut(source.SyncedBy, "/")
	if !found {
		return
	}

	var resource utils.ResourceDescriptor
	var revisionPath []string
	switch engine {
	case gitOpsEngineFlux:
		resource = utils.KustomizationResource
		revisionPath = []string{"status", "lastAppliedRevision"}
	case gitOpsEngineArgoCD:
		resource = utils.ArgoApplicationResource
		revisionPath = []string{"status", "sync", "revision"}
	default:
		return
	}

	if !d.crdChecker.IsAvailable(resource) {
		return
	}

	object, err := d.clientProvider.DynamicClient().
		Resource(kubernetes.CreateGroupVersionResource(resource.ApiVersion, resource.Plural)).
		Namespace(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		d.logger.Debug("could not read the object that synced the PlatformConfig",
			"kind", resource.Kind, "namespace", namespace, "name", name, "error", err)
		return
	}

	revision, found, err := unstructured.NestedString(object.Object, revisionPath...)
	if err != nil || !found {
		return
	}
	source.Revision = revision
}

// configSourceEqual compares two sources field by field so an unchanged status
// is not patched on every sweep.
func configSourceEqual(current, desired *v1alpha1.PlatformConfigSource) bool {
	if current == nil || desired == nil {
		return current == desired
	}
	return current.Source == desired.Source &&
		current.Revision == desired.Revision &&
		current.SyncedBy == desired.SyncedBy
}
