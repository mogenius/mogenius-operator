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

var (
	fluxHelmRepositoryGVR = schema.GroupVersionResource{
		Group:    "source.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "helmrepositories",
	}
	fluxHelmReleaseGVR = schema.GroupVersionResource{
		Group:    "helm.toolkit.fluxcd.io",
		Version:  "v2",
		Resource: "helmreleases",
	}
)

// fluxNamespace is where all Flux CRs are created. The HelmRelease
// spec.targetNamespace controls the actual deployment namespace.
const fluxNamespace = "flux-system"

type fluxInstaller struct {
	clientProvider k8sclient.K8sClientProvider
}

func (f *fluxInstaller) Install(component string, artifact GitOpsArtifact) error {
	repo := buildFluxHelmRepository(component, artifact.HelmChart.Repository)
	if err := applyUnstructured(f.clientProvider, fluxHelmRepositoryGVR, fluxNamespace, repo); err != nil {
		return fmt.Errorf("apply flux helmrepository %s: %w", component, err)
	}

	release := buildFluxHelmRelease(component, artifact, artifact.Values)
	if err := applyUnstructured(f.clientProvider, fluxHelmReleaseGVR, fluxNamespace, release); err != nil {
		return fmt.Errorf("apply flux helmrelease %s: %w", component, err)
	}

	if len(artifact.ExtraObjects) > 0 {
		moacRepo := buildFluxHelmRepository(component+"-resources", moacRepository)
		if err := applyUnstructured(f.clientProvider, fluxHelmRepositoryGVR, fluxNamespace, moacRepo); err != nil {
			return fmt.Errorf("apply flux moac helmrepository %s-resources: %w", component, err)
		}

		moacRelease := buildFluxMoacHelmRelease(component, artifact)
		if err := applyUnstructured(f.clientProvider, fluxHelmReleaseGVR, fluxNamespace, moacRelease); err != nil {
			return fmt.Errorf("apply flux moac helmrelease %s-resources: %w", component, err)
		}
	}

	return nil
}

func (f *fluxInstaller) UnInstall(component string) error {
	ctx := context.Background()
	repoClient := f.clientProvider.DynamicClient().Resource(fluxHelmRepositoryGVR).Namespace(fluxNamespace)
	releaseClient := f.clientProvider.DynamicClient().Resource(fluxHelmReleaseGVR).Namespace(fluxNamespace)

	for _, name := range []string{component, component + "-resources"} {
		if err := releaseClient.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete flux helmrelease %s: %w", name, err)
		}
		if err := repoClient.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete flux helmrepository %s: %w", name, err)
		}
	}
	return nil
}

func buildFluxHelmRepository(name string, url string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "source.toolkit.fluxcd.io/v1",
			"kind":       "HelmRepository",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": fluxNamespace,
				"labels":    defaultLabels(name),
			},
			"spec": map[string]interface{}{
				"url": url,
			},
		},
	}
}

func buildFluxHelmRelease(component string, artifact GitOpsArtifact, values map[string]interface{}) *unstructured.Unstructured {
	spec := map[string]interface{}{
		"targetNamespace": artifact.Namespace,
		"chart": map[string]interface{}{
			"spec": map[string]interface{}{
				"chart":   artifact.HelmChart.Chart,
				"version": artifact.HelmChart.Version,
				"sourceRef": map[string]interface{}{
					"kind":      "HelmRepository",
					"name":      component,
					"namespace": fluxNamespace,
				},
			},
		},
		"install": map[string]interface{}{
			"createNamespace": true,
		},
	}
	if len(values) > 0 {
		spec["values"] = values
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata": map[string]interface{}{
				"name":      component,
				"namespace": fluxNamespace,
				"labels":    defaultLabels(component),
			},
			"spec": spec,
		},
	}
}

func buildFluxMoacHelmRelease(component string, artifact GitOpsArtifact) *unstructured.Unstructured {
	name := component + "-resources"
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": fluxNamespace,
				"labels":    defaultLabels(component),
			},
			"spec": map[string]interface{}{
				"targetNamespace": artifact.Namespace,
				"chart": map[string]interface{}{
					"spec": map[string]interface{}{
						"chart":   moacChart,
						"version": moacVersion,
						"sourceRef": map[string]interface{}{
							"kind":      "HelmRepository",
							"name":      name,
							"namespace": fluxNamespace,
						},
					},
				},
				"install": map[string]interface{}{
					"createNamespace": true,
				},
				"values": map[string]interface{}{
					"rawResources": artifact.ExtraObjects,
				},
			},
		},
	}
}

