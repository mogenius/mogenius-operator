package kubernetes

import (
	"context"

	v1job "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetJob(namespaceName string, name string) (*v1job.Job, error) {
	clientset := clientProvider.K8sClientSet()
	job, err := clientset.BatchV1().Jobs(namespaceName).Get(context.TODO(), name, metav1.GetOptions{})
	job.Kind = "Job"
	job.APIVersion = "batch/v1"

	return job, err
}
