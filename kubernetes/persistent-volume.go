package kubernetes

import (
	"context"
	"os/exec"

	"mogenius-k8s-manager/logger"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllPersistentVolumes() K8sWorkloadResult {
	result := []core.PersistentVolume{}

	provider := NewKubeProvider()
	pvList, err := provider.ClientSet.CoreV1().PersistentVolumes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Log.Errorf("AllPersistentVolumes ERROR: %s", err.Error())
		return WorkloadResult(nil, err)
	}

	for _, pv := range pvList.Items {
		result = append(result, pv)
	}
	return WorkloadResult(result, nil)
}

func UpdateK8sPersistentVolume(data core.PersistentVolume) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	pvClient := kubeProvider.ClientSet.CoreV1().PersistentVolumes()
	_, err := pvClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DeleteK8sPersistentVolume(data core.PersistentVolume) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	pvClient := kubeProvider.ClientSet.CoreV1().PersistentVolumes()
	err := pvClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DescribeK8sPersistentVolume(name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "persistentvolume", name)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		logger.Log.Errorf("Error: %s", string(output))
		return WorkloadResult(nil, string(output))
	}
	return WorkloadResult(string(output), nil)
}
