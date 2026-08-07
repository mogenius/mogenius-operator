package flux

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// FluxHelmRelease is a flattened view of a Flux HelmRelease CR carrying the
// resolved identity of the real helm release that helm-controller creates for
// it. Unlike Argo CD (src/argocd/applications.go), Flux-managed charts already
// appear in the helm release list as genuine releases — the list pipeline uses
// this data to TAG the matching entries, never to add duplicates.
type FluxHelmRelease struct {
	// Name / Namespace are the HelmRelease CR's own metadata.
	Name      string
	Namespace string
	// ReleaseName is spec.releaseName, defaulting to the CR name — the
	// platform contract's "spec.releaseName || metadata.name" matching rule.
	ReleaseName string
	// TargetNamespace is spec.targetNamespace, defaulting to the CR namespace
	// ("spec.targetNamespace || metadata.namespace").
	TargetNamespace string
}

// ListHelmReleases returns all Flux HelmRelease CRs cluster-wide. When the
// HelmRelease CRD is not installed it returns an empty list and no error, so
// clusters without Flux do not log a warning on every helm list request.
func (self *flux) ListHelmReleases() ([]FluxHelmRelease, error) {
	list, err := self.clientProvider.DynamicClient().
		Resource(fluxKindGVRs[kindHelmRelease]).
		Namespace(metav1.NamespaceAll).
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return nil, nil
		}
		return nil, err
	}

	results := make([]FluxHelmRelease, 0, len(list.Items))
	for i := range list.Items {
		hr := list.Items[i]

		releaseName, _, _ := unstructured.NestedString(hr.Object, "spec", "releaseName")
		if releaseName == "" {
			releaseName = hr.GetName()
		}
		targetNamespace, _, _ := unstructured.NestedString(hr.Object, "spec", "targetNamespace")
		if targetNamespace == "" {
			targetNamespace = hr.GetNamespace()
		}

		results = append(results, FluxHelmRelease{
			Name:            hr.GetName(),
			Namespace:       hr.GetNamespace(),
			ReleaseName:     releaseName,
			TargetNamespace: targetNamespace,
		})
	}
	return results, nil
}
