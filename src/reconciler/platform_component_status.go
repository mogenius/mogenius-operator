package reconciler

import (
	"context"

	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// componentDeclaration is what the spec says about one component: whether it
// carries a block for it at all, and whether that block enables it.
//
// The distinction drives the status. A component the spec never mentions is not
// this operator's business — reporting it as Ready made a status where
// cert-manager, external-dns and the monitoring stack all looked installed on a
// cluster that declared none of them, which is worse than saying nothing.
type componentDeclaration struct {
	declared bool
	enabled  bool
}

// declaredComponents maps every component to what the spec declares about it.
//
// Written out rather than reflected over: the component constants name
// platform-defaults files, the spec fields are Go names, and a mapping that
// guesses between the two breaks silently when either side is renamed.
func declaredComponents(spec v1alpha1.PlatformConfigSpec) map[string]componentDeclaration {
	declared := make(map[string]componentDeclaration, 11)

	if spec.GitOps != nil {
		if spec.GitOps.FluxCD != nil {
			declared[componentFluxCD] = componentDeclaration{declared: true, enabled: spec.GitOps.FluxCD.Enabled}
		}
		if spec.GitOps.ArgoCD != nil {
			declared[componentArgoCD] = componentDeclaration{declared: true, enabled: spec.GitOps.ArgoCD.Enabled}
		}
		if len(spec.GitOps.Repositories) > 0 {
			declared[componentPlatformRepositories] = componentDeclaration{declared: true, enabled: true}
		}
	}
	if spec.CertManager != nil {
		declared[componentCertManager] = componentDeclaration{declared: true, enabled: spec.CertManager.Enabled}
	}
	if spec.Traefik != nil {
		declared[componentTraefik] = componentDeclaration{declared: true, enabled: spec.Traefik.Enabled}
	}
	if spec.ExternalDNS != nil {
		declared[componentExternalDNS] = componentDeclaration{declared: true, enabled: spec.ExternalDNS.Enabled}
	}
	if spec.KubePrometheusStack != nil {
		declared[componentKubePrometheusStack] = componentDeclaration{declared: true, enabled: spec.KubePrometheusStack.Enabled}
	}
	if spec.Loki != nil {
		declared[componentLoki] = componentDeclaration{declared: true, enabled: spec.Loki.Enabled}
	}
	if spec.Alloy != nil {
		declared[componentAlloy] = componentDeclaration{declared: true, enabled: spec.Alloy.Enabled}
	}
	if spec.RenovateOperator != nil {
		declared[componentRenovateOperator] = componentDeclaration{declared: true, enabled: spec.RenovateOperator.Enabled}
	}
	if spec.ExternalSecretsOperator != nil {
		declared[componentExternalSecretsOperator] = componentDeclaration{declared: true, enabled: spec.ExternalSecretsOperator.Enabled}
	}

	return declared
}

// deliveryState is what the GitOps engine reports about one component's object.
type deliveryState struct {
	// found is false when the engine has no object for the component yet.
	found bool
	// ready is the engine's own verdict.
	ready bool
	// message is the engine's explanation, shown verbatim in the condition.
	message string
}

// deliveryStates asks the engine how the objects it was handed are actually
// doing, indexed by component name.
//
// Applying a HelmRelease or an Application succeeds long before the release it
// describes is installed, so "the operator wrote the object" is not an answer
// to "is the component running". Without reading this back, a chart that fails
// its values schema — the operator's own defaults against a newer chart, say —
// leaves the PlatformConfig reporting Ready while nothing runs, and the only
// place the failure exists is the engine's own object.
//
// One list per engine rather than a get per component: nine gets on every
// sweep is a cost the status does not justify.
func (d *reconcilerModule) deliveryStates(ctx context.Context, engine string, namespace string) map[string]deliveryState {
	var resource utils.ResourceDescriptor
	switch engine {
	case gitOpsEngineFlux:
		resource = utils.FluxHelmReleaseResource
	case gitOpsEngineArgoCD:
		resource = utils.ArgoApplicationResource
	default:
		return nil
	}

	if !d.crdChecker.IsAvailable(resource) {
		return nil
	}

	list, err := d.clientProvider.DynamicClient().
		Resource(kubernetes.CreateGroupVersionResource(resource.ApiVersion, resource.Plural)).
		Namespace(namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		// Not fatal: a status that cannot be read is reported as pending, which
		// is honest, where failing the reconcile over it would stop the
		// platform for the sake of a message.
		d.logger.Warn("could not read component delivery status",
			"kind", resource.Kind, "namespace", namespace, "error", err)
		return nil
	}

	states := make(map[string]deliveryState, len(list.Items))
	for i := range list.Items {
		item := list.Items[i]
		name := item.GetName()
		switch engine {
		case gitOpsEngineFlux:
			states[name] = fluxDeliveryState(&item)
		case gitOpsEngineArgoCD:
			states[name] = argoDeliveryState(&item)
		}
	}
	return states
}

// fluxDeliveryState reads a HelmRelease's Ready condition.
func fluxDeliveryState(object *unstructured.Unstructured) deliveryState {
	conditions, found, err := unstructured.NestedSlice(object.Object, "status", "conditions")
	if err != nil || !found {
		return deliveryState{found: true}
	}

	for _, raw := range conditions {
		condition, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if conditionType, _, _ := unstructured.NestedString(condition, "type"); conditionType != "Ready" {
			continue
		}
		status, _, _ := unstructured.NestedString(condition, "status")
		message, _, _ := unstructured.NestedString(condition, "message")
		return deliveryState{found: true, ready: status == "True", message: message}
	}

	return deliveryState{found: true}
}

// argoDeliveryState reads an Application's health and sync status.
//
// Both matter: a synced Application whose workload is Degraded is not a running
// component, and a Healthy one that never synced is reporting on the previous
// revision.
func argoDeliveryState(object *unstructured.Unstructured) deliveryState {
	health, _, _ := unstructured.NestedString(object.Object, "status", "health", "status")
	sync, _, _ := unstructured.NestedString(object.Object, "status", "sync", "status")

	if health == "" && sync == "" {
		return deliveryState{found: true}
	}

	ready := health == "Healthy" && sync == "Synced"
	message := "health " + orUnknown(health) + ", sync " + orUnknown(sync)
	if !ready {
		if reason, _, _ := unstructured.NestedString(object.Object, "status", "operationState", "message"); reason != "" {
			message += ": " + reason
		}
	}
	return deliveryState{found: true, ready: ready, message: message}
}

func orUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

// deliveredStateFor answers what is known about one component's delivery.
//
// The engine components are the exception: mogenius installs those with the
// Helm SDK, so there is no HelmRelease or Application to read and the detected
// engine status is the only truth about them.
func deliveredStateFor(
	component string,
	delivered map[string]deliveryState,
	gitOpsStatus *v1alpha1.GitOpsStatus,
) (deliveryState, bool) {
	if component == componentFluxCD || component == componentArgoCD {
		if gitOpsStatus == nil {
			return deliveryState{}, false
		}
		if gitOpsStatus.Installed {
			return deliveryState{found: true, ready: true, message: "controllers running"}, true
		}
		return deliveryState{found: true, message: "installed, but its controllers are not running"}, true
	}

	// Not delivered through an engine object at all: the repository sync
	// objects are applied directly, and their own reconcile result is the only
	// verdict there is.
	if component == componentPlatformRepositories {
		return deliveryState{}, false
	}

	state, ok := delivered[component]
	return state, ok
}
