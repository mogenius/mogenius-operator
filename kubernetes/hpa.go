package kubernetes

import (
	"context"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	v2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllHpas(namespaceName string) []v2.HorizontalPodAutoscaler {
	result := []v2.HorizontalPodAutoscaler{}

	provider := NewKubeProvider()
	hpaList, err := provider.ClientSet.AutoscalingV2().HorizontalPodAutoscalers(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllHpas ERROR: %s", err.Error())
		return result
	}

	for _, hpa := range hpaList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, hpa.ObjectMeta.Namespace) {
			result = append(result, hpa)
		}
	}
	return result
}

func UpdateK8sHpa(data v2.HorizontalPodAutoscaler) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	hpaClient := kubeProvider.ClientSet.AutoscalingV2().HorizontalPodAutoscalers(data.Namespace)
	_, err := hpaClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DeleteK8sHpa(data v2.HorizontalPodAutoscaler) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	hpaClient := kubeProvider.ClientSet.AutoscalingV2().HorizontalPodAutoscalers(data.Namespace)
	err := hpaClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}
