package store

import (
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeyclient"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appsV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	networkingV1 "k8s.io/api/networking/v1"
)

func ListNetworkPolicies(valkeyClient valkeyclient.ValkeyClient, namespace string) ([]networkingV1.NetworkPolicy, error) {
	result := []networkingV1.NetworkPolicy{}

	// TODO replace with GetAvailableResources in the future
	resourceNamespace := ""
	resource := utils.SyncResourceEntry{
		Kind:      "NetworkPolicy",
		Name:      "networkpolicies",
		Namespace: &resourceNamespace,
		Group:     "networking.k8s.io/v1",
		Version:   "",
	}

	policies, err := valkeyclient.GetObjectsByPrefix[networkingV1.NetworkPolicy](valkeyClient, valkeyclient.ORDER_NONE, VALKEY_RESOURCE_PREFIX, resource.Group, "NetworkPolicy", namespace)
	if err != nil {
		return result, err
	}

	for _, ref := range policies {
		if namespace != "" && ref.Namespace != namespace {
			continue
		}

		result = append(result, ref)
	}

	return result, nil
}

func ListDaemonSets(valkeyClient valkeyclient.ValkeyClient, namespace string) ([]appsV1.DaemonSet, error) {
	result := []appsV1.DaemonSet{}

	// TODO replace with GetAvailableResources in the future
	resourceNamespace := ""
	resource := utils.SyncResourceEntry{
		Kind:      "DaemonSet",
		Name:      "daemonsets",
		Namespace: &resourceNamespace,
		Group:     "apps/v1",
		Version:   "",
	}

	daemonsets, err := valkeyclient.GetObjectsByPrefix[appsV1.DaemonSet](valkeyClient, valkeyclient.ORDER_NONE, resource.Group, "DaemonSet", namespace)
	if err != nil {
		return result, err
	}

	for _, ref := range daemonsets {
		if namespace != "" && ref.Namespace != namespace {
			continue
		}

		result = append(result, ref)
	}

	return result, nil
}

func ListDeployments(valkeyClient valkeyclient.ValkeyClient, namespace string) ([]appsV1.Deployment, error) {
	result := []appsV1.Deployment{}

	// TODO replace with GetAvailableResources in the future
	resourceNamespace := ""
	resource := utils.SyncResourceEntry{
		Kind:      "Deployment",
		Name:      "deployments",
		Namespace: &resourceNamespace,
		Group:     "apps/v1",
		Version:   "",
	}

	deployments, err := valkeyclient.GetObjectsByPrefix[appsV1.Deployment](valkeyClient, valkeyclient.ORDER_NONE, VALKEY_RESOURCE_PREFIX, resource.Group, "Deployment", namespace)
	if err != nil {
		return result, err
	}

	for _, ref := range deployments {
		if namespace != "" && ref.Namespace != namespace {
			continue
		}

		result = append(result, ref)
	}

	return result, nil
}

func ListStatefulSets(valkeyClient valkeyclient.ValkeyClient, namespace string) ([]appsV1.StatefulSet, error) {
	result := []appsV1.StatefulSet{}

	// TODO replace with GetAvailableResources in the future
	resourceNamespace := ""
	resource := utils.SyncResourceEntry{
		Kind:      "StatefulSet",
		Name:      "statefulsets",
		Namespace: &resourceNamespace,
		Group:     "apps/v1",
		Version:   "",
	}
	statefulsets, err := valkeyclient.GetObjectsByPrefix[appsV1.StatefulSet](valkeyClient, valkeyclient.ORDER_NONE, VALKEY_RESOURCE_PREFIX, resource.Group, "StatefulSet", namespace)
	if err != nil {
		return result, err
	}

	for _, ref := range statefulsets {
		if namespace != "" && ref.Namespace != namespace {
			continue
		}

		result = append(result, ref)
	}

	return result, nil
}

func ListEvents(valkeyClient valkeyclient.ValkeyClient, namespace string) ([]coreV1.Event, error) {
	result := []coreV1.Event{}

	events, err := valkeyclient.GetObjectsByPrefix[coreV1.Event](valkeyClient, valkeyclient.ORDER_DESC, VALKEY_RESOURCE_PREFIX, "v1", "Event", namespace)
	if err != nil {
		return result, err
	}

	for _, ref := range events {
		if namespace != "" && ref.Namespace != namespace {
			continue
		}

		result = append(result, ref)
	}

	return result, nil
}

func ListPods(valkeyClient valkeyclient.ValkeyClient, parts ...string) ([]coreV1.Pod, error) {
	result := []coreV1.Pod{}

	// TODO replace with GetAvailableResources in the future
	resourceNamespace := ""
	resource := utils.SyncResourceEntry{
		Kind:      "Pod",
		Name:      "pods",
		Namespace: &resourceNamespace,
		Group:     "v1",
		Version:   "",
	}

	args := append([]string{VALKEY_RESOURCE_PREFIX, resource.Group, "Pod"}, parts...)
	pods, err := valkeyclient.GetObjectsByPrefix[coreV1.Pod](valkeyClient, valkeyclient.ORDER_NONE, args...)
	if err != nil {
		return result, err
	}

	return pods, nil
}

func ListAllNamespaces(valkeyClient valkeyclient.ValkeyClient) ([]coreV1.Namespace, error) {
	result := []coreV1.Namespace{}

	// TODO replace with GetAvailableResources in the future
	resourceNamespace := ""
	resource := utils.SyncResourceEntry{
		Kind:      "Namespace",
		Name:      "namespaces",
		Namespace: &resourceNamespace,
		Group:     "v1",
		Version:   "",
	}

	namespaces, err := valkeyclient.GetObjectsByPrefix[coreV1.Namespace](valkeyClient, valkeyclient.ORDER_NONE, VALKEY_RESOURCE_PREFIX, resource.Group, "Namespace")
	if err != nil {
		return result, err
	}

	return namespaces, nil
}

func GetNamespace(valkeyClient valkeyclient.ValkeyClient, name string) *coreV1.Namespace {
	// TODO replace with GetAvailableResources in the future
	resource := utils.SyncResourceEntry{
		Kind:    "Namespace",
		Name:    "namespaces",
		Group:   "v1",
		Version: "",
	}

	namespace, _ := valkeyclient.GetObjectForKey[coreV1.Namespace](valkeyClient, VALKEY_RESOURCE_PREFIX, resource.Group, resource.Kind, "", name)
	return namespace
}

func GetResourceByKindAndNamespace(valkeyClient valkeyclient.ValkeyClient, groupVersion string, kind string, namespace string) []unstructured.Unstructured {
	var results []unstructured.Unstructured

	storeResults, err := valkeyclient.GetObjectsByPrefix[unstructured.Unstructured](valkeyClient, valkeyclient.ORDER_NONE, VALKEY_RESOURCE_PREFIX, groupVersion, kind, namespace)
	if err != nil {
		return results
	}

	for _, ref := range storeResults {
		if (namespace != "" && ref.GetNamespace() != namespace) || (kind != "" && ref.GetKind() != kind) {
			continue
		}

		results = append(results, ref)
	}
	return results
}
