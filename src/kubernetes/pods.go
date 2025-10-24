package kubernetes

import (
	"bytes"
	"context"
	"text/template"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ServicePodExistsResult struct {
	PodExists bool `json:"podExists"`
}

func PodExists(namespace string, name string) ServicePodExistsResult {
	result := ServicePodExistsResult{}

	clientset := clientProvider.K8sClientSet()
	podClient := clientset.CoreV1().Pods(namespace)
	pod, err := podClient.Get(context.Background(), name, metav1.GetOptions{})
	if err != nil || pod == nil {
		result.PodExists = false
		return result
	}

	result.PodExists = true
	return result
}

func AllPodsOnNode(nodeName string) []v1.Pod {
	result := []v1.Pod{}

	clientset := clientProvider.K8sClientSet()
	podsList, err := clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
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

	clientset := clientProvider.K8sClientSet()
	podsList, err := clientset.CoreV1().Pods(namespaceName).List(context.Background(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
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
	clientset := clientProvider.K8sClientSet()
	getOptions := metav1.GetOptions{}

	podClient := clientset.CoreV1().Pods(namespace)

	pod, err := podClient.Get(context.Background(), name, getOptions)
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
