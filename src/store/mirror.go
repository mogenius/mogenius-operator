package store

import (
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeystore"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appsV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	networkingV1 "k8s.io/api/networking/v1"
)

func ListNetworkPolicies(namespace string) ([]networkingV1.NetworkPolicy, error) {
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

	policies, err := valkeystore.GetObjectsByPrefix[networkingV1.NetworkPolicy](valkeyStore, valkeystore.ORDER_NONE, VALKEY_KEY_PREFIX, resource.Group, "NetworkPolicy", namespace)
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

func ListDaemonSets(namespace string) ([]appsV1.DaemonSet, error) {
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

	daemonsets, err := valkeystore.GetObjectsByPrefix[appsV1.DaemonSet](valkeyStore, valkeystore.ORDER_NONE, resource.Group, "DaemonSet", namespace)
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

func ListDeployments(namespace string) ([]appsV1.Deployment, error) {
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

	deployments, err := valkeystore.GetObjectsByPrefix[appsV1.Deployment](valkeyStore, valkeystore.ORDER_NONE, VALKEY_KEY_PREFIX, resource.Group, "Deployment", namespace)
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

func ListStatefulSets(namespace string) ([]appsV1.StatefulSet, error) {
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
	statefulsets, err := valkeystore.GetObjectsByPrefix[appsV1.StatefulSet](valkeyStore, valkeystore.ORDER_NONE, VALKEY_KEY_PREFIX, resource.Group, "StatefulSet", namespace)
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

func ListEvents(namespace string) ([]coreV1.Event, error) {
	result := []coreV1.Event{}

	events, err := valkeystore.GetObjectsByPrefix[coreV1.Event](valkeyStore, valkeystore.ORDER_DESC, VALKEY_KEY_PREFIX, "Event", namespace)
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

func ListPods(parts ...string) ([]coreV1.Pod, error) {
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

	args := append([]string{VALKEY_KEY_PREFIX, resource.Group, "Pod"}, parts...)
	pods, err := valkeystore.GetObjectsByPrefix[coreV1.Pod](valkeyStore, valkeystore.ORDER_NONE, args...)
	if err != nil {
		return result, err
	}

	return pods, nil
}

func ListAllNamespaces() ([]coreV1.Namespace, error) {
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

	namespaces, err := valkeystore.GetObjectsByPrefix[coreV1.Namespace](valkeyStore, valkeystore.ORDER_NONE, VALKEY_KEY_PREFIX, resource.Group, "Namespace")
	if err != nil {
		return result, err
	}

	return namespaces, nil
}

func GetNamespace(name string) *coreV1.Namespace {
	// TODO replace with GetAvailableResources in the future
	resource := utils.SyncResourceEntry{
		Kind:    "Namespace",
		Name:    "namespaces",
		Group:   "v1",
		Version: "",
	}

	namespace, _ := valkeystore.GetObjectForKey[coreV1.Namespace](valkeyStore, VALKEY_KEY_PREFIX, resource.Group, resource.Kind, name)
	return namespace
}

func GetResourceByKindAndNamespace(groupVersion string, kind string, namespace string) []unstructured.Unstructured {
	var results []unstructured.Unstructured

	storeResults, err := valkeystore.GetObjectsByPrefix[unstructured.Unstructured](valkeyStore, valkeystore.ORDER_NONE, VALKEY_KEY_PREFIX, groupVersion, kind, namespace)
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
