package kubernetes

import (
	"context"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllStatefulSets(namespaceName string) []v1.StatefulSet {
	result := []v1.StatefulSet{}

	provider := NewKubeProvider()
	statefulSetList, err := provider.ClientSet.AppsV1().StatefulSets(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllStatefulSets ERROR: %s", err.Error())
		return result
	}

	for _, statefulSet := range statefulSetList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, statefulSet.ObjectMeta.Namespace) {
			result = append(result, statefulSet)
		}
	}
	return result
}

func UpdateK8sStatefulset(data v1.StatefulSet) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	statefulsetClient := kubeProvider.ClientSet.AppsV1().StatefulSets(data.Namespace)
	_, err := statefulsetClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DeleteK8sStatefulset(data v1.StatefulSet) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	statefulsetClient := kubeProvider.ClientSet.AppsV1().StatefulSets(data.Namespace)
	err := statefulsetClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}
