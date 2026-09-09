package reconciler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"mogenius-operator/src/helm"
	"mogenius-operator/src/utils"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const (
	// engineReadyTimeout bounds the wait for the engine's controllers after the
	// chart went in. Long enough for image pulls on a cold cluster, short
	// enough that a reconcile slot is not held for minutes when something is
	// actually broken.
	engineReadyTimeout = 3 * time.Minute
	// engineCRDTimeout bounds the wait for the CRDs the engine's chart
	// registers. They land with the release manifest, so this only absorbs API
	// server registration lag.
	engineCRDTimeout = 30 * time.Second
	// enginePollInterval is how often those two waits re-probe.
	enginePollInterval = 3 * time.Second
)

// engineBootstrapObject is applied straight through the dynamic client, before
// the engine is able to deliver anything itself.
//
// Flux's FluxInstance is the reason this exists: the flux-operator chart ships
// no controllers, they come from that resource, so it cannot travel the normal
// way — the normal way needs a running helm-controller. Argo CD's AppProject
// has the same shape of problem: every Application the platform creates is
// placed in that project, including the one that would have carried it.
type engineBootstrapObject struct {
	resource utils.ResourceDescriptor
	object   map[string]any
}

// engineSpec describes a GitOps engine the operator installs and owns.
type engineSpec struct {
	// component carries chart, patches and namespace, exactly as for any other
	// platform component.
	component componentSpec
	// engine is the detection name, gitOpsEngineFlux or gitOpsEngineArgoCD.
	engine string
	// bootstrapObjects are applied directly once the chart's CRDs exist.
	bootstrapObjects func(namespace string) []engineBootstrapObject
	// extraObjects and extraValues match reconcileComponent's callbacks.
	extraObjects func(ctx context.Context) ([]any, error)
	extraValues  func(ctx context.Context) (map[string]any, error)
}

// reconcileGitOpsEngine installs the GitOps engine with the Helm SDK and keeps
// owning it there.
//
// The engine cannot be delivered the way every other component is. Components
// ship as HelmReleases and Applications that the engine reconciles, so
// delivering the engine that way is circular: a HelmRelease for flux-operator
// needs a helm-controller that only exists once flux-operator ran. And even
// where it would resolve, it would leave the engine's controllers managing the
// release they themselves come from, free to delete their own workload halfway
// through an upgrade.
//
// So the chart goes in through the SDK, the objects that must exist before the
// controllers do are applied directly, and only the rest travels the normal
// way once the engine answers.
func (d *reconcilerModule) reconcileGitOpsEngine(
	ctx context.Context,
	platformSpec v1alpha1.PlatformConfigSpec,
	installer gitops.GitOpsInstaller,
	detection gitOpsDetection,
	es engineSpec,
) *ReconcileResult {
	cs := es.component
	namespace := helmNamespace(cs.chart, cs.defaultNamespace)

	// Only reachable for an engine the spec enables — reconcilePlatformConfig
	// dispatches on exactly that. Disabling one is not a request to remove it:
	// pulling the engine out would strand every component it delivers, so it is
	// left running and simply stops being managed.
	if !cs.enabled {
		return nil
	}

	artifact, result := d.buildComponentArtifact(ctx, platformSpec, cs, es.extraObjects, es.extraValues)
	if result != nil {
		return result
	}

	ownership, err := helm.LookupReleaseOwnership(namespace, artifact.HelmChart.Name)
	if err != nil {
		return &ReconcileResult{Err: fmt.Errorf("check ownership of %s: %w", cs.name, err)}
	}

	// Someone else's engine. Adopting it would overwrite their values on this
	// sweep and every one after, so it is left alone — mogenius still delivers
	// components through it, which is what a user-managed engine is for. The
	// fix is to stop asking mogenius to install one.
	if ownership == helm.ReleaseForeign {
		return &ReconcileResult{Err: fmt.Errorf(
			"release %s/%s already exists and was not installed by mogenius: set spec.gitOps.%s.enabled to false to keep using it as a user-managed engine",
			namespace, artifact.HelmChart.Name, es.engine)}
	}

	if err := d.ensureNamespace(ctx, namespace); err != nil {
		return &ReconcileResult{Err: fmt.Errorf("ensure namespace %s: %w", namespace, err)}
	}

	if err := installEngineChart(artifact, namespace, ownership == helm.ReleaseOwnedByOperator); err != nil {
		return &ReconcileResult{Err: fmt.Errorf("install %s: %w", cs.name, err)}
	}

	for _, bootstrap := range es.bootstrapObjects(namespace) {
		// The chart just registered this CRD; the checker may still be holding
		// a cached absence from before the install.
		if !d.waitForCRD(ctx, bootstrap.resource) {
			return &ReconcileResult{Err: fmt.Errorf(
				"%s installed but %s did not register within %s", cs.name, bootstrap.resource.Kind, engineCRDTimeout)}
		}
		if err := d.applyPlatformObject(bootstrap.resource, namespace, bootstrap.object); err != nil {
			return &ReconcileResult{Err: fmt.Errorf("apply %s for %s: %w", bootstrap.resource.Kind, cs.name, err)}
		}
	}

	// Everything downstream — the repository Applications, the other platform
	// components — is created as a custom resource the engine has to pick up,
	// so nothing is gained by racing ahead of its controllers.
	if !d.waitForEngineControllers(ctx, es.engine) {
		return &ReconcileResult{Err: fmt.Errorf(
			"%s installed but its controllers were not running within %s", cs.name, engineReadyTimeout)}
	}

	if err := installer.ApplyExtras(cs.name, artifact); err != nil {
		return &ReconcileResult{Err: fmt.Errorf("apply resources for %s: %w", cs.name, err)}
	}

	// The engine that was missing when this reconcile started is running now,
	// and the detection the caller is about to report predates it.
	if det := detection.forEngine(es.engine); det == nil || !det.installed {
		d.requeuePlatformConfig()
	}

	return nil
}

// installEngineChart installs or upgrades the engine's Helm release. OCI and
// classic repositories take different entry points on the way in, but the same
// one on the way up.
func installEngineChart(artifact gitops.GitOpsArtifact, namespace string, exists bool) error {
	values, err := yaml.Marshal(artifact.Values)
	if err != nil {
		return fmt.Errorf("marshal values: %w", err)
	}

	chart := artifact.HelmChart

	if strings.HasPrefix(chart.Repository, "oci://") {
		request := helm.HelmChartInstallUpgradeRequest{
			Namespace: namespace,
			// For OCI the repository already is the full chart reference.
			Chart:   chart.Repository,
			Release: chart.Name,
			Version: chart.Version,
			Values:  string(values),
		}
		if exists {
			_, err = helm.HelmReleaseUpgrade(request)
			return err
		}
		_, err = helm.HelmOciInstall(helm.HelmChartOciInstallUpgradeRequest{
			OCIChartUrl: chart.Repository,
			Namespace:   namespace,
			Release:     chart.Name,
			Version:     chart.Version,
			Values:      string(values),
		})
		return err
	}

	// A classic repository has to be in helm's repository file before the
	// chart can be located by name. Prefixed so it cannot collide with a
	// repository the user added under the chart's own name.
	repoName := "mogenius-" + chart.Chart
	if _, err := helm.HelmRepoAdd(helm.HelmRepoAddRequest{Name: repoName, Url: chart.Repository}); err != nil &&
		!strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("add repository %s: %w", chart.Repository, err)
	}

	request := helm.HelmChartInstallUpgradeRequest{
		Namespace: namespace,
		Chart:     repoName + "/" + chart.Chart,
		Release:   chart.Name,
		Version:   chart.Version,
		Values:    string(values),
	}
	if exists {
		_, err = helm.HelmReleaseUpgrade(request)
		return err
	}
	_, err = helm.HelmChartInstall(request)
	return err
}

// ensureNamespace creates the engine's namespace. Helm does not create it, and
// on a cluster being onboarded neither flux-system nor argocd exists yet.
func (d *reconcilerModule) ensureNamespace(ctx context.Context, namespace string) error {
	client := d.clientProvider.K8sClientSet().CoreV1().Namespaces()

	if _, err := client.Get(ctx, namespace, metav1.GetOptions{}); err == nil {
		return nil
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("read namespace: %w", err)
	}

	_, err := client.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   namespace,
			Labels: map[string]string{"app.kubernetes.io/managed-by": "mogenius-operator"},
		},
	}, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create namespace: %w", err)
	}
	return nil
}

// waitForCRD re-probes one resource until the API server serves it. The cached
// absence is dropped first, because it may predate the install that registered
// the definition.
func (d *reconcilerModule) waitForCRD(ctx context.Context, resource utils.ResourceDescriptor) bool {
	d.crdChecker.forget(resource)

	deadline := time.Now().Add(engineCRDTimeout)
	for {
		if d.crdChecker.IsAvailable(resource) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(enginePollInterval):
			d.crdChecker.forget(resource)
		}
	}
}

// waitForEngineControllers polls the engine detection until it reports the
// engine as installed, which only happens once its controllers actually run.
func (d *reconcilerModule) waitForEngineControllers(ctx context.Context, engine string) bool {
	deadline := time.Now().Add(engineReadyTimeout)
	for {
		if detected := d.detectGitOpsStatus(ctx).forEngine(engine); detected != nil && detected.installed {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(enginePollInterval):
		}
	}
}

// requeuePlatformConfig re-reconciles the PlatformConfig without waiting for
// the background sweep.
//
// Installing the engine changes nothing about the resource, so it produces no
// watch event, and the sweep is fifteen minutes away. Only called after real
// progress — requeueing while merely waiting for something would spin.
func (d *reconcilerModule) requeuePlatformConfig() {
	if d.requeue == nil {
		return
	}
	d.requeue(utils.PlatformConfigResource, func(*unstructured.Unstructured) bool { return true })
}
