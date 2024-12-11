package kubernetes

import (
	"context"

	v1job "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllJobs(namespaceName string) []v1job.Job {
	result := []v1job.Job{}

	clientset := clientProvider.K8sClientSet()
	jobList, err := clientset.BatchV1().Jobs(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		k8sLogger.Error("AllJobs", "error", err.Error())
		return result
	}

	for _, job := range jobList.Items {
		job.Kind = "Job"
		job.APIVersion = "batch/v1"
		result = append(result, job)
	}
	return result
}

func GetJob(namespaceName string, name string) (*v1job.Job, error) {
	clientset := clientProvider.K8sClientSet()
	job, err := clientset.BatchV1().Jobs(namespaceName).Get(context.TODO(), name, metav1.GetOptions{})
	job.Kind = "Job"
	job.APIVersion = "batch/v1"

	return job, err
}
