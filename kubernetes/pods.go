package kubernetes

import (
	"context"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func PodStatus(resource string, namespace string, name string, statusOnly bool) *v1.Pod {
	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}
	if err != nil {
		logger.Log.Errorf("CreateNamespace ERROR: %s", err.Error())
	}

	getOptions := metav1.GetOptions{}

	podClient := kubeProvider.ClientSet.CoreV1().Pods(namespace)

	pod, err := podClient.Get(context.TODO(), name, getOptions)
	if err != nil {
		logger.Log.Error("PodStatus Error: %s", err.Error())
	}

	if statusOnly {
		filterStatus(pod)
	}

	return pod
}

func filterStatus(pod *v1.Pod) {
	pod.ManagedFields = nil
	pod.ObjectMeta = metav1.ObjectMeta{}
	pod.Spec = v1.PodSpec{}
}
