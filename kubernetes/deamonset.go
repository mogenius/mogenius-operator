package kubernetes

import (
	"mogenius-k8s-manager/store"
	"reflect"

	v1 "k8s.io/api/apps/v1"
)

func ListAllDaemonSets(namespace string) ([]v1.DaemonSet, error) {
	result := []v1.DaemonSet{}
	daemonsets, err := store.GlobalStore.SearchByPrefix(reflect.TypeOf(v1.DaemonSet{}), "DaemonSet", namespace)

	if err != nil {
		k8sLogger.Error("ListAllDaemonSet", "error", err)
		return result, err
	}

	for _, ref := range daemonsets {
		if ref == nil {
			continue
		}

		daemonset := ref.(*v1.DaemonSet)
		if daemonset == nil {
			continue
		}

		result = append(result, *daemonset)
	}

	return result, nil
}
