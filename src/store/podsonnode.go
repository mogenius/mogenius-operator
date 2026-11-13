package store

import (
	"mogenius-operator/src/valkeyclient"

	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// podProcessView is the part of a stored pod that the node-metrics
// container enumerator needs to match pods to processes. Decoding into it
// lets encoding/json skip managedFields, the pod spec and every other field
// instead of building full objects for each pod on the node every few
// seconds.
type podProcessView struct {
	Metadata struct {
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		NodeName    string `json:"nodeName"`
		HostNetwork bool   `json:"hostNetwork"`
	} `json:"spec"`
	Status struct {
		PodIP             string       `json:"podIP"`
		StartTime         *metav1.Time `json:"startTime"`
		ContainerStatuses []struct {
			ContainerID string `json:"containerID"`
			Ready       bool   `json:"ready"`
		} `json:"containerStatuses"`
	} `json:"status"`
}

func (self *podProcessView) toPod() coreV1.Pod {
	pod := coreV1.Pod{}
	pod.Namespace = self.Metadata.Namespace
	pod.Name = self.Metadata.Name
	pod.Spec.NodeName = self.Spec.NodeName
	pod.Spec.HostNetwork = self.Spec.HostNetwork
	pod.Status.PodIP = self.Status.PodIP
	pod.Status.StartTime = self.Status.StartTime
	pod.Status.ContainerStatuses = make([]coreV1.ContainerStatus, len(self.Status.ContainerStatuses))
	for idx, status := range self.Status.ContainerStatuses {
		pod.Status.ContainerStatuses[idx].ContainerID = status.ContainerID
		pod.Status.ContainerStatuses[idx].Ready = status.Ready
	}
	return pod
}

// GetPodProcessViewsOnNode returns the pods scheduled on nodeName with only
// these fields set: namespace, name, spec.nodeName, spec.hostNetwork,
// status.podIP, status.startTime and the containerID/ready of each container
// status. It reads the same per-node index as GetPodsOnNode and falls back
// to it (full objects) when the index is empty or unreadable.
func GetPodProcessViewsOnNode(nodeName string) []coreV1.Pod {
	client := valkeyClient.GetValkeyClient()
	members, err := client.Do(valkeyClient.GetContext(), client.B().Smembers().Key(resourceNodeIndexKey(nodeName)).Build()).AsStrSlice()
	if err == nil && len(members) > 0 {
		views, err := valkeyclient.GetObjectsForKeys[podProcessView](valkeyClient, members)
		if err == nil {
			pods := make([]coreV1.Pod, len(views))
			for idx := range views {
				pods[idx] = views[idx].toPod()
			}
			return pods
		}
	}

	return GetPodsOnNode(nodeName)
}
