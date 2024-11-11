package store

import (
	"errors"
	"reflect"

	appsV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	networkingV1 "k8s.io/api/networking/v1"
)

func ListNetworkPolicies(namespace string) ([]networkingV1.NetworkPolicy, error) {
	result := []networkingV1.NetworkPolicy{}

	policies, err := GlobalStore.SearchByPrefix(reflect.TypeOf(networkingV1.NetworkPolicy{}), "NetworkPolicy", namespace)
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

		result = append(result, *netpol)
	}

	return result, nil
}

func ListDaemonSets(namespace string) ([]appsV1.DaemonSet, error) {
	result := []appsV1.DaemonSet{}

	daemonsets, err := GlobalStore.SearchByPrefix(reflect.TypeOf(appsV1.DaemonSet{}), "DaemonSet", namespace)
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

		result = append(result, *daemonset)
	}

	return result, nil
}

func ListDeployments(namespace string) ([]appsV1.Deployment, error) {
	result := []appsV1.Deployment{}

	deployments, err := GlobalStore.SearchByPrefix(reflect.TypeOf(appsV1.Deployment{}), "Deployment", namespace)
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

		result = append(result, *deployment)
	}

	return result, nil
}

func ListStatefulSets(namespace string) ([]appsV1.StatefulSet, error) {
	result := []appsV1.StatefulSet{}

	statefulsets, err := GlobalStore.SearchByPrefix(reflect.TypeOf(appsV1.StatefulSet{}), "StatefulSet", namespace)
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

		result = append(result, *statefulset)
	}

	return result, nil
}

func ListEvents(namespace string) ([]coreV1.Event, error) {
	result := []coreV1.Event{}

	events, err := GlobalStore.SearchByPrefix(reflect.TypeOf(coreV1.Event{}), "Event", namespace)
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

		result = append(result, *event)
	}

	return result, nil
}

func ListPods(parts ...string) ([]coreV1.Pod, error) {
	result := []coreV1.Pod{}

	args := append([]string{"Pod"}, parts...)
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

	namespaces, err := GlobalStore.SearchByPrefix(reflect.TypeOf(coreV1.Namespace{}), "Namespace")
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
	ref := GlobalStore.GetByKeyParts(reflect.TypeOf(coreV1.Namespace{}), "Namespace", name)
	if ref == nil {
		return nil
	}

	namespace := ref.(*coreV1.Namespace)
	if namespace == nil {
		return nil
	}

	return namespace
}
