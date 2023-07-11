package kubernetes

import (
	"context"
	"os/exec"

	"mogenius-k8s-manager/logger"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllPersistentVolumeClaims(namespaceName string) []core.PersistentVolumeClaim {
	result := []core.PersistentVolumeClaim{}

	provider := NewKubeProvider()
	pvList, err := provider.ClientSet.CoreV1().PersistentVolumeClaims(namespaceName).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Log.Errorf("AllPersistentVolumeClaims ERROR: %s", err.Error())
		return result
	}

	for _, pv := range pvList.Items {
		result = append(result, pv)
	}
	return result
}

func UpdateK8sPersistentVolumeClaim(data core.PersistentVolumeClaim) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	pvcClient := kubeProvider.ClientSet.CoreV1().PersistentVolumeClaims(data.Namespace)
	_, err := pvcClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DeleteK8sPersistentVolumeClaim(data core.PersistentVolumeClaim) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	pvcClient := kubeProvider.ClientSet.CoreV1().PersistentVolumeClaims(data.Namespace)
	err := pvcClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DescribeK8sPersistentVolumeClaim(namespace string, name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "persistentvolumeclaim", name, "-n", namespace)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		return WorkloadResult(err.Error())
	}
	return WorkloadResult(string(output))
}
