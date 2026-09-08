package reconciler

import (
	"context"
	"strings"

	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// fluxReportGVR is the Flux Operator's cluster report. It is optional: plain
// Flux installations (flux bootstrap, upstream helm chart) do not ship it.
var fluxReportGVR = schema.GroupVersionResource{
	Group:    "fluxcd.controlplane.io",
	Version:  "v1",
	Resource: "fluxreports",
}

const (
	fluxReportName         = "flux"
	helmReleaseNameAnno    = "meta.helm.sh/release-name"
	helmReleaseNsAnno      = "meta.helm.sh/release-namespace"
	partOfLabel            = "app.kubernetes.io/part-of"
	versionLabel           = "app.kubernetes.io/version"
	argoCDPartOfValue      = "argocd"
	fluxPartOfValue        = "flux"
	argoCDServerDeployment = "argocd-server"
)

// engineDetection is what probing the cluster found out about one GitOps engine.
type engineDetection struct {
	engine      string
	installed   bool
	namespace   string
	releaseName string
	version     string
	controllers []v1alpha1.GitOpsControllerStatus
}

// gitOpsDetection holds the probe result for every engine mogenius knows about.
type gitOpsDetection struct {
	argoCD *engineDetection
	flux   *engineDetection
}

func (g gitOpsDetection) forEngine(engine string) *engineDetection {
	switch engine {
	case gitOpsEngineArgoCD:
		return g.argoCD
	case gitOpsEngineFlux:
		return g.flux
	}
	return nil
}

// preferred picks the engine to report when the spec names none. Only an engine
// that actually runs qualifies: CRDs outlive the install they came with, so a
// leftover argoproj.io group must not outrank the engine doing the work. ArgoCD
// wins a tie because it is the engine mogenius installs by default. Nil means
// nothing runs, which leaves whatever the spec declares in place.
func (g gitOpsDetection) preferred() *engineDetection {
	if g.argoCD != nil && g.argoCD.installed {
		return g.argoCD
	}
	if g.flux != nil && g.flux.installed {
		return g.flux
	}
	return nil
}

// detectGitOpsStatus probes the live cluster for installed GitOps engines.
// Every probe is individually error tolerant — a missing CRD or a denied RBAC
// rule degrades to "not detected" and never fails the reconcile.
func (d *reconcilerModule) detectGitOpsStatus(ctx context.Context) gitOpsDetection {
	return gitOpsDetection{
		argoCD: d.detectEngine(ctx, gitOpsEngineArgoCD, utils.AppProjectResource, argoCDPartOfValue),
		flux:   d.detectEngine(ctx, gitOpsEngineFlux, utils.KustomizationResource, fluxPartOfValue),
	}
}

// detectEngine probes a single engine. crd is the resource the engine registers
// -- without it the engine was never installed here, with it the probe goes on
// to look for what actually runs; partOf is the app.kubernetes.io/part-of label
// value its controller deployments carry.
func (d *reconcilerModule) detectEngine(ctx context.Context, engine string, crd utils.ResourceDescriptor, partOf string) *engineDetection {
	if !d.crdChecker.IsAvailable(crd) {
		return nil
	}

	detection := &engineDetection{engine: engine}

	deployments, listErr := d.listEngineDeployments(ctx, partOf)
	for _, deployment := range deployments {
		annotations := deployment.GetAnnotations()
		if detection.namespace == "" {
			detection.namespace = firstNonEmpty(annotations[helmReleaseNsAnno], deployment.GetNamespace())
		}
		if detection.releaseName == "" {
			detection.releaseName = annotations[helmReleaseNameAnno]
		}
		detection.controllers = append(detection.controllers, controllerStatus(deployment))
	}

	switch engine {
	case gitOpsEngineFlux:
		reportVersion, reportFound := d.fluxReport(ctx, detection.namespace)
		detection.version = firstNonEmpty(reportVersion, mostCommonControllerVersion(detection.controllers))
		// The Flux Operator reports the distribution it manages, which proves the
		// install on its own -- its controllers are created by the operator and
		// need not be there yet.
		detection.installed = reportFound
	case gitOpsEngineArgoCD:
		detection.version = firstNonEmpty(
			controllerVersionByName(detection.controllers, argoCDServerDeployment),
			mostCommonControllerVersion(detection.controllers),
		)
	}

	// The CRD alone is no evidence of an install: neither `helm uninstall` nor
	// `kubectl delete -f install.yaml` removes CRDs, so a removed engine would
	// stay "installed" forever and outrank the engine that replaced it. Running
	// controllers are the evidence -- except when they could not be listed at
	// all, where the CRD is the only thing left to go by.
	detection.installed = detection.installed || len(detection.controllers) > 0 || listErr != nil

	return detection
}

// listEngineDeployments returns the engine's controller deployments across all
// namespaces. The error is handed up rather than swallowed: an empty list means
// the engine has no controllers, and only the error tells that apart from not
// having been allowed to look.
func (d *reconcilerModule) listEngineDeployments(ctx context.Context, partOf string) ([]unstructured.Unstructured, error) {
	gvr := kubernetes.CreateGroupVersionResource(utils.DeploymentResource.ApiVersion, utils.DeploymentResource.Plural)
	list, err := d.clientProvider.DynamicClient().Resource(gvr).List(ctx, metav1.ListOptions{
		LabelSelector: partOfLabel + "=" + partOf,
	})
	if err != nil {
		d.logger.Debug("GitOps detection: listing engine deployments failed", "partOf", partOf, "error", err)
		return nil, err
	}
	return list.Items, nil
}

// fluxReport reads the Flux Operator's FluxReport and returns the distribution
// version it carries plus whether the report exists at all. The CRD is
// frequently absent, so every failure is silent and reports "not found".
func (d *reconcilerModule) fluxReport(ctx context.Context, namespace string) (version string, found bool) {
	if namespace == "" {
		namespace = fluxcdDefaultNamespace
	}
	report, err := d.clientProvider.DynamicClient().Resource(fluxReportGVR).Namespace(namespace).Get(ctx, fluxReportName, metav1.GetOptions{})
	if err != nil {
		d.logger.Debug("GitOps detection: FluxReport unavailable", "namespace", namespace, "error", err)
		return "", false
	}
	for _, path := range [][]string{
		{"spec", "distribution", "version"},
		{"status", "distribution", "version"},
	} {
		if version, ok, err := unstructured.NestedString(report.Object, path...); err == nil && ok && version != "" {
			return version, true
		}
	}
	return "", true
}

func controllerStatus(deployment unstructured.Unstructured) v1alpha1.GitOpsControllerStatus {
	version := deployment.GetLabels()[versionLabel]
	if version == "" {
		version = imageTag(firstContainerImage(deployment))
	}

	replicas, _, _ := unstructured.NestedInt64(deployment.Object, "spec", "replicas")
	readyReplicas, _, _ := unstructured.NestedInt64(deployment.Object, "status", "readyReplicas")

	return v1alpha1.GitOpsControllerStatus{
		Name:    deployment.GetName(),
		Version: version,
		Ready:   replicas > 0 && readyReplicas >= replicas,
	}
}

func firstContainerImage(deployment unstructured.Unstructured) string {
	containers, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	if err != nil || !found || len(containers) == 0 {
		return ""
	}
	container, ok := containers[0].(map[string]any)
	if !ok {
		return ""
	}
	image, _ := container["image"].(string)
	return image
}

// imageTag extracts the tag from a container image reference, tolerating
// registry ports (host:5000/img:tag) and digests (img:tag@sha256:…).
func imageTag(image string) string {
	if image == "" {
		return ""
	}
	if at := strings.Index(image, "@"); at >= 0 {
		image = image[:at]
	}
	name := image
	if slash := strings.LastIndex(image, "/"); slash >= 0 {
		name = image[slash+1:]
	}
	_, tag, found := strings.Cut(name, ":")
	if !found {
		return ""
	}
	return tag
}

func controllerVersionByName(controllers []v1alpha1.GitOpsControllerStatus, name string) string {
	for _, controller := range controllers {
		if controller.Name == name {
			return controller.Version
		}
	}
	// Helm releases prefix the deployment name with the release name.
	for _, controller := range controllers {
		if strings.HasSuffix(controller.Name, "-"+name) {
			return controller.Version
		}
	}
	return ""
}

// mostCommonControllerVersion returns the version shared by the most
// controllers, which is the engine version for multi-controller engines
// like Flux. Ties are broken alphabetically so the result is stable.
func mostCommonControllerVersion(controllers []v1alpha1.GitOpsControllerStatus) string {
	counts := map[string]int{}
	for _, controller := range controllers {
		if controller.Version != "" {
			counts[controller.Version]++
		}
	}

	best := ""
	for version, count := range counts {
		if count > counts[best] || (count == counts[best] && version < best) {
			best = version
		}
	}
	return best
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
