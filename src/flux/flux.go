// Package flux implements the operator-side handlers for the
// cluster/flux-* socket commands. Unlike ArgoCD (src/argocd), Flux has no
// server API: every operation is an annotation or spec patch on a Flux
// custom resource, applied through the dynamic client. There is therefore
// no token, ConfigMap or server-URL discovery logic in this package.
package flux

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/logging"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// Flux reconcile annotations, see
	// https://fluxcd.io/flux/components/helm/helmreleases/#triggering-a-reconcile
	FLUX_REQUESTED_AT_ANNOTATION = "reconcile.fluxcd.io/requestedAt"
	FLUX_FORCE_AT_ANNOTATION     = "reconcile.fluxcd.io/forceAt"
	FLUX_RESET_AT_ANNOTATION     = "reconcile.fluxcd.io/resetAt"

	kindHelmRelease = "HelmRelease"
)

type Flux interface {
	FluxReconcile(data FluxResourceRequest) (bool, error)
	FluxReconcileWithSource(data FluxResourceRequest) (bool, error)
	FluxSuspend(data FluxResourceRequest) (bool, error)
	FluxResume(data FluxResourceRequest) (bool, error)
	FluxForce(data FluxResourceRequest) (bool, error)
	ListHelmReleases() ([]FluxHelmRelease, error)
}

type flux struct {
	logger         *slog.Logger
	clientProvider k8sclient.K8sClientProvider
}

func NewFlux(logManager logging.SlogManager, clientProviderModule k8sclient.K8sClientProvider) Flux {
	flux := flux{
		logger:         logManager.CreateLogger("flux"),
		clientProvider: clientProviderModule,
	}
	return &flux
}

// FluxResourceRequest identifies a single Flux custom resource. The json
// keys kind/namespace/name are deliberately top-level: the audit-log
// fallback extraction (store.AddToAuditLog) reads exactly these keys from
// the datagram payload so entries attribute to the resource's namespace
// bucket instead of the generic _cluster bucket.
type FluxResourceRequest struct {
	Kind      string `json:"kind" validate:"required"`
	Namespace string `json:"namespace" validate:"required"`
	Name      string `json:"name" validate:"required"`
}

// fluxKindGVRs is the allowlist of kinds the cluster/flux-* commands accept.
// API versions must match what the platform's DefaultResourceDescriptor and
// src/gitops/flux.go use (source/kustomize v1, helm v2).
var fluxKindGVRs = map[string]schema.GroupVersionResource{
	"Kustomization":  {Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"},
	kindHelmRelease:  {Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases"},
	"GitRepository":  {Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "gitrepositories"},
	"OCIRepository":  {Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "ocirepositories"},
	"HelmRepository": {Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "helmrepositories"},
	"Bucket":         {Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "buckets"},
}

// fluxSourceRefKindGVRs maps the kinds a sourceRef/chartRef may point at.
// Superset of the source kinds in fluxKindGVRs: a HelmRelease chartRef can
// also reference a HelmChart, which is not directly addressable via the
// socket commands but must be annotatable for reconcile-with-source.
var fluxSourceRefKindGVRs = map[string]schema.GroupVersionResource{
	"GitRepository":  {Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "gitrepositories"},
	"OCIRepository":  {Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "ocirepositories"},
	"HelmRepository": {Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "helmrepositories"},
	"Bucket":         {Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "buckets"},
	"HelmChart":      {Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "helmcharts"},
}

// resolveKindGVR validates the request kind against the allowlist
// (case-insensitively) and returns its GroupVersionResource.
func resolveKindGVR(kind string) (schema.GroupVersionResource, error) {
	if gvr, ok := fluxKindGVRs[kind]; ok {
		return gvr, nil
	}
	for canonical, gvr := range fluxKindGVRs {
		if strings.EqualFold(canonical, kind) {
			return gvr, nil
		}
	}
	return schema.GroupVersionResource{}, fmt.Errorf("unsupported flux resource kind: %q", kind)
}

// fluxSourceRef is a resolved spec.sourceRef / spec.chartRef target.
type fluxSourceRef struct {
	Kind      string
	Name      string
	Namespace string
}

// resolveSourceRef extracts the source reference of a Flux object:
// spec.chartRef or spec.chart.spec.sourceRef for a HelmRelease (chartRef
// wins, mirroring the platform API fallback), spec.sourceRef for everything
// else. Returns nil when the object carries no (complete) source reference
// - source kinds like GitRepository have none.
func resolveSourceRef(obj *unstructured.Unstructured) *fluxSourceRef {
	if obj == nil {
		return nil
	}

	paths := [][]string{{"spec", "sourceRef"}}
	if obj.GetKind() == kindHelmRelease {
		paths = [][]string{
			{"spec", "chartRef"},
			{"spec", "chart", "spec", "sourceRef"},
		}
	}

	for _, path := range paths {
		ref, found, err := unstructured.NestedMap(obj.Object, path...)
		if err != nil || !found {
			continue
		}
		kind, _ := ref["kind"].(string)
		name, _ := ref["name"].(string)
		namespace, _ := ref["namespace"].(string)
		if kind == "" || name == "" {
			continue
		}
		if namespace == "" {
			namespace = obj.GetNamespace()
		}
		return &fluxSourceRef{Kind: kind, Name: name, Namespace: namespace}
	}
	return nil
}

// reconcileToken returns the timestamp value Flux expects in its reconcile
// annotations. All annotations of a single operation must carry the SAME
// token, so callers generate it once and reuse it.
func reconcileToken() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// mergePatch applies a JSON merge patch (RFC 7386: nested maps merge, only
// the given keys are replaced) to the resource identified by gvr/namespace/name.
func (self *flux) mergePatch(gvr schema.GroupVersionResource, namespace string, name string, patch map[string]any) error {
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch for %s %s/%s: %w", gvr.Resource, namespace, name, err)
	}
	_, err = self.clientProvider.DynamicClient().
		Resource(gvr).
		Namespace(namespace).
		Patch(context.Background(), name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch %s %s/%s: %w", gvr.Resource, namespace, name, err)
	}
	return nil
}

func annotationPatch(annotations map[string]string) map[string]any {
	return map[string]any{
		"metadata": map[string]any{
			"annotations": annotations,
		},
	}
}

// FluxReconcile triggers a reconciliation of the resource by setting the
// reconcile.fluxcd.io/requestedAt annotation.
func (self *flux) FluxReconcile(data FluxResourceRequest) (bool, error) {
	gvr, err := resolveKindGVR(data.Kind)
	if err != nil {
		return false, err
	}
	err = self.mergePatch(gvr, data.Namespace, data.Name, annotationPatch(map[string]string{
		FLUX_REQUESTED_AT_ANNOTATION: reconcileToken(),
	}))
	if err != nil {
		return false, err
	}
	return true, nil
}

// FluxReconcileWithSource annotates the resource's source (resolved from
// spec.sourceRef, or spec.chartRef / spec.chart.spec.sourceRef for a
// HelmRelease) first and then the resource itself. Resources without a
// source reference (e.g. GitRepository) are reconciled directly.
func (self *flux) FluxReconcileWithSource(data FluxResourceRequest) (bool, error) {
	gvr, err := resolveKindGVR(data.Kind)
	if err != nil {
		return false, err
	}

	obj, err := self.clientProvider.DynamicClient().
		Resource(gvr).
		Namespace(data.Namespace).
		Get(context.Background(), data.Name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get %s %s/%s: %w", gvr.Resource, data.Namespace, data.Name, err)
	}

	token := reconcileToken()
	if sourceRef := resolveSourceRef(obj); sourceRef != nil {
		sourceGVR, ok := fluxSourceRefKindGVRs[sourceRef.Kind]
		if !ok {
			self.logger.Warn("skipping source annotation for unsupported sourceRef kind",
				slog.String("sourceRefKind", sourceRef.Kind),
				slog.String("kind", data.Kind),
				slog.String("namespace", data.Namespace),
				slog.String("name", data.Name),
			)
		} else {
			err = self.mergePatch(sourceGVR, sourceRef.Namespace, sourceRef.Name, annotationPatch(map[string]string{
				FLUX_REQUESTED_AT_ANNOTATION: token,
			}))
			if err != nil {
				return false, err
			}
		}
	}

	err = self.mergePatch(gvr, data.Namespace, data.Name, annotationPatch(map[string]string{
		FLUX_REQUESTED_AT_ANNOTATION: token,
	}))
	if err != nil {
		return false, err
	}
	return true, nil
}

// FluxSuspend sets spec.suspend=true, pausing reconciliation of the resource.
func (self *flux) FluxSuspend(data FluxResourceRequest) (bool, error) {
	gvr, err := resolveKindGVR(data.Kind)
	if err != nil {
		return false, err
	}
	err = self.mergePatch(gvr, data.Namespace, data.Name, map[string]any{
		"spec": map[string]any{
			"suspend": true,
		},
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// FluxResume sets spec.suspend=false and requests an immediate reconcile in
// the same patch so the resource does not wait a full interval after resuming.
func (self *flux) FluxResume(data FluxResourceRequest) (bool, error) {
	gvr, err := resolveKindGVR(data.Kind)
	if err != nil {
		return false, err
	}
	err = self.mergePatch(gvr, data.Namespace, data.Name, map[string]any{
		"spec": map[string]any{
			"suspend": false,
		},
		"metadata": map[string]any{
			"annotations": map[string]string{
				FLUX_REQUESTED_AT_ANNOTATION: reconcileToken(),
			},
		},
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// FluxForce triggers a one-off forced reconcile of a HelmRelease (upgrade
// even without changes, reset of the failure counters). forceAt, resetAt and
// requestedAt must carry the same token for helm-controller to honor them.
// Only HelmRelease supports these annotations.
func (self *flux) FluxForce(data FluxResourceRequest) (bool, error) {
	gvr, err := resolveKindGVR(data.Kind)
	if err != nil {
		return false, err
	}
	if !strings.EqualFold(data.Kind, kindHelmRelease) {
		return false, fmt.Errorf("flux force is only supported for HelmRelease, got %q", data.Kind)
	}
	token := reconcileToken()
	err = self.mergePatch(gvr, data.Namespace, data.Name, annotationPatch(map[string]string{
		FLUX_REQUESTED_AT_ANNOTATION: token,
		FLUX_FORCE_AT_ANNOTATION:     token,
		FLUX_RESET_AT_ANNOTATION:     token,
	}))
	if err != nil {
		return false, err
	}
	return true, nil
}
