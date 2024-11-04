package kubernetes

import (
	"mogenius-k8s-manager/store"
	"reflect"

	v1 "k8s.io/api/apps/v1"
)

func ListAllStatefulSets(namespace string) ([]v1.StatefulSet, error) {
	result := []v1.StatefulSet{}
	statefulsets, err := store.GlobalStore.SearchByPrefix(reflect.TypeOf(v1.StatefulSet{}), "StatefulSet", namespace)

	if err != nil {
		k8sLogger.Warn("ListAllStatefulSet", "warning", err)
		return result, err
	}

	for _, ref := range statefulsets {
		if ref == nil {
			continue
		}

		statefulset := ref.(*v1.StatefulSet)
		if statefulset == nil {
			continue
		}

		result = append(result, *statefulset)
	}

	return result, nil
}
