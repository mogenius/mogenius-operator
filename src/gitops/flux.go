package gitops

import (
	"context"
	"fmt"
	"mogenius-operator/src/k8sclient"
	"strings"

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
	fluxOCIRepositoryGVR = schema.GroupVersionResource{
		Group:    "source.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "ocirepositories",
	}
	fluxHelmReleaseGVR = schema.GroupVersionResource{
		Group:    "helm.toolkit.fluxcd.io",
		Version:  "v2",
		Resource: "helmreleases",
	}
)

func isOCIRepository(url string) bool {
	return strings.HasPrefix(url, "oci://")
}

type fluxInstaller struct {
	clientProvider k8sclient.K8sClientProvider
	namespace      string
}

func (f *fluxInstaller) Install(component string, artifact GitOpsArtifact) error {
	if isOCIRepository(artifact.HelmChart.Repository) {
		ociRepo := buildFluxOCIRepository(component, artifact.HelmChart.Repository, artifact.HelmChart.Version, f.namespace)
		if err := applyUnstructured(f.clientProvider, fluxOCIRepositoryGVR, f.namespace, ociRepo); err != nil {
			return fmt.Errorf("apply flux ocirepository %s: %w", component, err)
		}

		release := buildFluxOCIHelmRelease(component, artifact, f.namespace)
		if err := applyUnstructured(f.clientProvider, fluxHelmReleaseGVR, f.namespace, release); err != nil {
			return fmt.Errorf("apply flux helmrelease %s: %w", component, err)
		}

		return nil
	}

	repo := buildFluxHelmRepository(component, artifact.HelmChart.Repository, f.namespace)
	if err := applyUnstructured(f.clientProvider, fluxHelmRepositoryGVR, f.namespace, repo); err != nil {
		return fmt.Errorf("apply flux helmrepository %s: %w", component, err)
	}

	release := buildFluxHelmRelease(component, artifact, artifact.Values, f.namespace)
	if err := applyUnstructured(f.clientProvider, fluxHelmReleaseGVR, f.namespace, release); err != nil {
		return fmt.Errorf("apply flux helmrelease %s: %w", component, err)
	}

	if len(artifact.ExtraObjects) > 0 {
		moacRepo := buildFluxHelmRepository(component+"-resources", moacRepository, f.namespace)
		if err := applyUnstructured(f.clientProvider, fluxHelmRepositoryGVR, f.namespace, moacRepo); err != nil {
			return fmt.Errorf("apply flux moac helmrepository %s-resources: %w", component, err)
		}

		moacRelease := buildFluxMoacHelmRelease(component, artifact, f.namespace)
		if err := applyUnstructured(f.clientProvider, fluxHelmReleaseGVR, f.namespace, moacRelease); err != nil {
			return fmt.Errorf("apply flux moac helmrelease %s-resources: %w", component, err)
		}
	}

	return nil
}

func (f *fluxInstaller) UnInstall(component string) error {
	ctx := context.Background()
	repoClient := f.clientProvider.DynamicClient().Resource(fluxHelmRepositoryGVR).Namespace(f.namespace)
	ociRepoClient := f.clientProvider.DynamicClient().Resource(fluxOCIRepositoryGVR).Namespace(f.namespace)
	releaseClient := f.clientProvider.DynamicClient().Resource(fluxHelmReleaseGVR).Namespace(f.namespace)

	for _, name := range []string{component, component + "-resources"} {
		if err := releaseClient.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete flux helmrelease %s: %w", name, err)
		}
		if err := repoClient.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete flux helmrepository %s: %w", name, err)
		}
		if err := ociRepoClient.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete flux ocirepository %s: %w", name, err)
		}
	}
	return nil
}

func buildFluxHelmRepository(name string, url string, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "source.toolkit.fluxcd.io/v1",
			"kind":       "HelmRepository",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
				"labels":    defaultLabels(name),
			},
			"spec": map[string]any{
				"url": url,
			},
		},
	}
}

func buildFluxHelmRelease(component string, artifact GitOpsArtifact, values map[string]any, namespace string) *unstructured.Unstructured {
	spec := map[string]any{
		"interval":        "10m",
		"releaseName":     artifact.HelmChart.Name,
		"targetNamespace": artifact.Namespace,
		"chart": map[string]any{
			"spec": map[string]any{
				"chart":   artifact.HelmChart.Chart,
				"version": artifact.HelmChart.Version,
				"sourceRef": map[string]any{
					"kind":      "HelmRepository",
					"name":      component,
					"namespace": namespace,
				},
			},
		},
		"install": map[string]any{
			"createNamespace": true,
			"strategy": map[string]any{
				"name": "RetryOnFailure",
			},
		},
		"upgrade": map[string]any{
			"strategy": map[string]any{
				"name": "RetryOnFailure",
			},
		},
	}
	if len(values) > 0 {
		spec["values"] = values
	}

	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata": map[string]any{
				"name":      component,
				"namespace": namespace,
				"labels":    defaultLabels(component),
			},
			"spec": spec,
		},
	}
}

func buildFluxOCIRepository(name, url, version, namespace string) *unstructured.Unstructured {
	semver := version
	if semver == "" {
		semver = "*"
	}
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "source.toolkit.fluxcd.io/v1",
			"kind":       "OCIRepository",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
				"labels":    defaultLabels(name),
			},
			"spec": map[string]any{
				"interval": "10m",
				"url":      url,
				"ref": map[string]any{
					"semver": semver,
				},
			},
		},
	}
}

func buildFluxOCIHelmRelease(component string, artifact GitOpsArtifact, namespace string) *unstructured.Unstructured {
	spec := map[string]any{
		"interval":           "10m",
		"releaseName":        artifact.HelmChart.Name,
		"serviceAccountName": component,
		"chartRef": map[string]any{
			"kind": "OCIRepository",
			"name": component,
		},
		"install": map[string]any{
			"strategy": map[string]any{
				"name":          "RetryOnFailure",
				"retryInterval": "3m",
			},
		},
		"upgrade": map[string]any{
			"force": true,
			"strategy": map[string]any{
				"name":          "RetryOnFailure",
				"retryInterval": "3m",
			},
		},
	}
	if len(artifact.Values) > 0 {
		spec["values"] = artifact.Values
	}
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata": map[string]any{
				"name":      component,
				"namespace": namespace,
				"labels":    defaultLabels(component),
			},
			"spec": spec,
		},
	}
}

func buildFluxMoacHelmRelease(component string, artifact GitOpsArtifact, namespace string) *unstructured.Unstructured {
	name := component + "-resources"
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
				"labels":    defaultLabels(component),
			},
			"spec": map[string]any{
				"interval":        "10m",
				"releaseName":     artifact.HelmChart.Name + "-resources",
				"targetNamespace": artifact.Namespace,
				"dependsOn": []any{
					map[string]any{
						"name":      component,
						"namespace": namespace,
					},
				},
				"chart": map[string]any{
					"spec": map[string]any{
						"chart":   moacChart,
						"version": moacVersion,
						"sourceRef": map[string]any{
							"kind":      "HelmRepository",
							"name":      name,
							"namespace": namespace,
						},
					},
				},
				"install": map[string]any{
					"createNamespace": true,
					"strategy": map[string]any{
						"name": "RetryOnFailure",
					},
				},
				"upgrade": map[string]any{
					"strategy": map[string]any{
						"name": "RetryOnFailure",
					},
				},
				"values": map[string]any{
					"rawResources": artifact.ExtraObjects,
				},
			},
		},
	}
}
