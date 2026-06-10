package argocd

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

const (
	// argoHelmRepoNameLabel is set by the platform when it creates an Argo
	// Application from a mogenius helm-repo chart. We list only Applications
	// carrying it (the operator's own platform-component Applications use
	// different labels and must not appear in the user's release list).
	argoHelmRepoNameLabel = "mogenius-helm-repo-name"
	// mogeniusHelmRepoURL / mogeniusMoacChart identify the internal "moac"
	// helper Application that ships raw resources alongside a chart. It is not a
	// user-facing release and is excluded from the list (matches the platform's
	// historical filter).
	mogeniusHelmRepoURL = "https://helm.mogenius.com/public"
	mogeniusMoacChart   = "moac"
)

var argoApplicationGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "applications",
}

// ArgoHelmApplication is a flattened view of an Argo CD Application that wraps a
// helm chart, carrying exactly what the helm release list needs to render and
// act on it (MOG-4394).
type ArgoHelmApplication struct {
	// Name / Namespace are the Application's own metadata (used to delete it).
	Name      string
	Namespace string
	// ReleaseName is spec.source.helm.releaseName.
	ReleaseName string
	// ChartName / RepoURL / TargetRevision come from spec.source.
	ChartName      string
	RepoURL        string
	TargetRevision string
	// RepoName is the mogenius-helm-repo-name label value.
	RepoName string
	// DestNamespace is spec.destination.namespace.
	DestNamespace string
	// ValuesObject is spec.source.helm.valuesObject rendered as YAML.
	ValuesObject string
	// CreatedAt is metadata.creationTimestamp.
	CreatedAt time.Time
	// Application is the full Application object, passed through to the frontend.
	Application map[string]any
}

// ListHelmReleaseApplications returns the Argo-CD-managed helm charts in the
// Argo install namespace, filtered the same way the platform API used to:
// only Applications carrying the mogenius-helm-repo-name label, excluding the
// internal "moac" resources Application. When Argo is not installed (the
// argo-cd-config ConfigMap is absent) it returns an empty list and no error.
func (self *argocd) ListHelmReleaseApplications() ([]ArgoHelmApplication, error) {
	namespace, ok := self.argoCdNamespace()
	if !ok {
		// No Argo CD configured/installed: nothing to merge, not an error.
		return nil, nil
	}

	list, err := self.clientProvider.DynamicClient().
		Resource(argoApplicationGVR).
		Namespace(namespace).
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	results := make([]ArgoHelmApplication, 0, len(list.Items))
	for i := range list.Items {
		app := list.Items[i]

		// Only helm-sourced Applications created from a mogenius helm repo.
		repoName, _, _ := unstructured.NestedString(app.Object, "metadata", "labels", argoHelmRepoNameLabel)
		if repoName == "" {
			continue
		}

		repoURL, _, _ := unstructured.NestedString(app.Object, "spec", "source", "repoURL")
		chart, _, _ := unstructured.NestedString(app.Object, "spec", "source", "chart")
		// Skip the internal moac resources Application.
		if repoURL == mogeniusHelmRepoURL && chart == mogeniusMoacChart {
			continue
		}

		releaseName, _, _ := unstructured.NestedString(app.Object, "spec", "source", "helm", "releaseName")
		if releaseName == "" {
			// Not a helm release we can address for upgrade/uninstall.
			continue
		}

		targetRevision, _, _ := unstructured.NestedString(app.Object, "spec", "source", "targetRevision")
		destNamespace, _, _ := unstructured.NestedString(app.Object, "spec", "destination", "namespace")

		valuesObject := ""
		if values, found, _ := unstructured.NestedMap(app.Object, "spec", "source", "helm", "valuesObject"); found && len(values) > 0 {
			if y, err := yaml.Marshal(values); err == nil {
				valuesObject = string(y)
			}
		}

		results = append(results, ArgoHelmApplication{
			Name:           app.GetName(),
			Namespace:      app.GetNamespace(),
			ReleaseName:    releaseName,
			ChartName:      chart,
			RepoURL:        repoURL,
			TargetRevision: targetRevision,
			RepoName:       repoName,
			DestNamespace:  destNamespace,
			ValuesObject:   valuesObject,
			CreatedAt:      app.GetCreationTimestamp().Time,
			Application:    app.Object,
		})
	}

	return results, nil
}

// argoCdNamespace resolves the namespace Argo CD is installed in from the
// argo-cd-config ConfigMap. The second return is false when Argo CD is not
// configured (ConfigMap missing or namespaceName unset).
func (self *argocd) argoCdNamespace() (string, bool) {
	if err := self.initArgoCdConfig(); err != nil {
		return "", false
	}
	if self.argoCdConfig == nil || self.argoCdConfig.Data == nil {
		return "", false
	}
	ns, ok := self.argoCdConfig.Data["namespaceName"]
	if !ok || ns == "" {
		return "", false
	}
	return ns, true
}
