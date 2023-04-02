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

type ServicePodExistsResult struct {
	PodExists bool `json:"podExists"`
}

func PodStatus(resource string, namespace string, name string, statusOnly bool) *v1.Pod {
	kubeProvider := NewKubeProvider()
	getOptions := metav1.GetOptions{}

	podClient := kubeProvider.ClientSet.CoreV1().Pods(namespace)

	pod, err := podClient.Get(context.TODO(), name, getOptions)
	if err != nil {
		logger.Log.Error("PodStatus Error: %s", err.Error())
		return nil
	}

	if statusOnly {
		filterStatus(pod)
	}

	return pod
}

func PodExists(namespace string, name string) ServicePodExistsResult {
	result := ServicePodExistsResult{}

	kubeProvider := NewKubeProvider()
	podClient := kubeProvider.ClientSet.CoreV1().Pods(namespace)
	pod, err := podClient.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil || pod == nil {
		result.PodExists = false
		return result
	}

	result.PodExists = true
	return result
}

func AllPods(namespaceName string) []v1.Pod {
	result := []v1.Pod{}

	provider := NewKubeProvider()
	podsList, err := provider.ClientSet.CoreV1().Pods(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllPods podMetricsList ERROR: %s", err.Error())
		return result
	}

	for _, pod := range podsList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, pod.ObjectMeta.Namespace) {
			result = append(result, pod)
		}
	}
	return result
}

func AllPodNames() []string {
	result := []string{}
	allPods := AllPods("")
	for _, pod := range allPods {
		result = append(result, pod.ObjectMeta.Name)
	}
	return result
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
		return result
	}

	podMetricsList, err := provider.ClientSet.MetricsV1beta1().PodMetricses(namespace).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("PodIdsForServiceId podMetricsList ERROR: %s", err.Error())
		return result
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

func UpdateK8sPod(data v1.Pod) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	podClient := kubeProvider.ClientSet.CoreV1().Pods(data.Namespace)
	_, err := podClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DeleteK8sPod(data v1.Pod) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	podClient := kubeProvider.ClientSet.CoreV1().Pods(data.Namespace)
	err := podClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func filterStatus(pod *v1.Pod) {
	pod.ManagedFields = nil
	pod.ObjectMeta = metav1.ObjectMeta{}
	pod.Spec = v1.PodSpec{}
}
