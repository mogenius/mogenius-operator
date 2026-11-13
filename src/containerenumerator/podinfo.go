package containerenumerator

import (
	"log/slog"
	"mogenius-operator/src/assert"
	"net/url"
	"slices"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
)

type PodInfoIdentifier = string

type PodInfo struct {
	Namespace  string
	Name       string
	PodIp      string
	StartTime  string
	Containers map[ContainerId][]ProcessId
}

func NewPodInfoList(logger *slog.Logger, k8sPods []v1.Pod, nodecontainers map[ContainerId][]ProcessId) []PodInfo {
	podInfoList := []PodInfo{}
	for _, item := range k8sPods {
		if item.Status.StartTime == nil {
			// startup hasnt completed yet
			continue
		}
		podInfo := PodInfo{}
		podInfo.Namespace = item.Namespace
		podInfo.Name = item.Name
		podInfo.PodIp = item.Status.PodIP
		podInfo.StartTime = item.Status.StartTime.Format(time.RFC3339)
		podInfo.Containers = map[ContainerId][]ProcessId{}
		// Native sidecars (init containers with restartPolicy Always, e.g.
		// docker-in-docker or a service mesh proxy) only appear in
		// initContainerStatuses. Regular init containers have exited and
		// have no processes left, so the process lookup below drops them.
		statuses := make([]v1.ContainerStatus, 0, len(item.Status.InitContainerStatuses)+len(item.Status.ContainerStatuses))
		statuses = append(statuses, item.Status.InitContainerStatuses...)
		statuses = append(statuses, item.Status.ContainerStatuses...)
		for _, container := range statuses {
			if !container.Ready {
				// dont bother with containers which havent finished startup
				continue
			}
			parsedUrl, err := url.Parse(container.ContainerID)
			if err != nil {
				// this should probably never be the case and could be an assertion instead
				logger.Warn("failed to parse container id", "container", container.ContainerID)
				continue
			}
			containerId := parsedUrl.Host

			processList, ok := nodecontainers[containerId]
			if !ok {
				// this might happen for very short periods of time when container.Ready gets reached after /proc was parsed for containers
				// in practice this means its eventually consistent and completely fine
				continue
			}
			if len(processList) == 0 {
				// dont bother with only partially created containers until processes are spawned in them
				continue
			}
			podInfo.Containers[containerId] = processList
		}
		if len(podInfo.Containers) == 0 {
			// ignore pods which dont have any container with running processes
			continue
		}
		podInfoList = append(podInfoList, podInfo)
	}

	return podInfoList
}

func (self *PodInfo) NamespaceAndName() PodInfoIdentifier {
	assert.Assert(self.Namespace != "", "encountered empty namespace")
	assert.Assert(self.Name != "", "encountered empty name")
	return self.Namespace + "/" + self.Name
}

// ContainerFirstPid pairs a container with the first process running in it.
type ContainerFirstPid struct {
	ContainerId ContainerId
	Pid         ProcessId
}

// SortedContainersWithFirstPid returns every container with its first pid,
// ordered by container id. All containers of a pod share the network
// namespace of the pod sandbox, so traffic collectors attach to one of them;
// the stable order keeps that choice the same across calls.
func (self *PodInfo) SortedContainersWithFirstPid() []ContainerFirstPid {
	data := make([]ContainerFirstPid, 0, len(self.Containers))
	for containerId, pids := range self.Containers {
		assert.Assert(len(pids) > 0, "podinfo should never contain information about containers with 0 processes")
		data = append(data, ContainerFirstPid{ContainerId: containerId, Pid: pids[0]})
	}
	slices.SortFunc(data, func(a, b ContainerFirstPid) int {
		return strings.Compare(a.ContainerId, b.ContainerId)
	})
	return data
}

// PodProcesses flattens the containers of the given pods into one map. The
// cpu and ram monitors read /proc/$pid/stat and statm only for these: they
// report nothing else, and every other container process on the node (pod
// sandboxes, hostNetwork and kube-system pods) is often unconfined. Reading
// its stat from an AppArmor-confined container is a ptrace read the kernel
// audits as DENIED for every process and every second, which fills
// kern.log, syslog and the journal of the node.
func PodProcesses(pods []PodInfo) map[ContainerId][]ProcessId {
	data := map[ContainerId][]ProcessId{}
	for _, pod := range pods {
		for containerId, pids := range pod.Containers {
			data[containerId] = pids
		}
	}
	return data
}

// To detect changes we have to compare a few properties:
//
// - the Namespace and Name have to be equal
// - the list of containers with pids has to have the same container ids
//
// PIDs inside a container are intentionally NOT compared: a container's
// network namespace is stable for the container's lifetime, and once snoopy
// is attached via an open ns fd the original PID may die without affecting
// the attachment. Re-registering on every PID change would tear down a
// working attachment and race against short-lived child processes (which
// fail to attach with EACCES on /proc/$pid/ns/net once they exit).
func (self *PodInfo) Equals(other *PodInfo) bool {
	if self.Namespace != other.Namespace {
		return false
	}

	if self.Name != other.Name {
		return false
	}

	if len(self.Containers) != len(other.Containers) {
		return false
	}

	for containerId := range self.Containers {
		if _, ok := other.Containers[containerId]; !ok {
			return false
		}
	}

	return true
}
