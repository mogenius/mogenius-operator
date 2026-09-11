package reconciler

import (
	"context"
	"fmt"

	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/gitops"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/utils"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// repositorySecretSuffix is the conventional name of a repository's credential
// Secret: <repository>-repository, in the engine's namespace. The bootstrap
// chart writes it under that name and so does the mogenius API, so the operator
// can find it without the PlatformConfig having to name it.
const repositorySecretSuffix = "-repository"

// reconcilePlatformRepositories creates the objects that make a GitOps engine
// sync spec.gitOps.repositories.
//
// Only for an engine mogenius does not own. When it does own one, the same
// objects ship as extra objects of the release it installs — doing both would
// have two writers for one Application.
//
// This is what closes the bootstrap loop. The operator seeds the PlatformConfig
// with the repository it was installed with, and reconciling that entry is what
// brings the real config into the cluster. It runs on every sweep, so a CRD
// that is not established yet simply succeeds on a later pass instead of
// needing anything to retry it from outside.
func (d *reconcilerModule) reconcilePlatformRepositories(
	ctx context.Context,
	spec v1alpha1.PlatformConfigSpec,
	engine string,
	namespace string,
) *ReconcileResult {
	if spec.GitOps == nil || len(spec.GitOps.Repositories) == 0 {
		return nil
	}

	if engine == gitOpsEngineFlux {
		// The FluxInstance deploys the controllers; the flux-operator chart
		// ships none. Absent on a cluster whose Flux was set up by hand, where
		// the controllers exist already and this CRD does not.
		if d.crdChecker.IsAvailable(utils.FluxInstanceResource) {
			if err := d.applyPlatformObject(ctx, utils.FluxInstanceResource, namespace, fluxInstanceObject(namespace)); err != nil {
				return &ReconcileResult{Err: fmt.Errorf("apply flux instance: %w", err)}
			}
		}
	}

	for _, repo := range spec.GitOps.Repositories {
		// Application repositories are declared for the mogenius platform, not
		// for this operator: no sync objects.
		if !repo.IsPlatformRepository() {
			continue
		}
		name := repo.Name
		if name == "" {
			name = repositorySecretName(repo.URL)
		}
		// Without a path there is no directory to sync, which is how the engine
		// addresses a repository at all.
		if repo.Path == "" {
			continue
		}

		switch engine {
		case gitOpsEngineArgoCD:
			// No credential reference: Argo CD discovers repository credentials
			// by label, so the Secret stands on its own.
			object := argoApplicationObject(name, repo, namespace, argoProjectName(spec.GitOps))
			if err := d.applyPlatformObject(ctx, utils.ArgoApplicationResource, namespace, object); err != nil {
				return &ReconcileResult{Err: fmt.Errorf("apply application %q: %w", name, err)}
			}

		case gitOpsEngineFlux:
			source := fluxGitRepositoryObject(name, repo, namespace, d.fluxRepositorySecretName(ctx, name, repo, namespace))
			if err := d.applyPlatformObject(ctx, utils.GitRepositoryResource, namespace, source); err != nil {
				return &ReconcileResult{Err: fmt.Errorf("apply git repository %q: %w", name, err)}
			}
			if err := d.applyPlatformObject(ctx, utils.KustomizationResource, namespace, fluxKustomizationObject(name, repo, namespace)); err != nil {
				return &ReconcileResult{Err: fmt.Errorf("apply kustomization %q: %w", name, err)}
			}
		}
	}

	return nil
}

// applyPlatformObject applies one object, labelled so it is recognisable as the
// platform's. No owner reference: these objects are what sync the PlatformConfig
// in, so tying their lifetime to it would have the object delete itself the
// moment the resource it delivers is removed.
//
// An object of the same name that the operator did not create is never
// written. gitops.Apply replaces the spec wholesale, so writing someone
// else's object destroys whatever their spec carried — on a cluster whose
// Flux came through the flux-operator, overwriting the user's FluxInstance
// dropped its spec.sync, and the engine garbage-collected the root
// Kustomization and with it every application it managed. Handing an object
// over to the platform is explicit: label it
// app.kubernetes.io/managed-by=mogenius-operator.
func (d *reconcilerModule) applyPlatformObject(ctx context.Context, resource utils.ResourceDescriptor, namespace string, object map[string]any) error {
	if !d.crdChecker.IsAvailable(resource) {
		// The engine's CRDs are not registered yet. A later sweep finds them.
		d.logger.Info("skipping platform repository object, CRD not available yet",
			"kind", resource.Kind, "namespace", namespace)
		return nil
	}

	obj := &unstructured.Unstructured{Object: object}
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels["app.kubernetes.io/managed-by"] = "mogenius-operator"
	labels["app.kubernetes.io/component"] = "platform-config"
	obj.SetLabels(labels)

	gvr := kubernetes.CreateGroupVersionResource(resource.ApiVersion, resource.Plural)
	existing, err := d.clientProvider.DynamicClient().Resource(gvr).Namespace(namespace).Get(ctx, obj.GetName(), metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		existing = nil
	case err != nil:
		// Not evidence of absence. Writing blind past a failed ownership check
		// is exactly the overwrite this guard exists to prevent.
		return fmt.Errorf("check ownership of %s %s/%s: %w", resource.Kind, namespace, obj.GetName(), err)
	}

	if platformObjectForeign(existing) {
		d.logger.Warn("skipping platform object: it already exists and was not created by mogenius",
			"kind", resource.Kind, "namespace", namespace, "name", obj.GetName(),
			"hint", "label it app.kubernetes.io/managed-by=mogenius-operator to hand it over to the platform")
		return nil
	}

	if resource.Kind == utils.FluxInstanceResource.Kind && existing != nil {
		preserveFluxInstanceSync(existing, obj)
	}

	return gitops.Apply(d.clientProvider, gvr, namespace, obj)
}

// platformObjectForeign reports whether an existing object belongs to someone
// other than the platform reconciler. Absence is not foreign — a missing
// object is free to create.
func platformObjectForeign(existing *unstructured.Unstructured) bool {
	if existing == nil {
		return false
	}
	return existing.GetLabels()["app.kubernetes.io/managed-by"] != "mogenius-operator"
}

// preserveFluxInstanceSync carries an existing spec.sync over onto the desired
// object when the template declares none. A FluxInstance's sync is what points
// the whole cluster at its Git repository; applying the bare template over an
// instance that carried one removes it, and the flux-operator then
// garbage-collects the root Kustomization — which prunes everything it ever
// deployed. The ownership guard keeps foreign instances out of reach; this
// keeps a sync someone added to an operator-owned instance alive too.
func preserveFluxInstanceSync(existing, desired *unstructured.Unstructured) {
	if _, found, _ := unstructured.NestedMap(desired.Object, "spec", "sync"); found {
		return
	}
	sync, found, err := unstructured.NestedMap(existing.Object, "spec", "sync")
	if err != nil || !found {
		return
	}
	// Errors only on malformed field paths, which "spec", "sync" is not.
	_ = unstructured.SetNestedMap(desired.Object, sync, "spec", "sync")
}

// fluxRepositorySecretName is the credential a repository's GitRepository binds
// with: the ExternalSecret's target when one is declared, otherwise the
// conventional <name>-repository Secret -- and only when that Secret actually
// exists. Flux is pointed at a credential by name, and a secretRef naming a
// Secret that is not there fails the source outright; a public repository
// needs none.
//
// Shared by both delivery paths on purpose. The engine-extras path once made
// this decision differently from the user-managed path (namely: not at all),
// and a private repository then failed with "authentication required" while
// its credential sat unused in the same namespace.
func (d *reconcilerModule) fluxRepositorySecretName(
	ctx context.Context,
	name string,
	repo v1alpha1.GitOpsRepositoryConfig,
	namespace string,
) string {
	if repo.ExternalSecret != nil {
		return name
	}
	if d.platformRepositorySecretExists(ctx, namespace, name+repositorySecretSuffix) {
		return name + repositorySecretSuffix
	}
	return ""
}

func (d *reconcilerModule) platformRepositorySecretExists(ctx context.Context, namespace, name string) bool {
	_, err := d.clientProvider.K8sClientSet().CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return true
	}
	if !apierrors.IsNotFound(err) {
		// Anything other than absence -- RBAC, an unreachable API server -- is
		// not evidence that the Secret is missing, and dropping the reference
		// over it would break a source that works.
		d.logger.Warn("could not check platform repository secret", "name", name, "error", err)
	}
	return false
}
