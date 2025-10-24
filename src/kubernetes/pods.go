package kubernetes

import (
	"bytes"
	"mogenius-k8s-manager/src/store"
	"text/template"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ServicePodExistsResult struct {
	PodExists bool `json:"podExists"`
}

func PodExists(namespace string, name string) ServicePodExistsResult {
	result := ServicePodExistsResult{}

	pod := store.GetPod(namespace, name)
	if pod == nil {
		result.PodExists = false
		return result
	}

	result.PodExists = true
	return result
}

func AllPodsOnNode(nodeName string) []v1.Pod {
	result := []v1.Pod{}

	pods := store.GetPods("*")

	for _, pod := range pods {
		if pod.Spec.NodeName != nodeName {
			continue
		}
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

	pods := store.GetPods(namespaceName)
	for _, pod := range pods {
		if pod.Namespace == "kube-system" {
			continue
		}
		result = append(result, pod)
	}
	return result
}

func PodStatus(namespace string, name string, statusOnly bool) *v1.Pod {
	pod := store.GetPod(namespace, name)
	if pod == nil {
		k8sLogger.Error("PodStatus", "error", "pod not found in store", "namespace", namespace, "name", name)
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
