package kubernetes

import (
	"context"
	"os/exec"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	v1job "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllJobs(namespaceName string) []v1job.Job {
	result := []v1job.Job{}

	provider := NewKubeProvider()
	jobList, err := provider.ClientSet.BatchV1().Jobs(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllJobs ERROR: %s", err.Error())
		return result
	}

	for _, job := range jobList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, job.ObjectMeta.Namespace) {
			result = append(result, job)
		}
	}
	return result
}

func UpdateK8sJob(data v1job.Job) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	jobClient := kubeProvider.ClientSet.BatchV1().Jobs(data.Namespace)
	_, err := jobClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DeleteK8sJob(data v1job.Job) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	jobClient := kubeProvider.ClientSet.BatchV1().Jobs(data.Namespace)
	err := jobClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DescribeK8sJob(namespace string, name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "job", name, "-n", namespace)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		return WorkloadResult(err.Error())
	}
	return WorkloadResult(string(output))
}
