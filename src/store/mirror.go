package store

import (
	"errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/utils"
	"reflect"

	appsV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	networkingV1 "k8s.io/api/networking/v1"
)

func ListNetworkPolicies(namespace string) ([]networkingV1.NetworkPolicy, error) {
	result := []networkingV1.NetworkPolicy{}

	assert.Assert(GlobalStore != nil)

	// TODO replace with GetAvailableResources in the future
	resourceNamespace := ""
	resource := utils.SyncResourceEntry{
		Kind:      "NetworkPolicy",
		Name:      "networkpolicies",
		Namespace: &resourceNamespace,
		Group:     "networking.k8s.io/v1",
		Version:   "",
	}

	policies, err := GlobalStore.SearchByPrefix(reflect.TypeOf(networkingV1.NetworkPolicy{}), resource.Group, "NetworkPolicy", namespace)
	if errors.Is(err, ErrNotFound) {
		return result, nil
	}
	if err != nil {
		return result, err
	}

	for _, ref := range policies {
		if ref == nil {
			continue
		}

		netpol := ref.(*networkingV1.NetworkPolicy)
		if netpol == nil {
			continue
		}

		if namespace != "" && netpol.Namespace != namespace {
			continue
		}

		result = append(result, *netpol)
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

	daemonsets, err := GlobalStore.SearchByPrefix(reflect.TypeOf(appsV1.DaemonSet{}), resource.Group, "DaemonSet", namespace)
	if errors.Is(err, ErrNotFound) {
		return result, nil
	}
	if err != nil {
		return result, err
	}

	for _, ref := range daemonsets {
		if ref == nil {
			continue
		}

		daemonset := ref.(*appsV1.DaemonSet)
		if daemonset == nil {
			continue
		}

		if namespace != "" && daemonset.Namespace != namespace {
			continue
		}

		result = append(result, *daemonset)
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
	deployments, err := GlobalStore.SearchByPrefix(reflect.TypeOf(appsV1.Deployment{}), resource.Group, "Deployment", namespace)
	if errors.Is(err, ErrNotFound) {
		return result, nil
	}
	if err != nil {
		return result, err
	}

	for _, ref := range deployments {
		if ref == nil {
			continue
		}

		deployment := ref.(*appsV1.Deployment)
		if deployment == nil {
			continue
		}

		if namespace != "" && deployment.Namespace != namespace {
			continue
		}

		result = append(result, *deployment)
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
	statefulsets, err := GlobalStore.SearchByPrefix(reflect.TypeOf(appsV1.StatefulSet{}), resource.Group, "StatefulSet", namespace)
	if errors.Is(err, ErrNotFound) {
		return result, nil
	}
	if err != nil {
		return result, err
	}

	for _, ref := range statefulsets {
		if ref == nil {
			continue
		}

		statefulset := ref.(*appsV1.StatefulSet)
		if statefulset == nil {
			continue
		}

		if namespace != "" && statefulset.Namespace != namespace {
			continue
		}

		result = append(result, *statefulset)
	}

	return result, nil
}

func ListEvents(namespace string) ([]coreV1.Event, error) {
	result := []coreV1.Event{}

	events, err := GlobalStore.SearchByKeyParts(reflect.TypeOf(coreV1.Event{}), "Event", namespace)
	if errors.Is(err, ErrNotFound) {
		return result, nil
	}
	if err != nil {
		return result, err
	}

	for _, ref := range events {
		if ref == nil {
			continue
		}

		event := ref.(*coreV1.Event)
		if event == nil {
			continue
		}

		if namespace != "" && event.Namespace != namespace {
			continue
		}

		result = append(result, *event)
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

	args := append([]string{resource.Group, "Pod"}, parts...)
	pods, err := GlobalStore.SearchByPrefix(reflect.TypeOf(coreV1.Pod{}), args...)
	if errors.Is(err, ErrNotFound) {
		return result, nil
	}
	if err != nil {
		return result, err
	}

	for _, ref := range pods {
		if ref == nil {
			continue
		}

		pod := ref.(*coreV1.Pod)
		if pod == nil {
			continue
		}

		result = append(result, *pod)
	}

	return result, nil
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

	namespaces, err := GlobalStore.SearchByPrefix(reflect.TypeOf(coreV1.Namespace{}), resource.Group, "Namespace")
	if errors.Is(err, ErrNotFound) {
		return result, nil
	}
	if err != nil {
		return result, err
	}

	for _, ref := range namespaces {
		if ref == nil {
			continue
		}

		namespace := ref.(*coreV1.Namespace)
		if namespace == nil {
			continue
		}

		result = append(result, *namespace)
	}

	return result, nil
}

func GetNamespace(name string) *coreV1.Namespace {
	// TODO replace with GetAvailableResources in the future
	resource := utils.SyncResourceEntry{
		Kind:    "Namespace",
		Name:    "namespaces",
		Group:   "v1",
		Version: "",
	}

	ref := GlobalStore.GetByKeyParts(reflect.TypeOf(coreV1.Namespace{}), resource.Group, resource.Kind, name)
	if ref == nil {
		return nil
	}

	namespace := ref.(*coreV1.Namespace)
	if namespace == nil {
		return nil
	}

	return namespace
}

func GetResourceByKindAndNamespace(groupVersion string, kind string, namespace string) []unstructured.Unstructured {
	var results []unstructured.Unstructured

	storeResults, err := GlobalStore.SearchByPrefix(reflect.TypeOf(unstructured.Unstructured{}), groupVersion, kind, namespace)
	if errors.Is(err, ErrNotFound) {
		return results
	}
	if err != nil {
		return results
	}

	for _, ref := range storeResults {
		if ref == nil {
			continue
		}

		obj := ref.(*unstructured.Unstructured)
		if obj == nil {
			continue
		}

		if (namespace != "" && obj.GetNamespace() != namespace) || (kind != "" && obj.GetKind() != kind) {
			continue
		}

		results = append(results, *obj)
	}
	return results
}
