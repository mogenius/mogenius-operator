package kubernetes

import (
	"context"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"sort"
	"strings"

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

func PodIdsFor(namespace string, serviceId *string) []string {
	result := []string{}

	var provider *KubeProviderMetrics
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		provider, err = NewKubeProviderMetricsLocal()
	} else {
		provider, err = NewKubeProviderMetricsInCluster()
	}
	if err != nil {
		logger.Log.Errorf("PodIdsForServiceId ERROR: %s", err.Error())
	}

	podMetricsList, err := provider.ClientSet.MetricsV1beta1().PodMetricses(namespace).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("PodIdsForServiceId podMetricsList ERROR: %s", err.Error())
	}

	for _, podMetrics := range podMetricsList.Items {
		if serviceId != nil {
			if strings.Contains(podMetrics.ObjectMeta.Name, *serviceId) {
				result = append(result, podMetrics.ObjectMeta.Name)
			}
		} else {
			result = append(result, podMetrics.ObjectMeta.Name)
		}
	}
	// SORT TO HAVE A DETERMINISTIC ORDERING
	sort.Strings(result)

	return result
}

func filterStatus(pod *v1.Pod) {
	pod.ManagedFields = nil
	pod.ObjectMeta = metav1.ObjectMeta{}
	pod.Spec = v1.PodSpec{}
}
