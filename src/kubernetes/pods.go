package kubernetes

import (
	"bytes"
	"context"
	"sort"
	"strings"
	"text/template"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetPod(namespace string, podName string) *v1.Pod {
	provider, err := NewKubeProvider()
	if err != nil {
		return nil
	}

	client := provider.ClientSet.CoreV1().Pods(namespace)
	pod, err := client.Get(context.TODO(), podName, metav1.GetOptions{})
	pod.Kind = "Pod"
	pod.APIVersion = "v1"
	if err != nil {
		k8sLogger.Error("GetPod", "error", err.Error())
		return nil
	}
	return pod
}

func KeplerPod() *v1.Pod {
	provider, err := NewKubeProvider()
	if err != nil {
		k8sLogger.Error("failed to create kube provider", "error", err)
		return nil
	}
	podClient := provider.ClientSet.CoreV1().Pods("")
	labelSelector := "app.kubernetes.io/component=exporter,app.kubernetes.io/name=kepler"
	pods, err := podClient.List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		k8sLogger.Error("failed to list kepler pods", "labelSelector", labelSelector, "error", err.Error())
		return nil
	}
	for _, pod := range pods.Items {
		if pod.GenerateName == "kepler-" {
			return &pod
		}
	}
	return nil
}

type ServicePodExistsResult struct {
	PodExists bool `json:"podExists"`
}

func PodExists(namespace string, name string) ServicePodExistsResult {
	result := ServicePodExistsResult{}

	provider, err := NewKubeProvider()
	if err != nil {
		return result
	}
	podClient := provider.ClientSet.CoreV1().Pods(namespace)
	pod, err := podClient.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil || pod == nil {
		result.PodExists = false
		return result
	}

	result.PodExists = true
	return result
}

func AllPodsOnNode(nodeName string) []v1.Pod {
	result := []v1.Pod{}

	provider, err := NewKubeProvider()
	if err != nil {
		return result
	}

	podsList, err := provider.ClientSet.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		k8sLogger.Error("AllPodsOnNode", "error", err.Error())
		return result
	}
	for _, pod := range podsList.Items {
		pod.Kind = "Pod"
		pod.APIVersion = "v1"
		result = append(result, pod)
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

func AllPodNamesForLabel(namespace string, labelKey string, labelValue string) []string {
	result := []string{}
	allPods := AllPods(namespace)
	for _, pod := range allPods {
		if pod.Labels[labelKey] == labelValue {
			result = append(result, pod.ObjectMeta.Name)
		}
	}
	return result
}

func AllPods(namespaceName string) []v1.Pod {
	result := []v1.Pod{}

	provider, err := NewKubeProvider()
	if err != nil {
		return result
	}
	podsList, err := provider.ClientSet.CoreV1().Pods(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		k8sLogger.Error("AllPods podMetricsList", "error", err.Error())
		return result
	}

	for _, pod := range podsList.Items {
		pod.Kind = "Pod"
		pod.APIVersion = "v1"
		result = append(result, pod)
	}
	return result
}

func PodStatus(namespace string, name string, statusOnly bool) *v1.Pod {
	provider, err := NewKubeProvider()
	if err != nil {
		return nil
	}
	getOptions := metav1.GetOptions{}

	podClient := provider.ClientSet.CoreV1().Pods(namespace)

	pod, err := podClient.Get(context.TODO(), name, getOptions)
	pod.Kind = "Pod"
	pod.APIVersion = "v1"
	if err != nil {
		k8sLogger.Error("PodStatus", "error", err.Error())
		return nil
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

func LastTerminatedStateIfAny(pod *v1.Pod) *v1.ContainerStateTerminated {
	if pod != nil {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			state := containerStatus.LastTerminationState

			if state.Terminated != nil {
				return state.Terminated
			}
		}
	}

	return nil
}

func LastTerminatedStateToString(terminatedState *v1.ContainerStateTerminated) string {
	if terminatedState == nil {
		return "Last State:	   nil\n"
	}

	tpl, err := template.New("state").Parse(
		"Last State:    Terminated\n" +
			"  Reason:      {{.Reason}}\n" +
			"  Message:     {{.Message}}\n" +
			"  Exit Code:   {{.ExitCode}}\n" +
			"  Started:     {{.StartedAt}}\n" +
			"  Finished:    {{.FinishedAt}}\n")
	if err != nil {
		k8sLogger.Error(err.Error())
		return ""
	}

	buf := bytes.Buffer{}
	err = tpl.Execute(&buf, terminatedState)
	if err != nil {
		k8sLogger.Error(err.Error())
		return ""
	}

	return buf.String()
}

func ServicePodStatus(namespace string, serviceName string) []v1.Pod {
	result := []v1.Pod{}
	provider, err := NewKubeProvider()
	if err != nil {
		return result
	}

	podClient := provider.ClientSet.CoreV1().Pods(namespace)

	pods, err := podClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		k8sLogger.Error("ServicePodStatus", "error", err.Error())
		return result
	}

	for _, pod := range pods.Items {
		if strings.Contains(pod.Name, serviceName) {
			pod.ManagedFields = nil
			pod.Spec = v1.PodSpec{}
			pod.Kind = "Pod"
			pod.APIVersion = "v1"
			result = append(result, pod)
		}
	}

	return result
}

func PodIdsFor(namespace string, serviceId *string) []string {
	result := []string{}

	provider, err := NewKubeProviderMetrics()
	if provider == nil || err != nil {
		k8sLogger.Error(err.Error())
		return result
	}

	podMetricsList, err := provider.ClientSet.MetricsV1beta1().PodMetricses(namespace).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		k8sLogger.Error("PodIdsForServiceId podMetricsList", "error", err.Error())
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
