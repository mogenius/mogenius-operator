package containerenumerator

import (
	"log/slog"
	"mogenius-operator/src/assert"
	"net/url"
	"slices"
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
		for _, container := range item.Status.ContainerStatuses {
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

func (self *PodInfo) ContainersWithFirstPid() map[ContainerId]ProcessId {
	data := map[ContainerId]ProcessId{}
	for containerId, pids := range self.Containers {
		assert.Assert(len(pids) > 0, "podinfo should never contain information about containers with 0 processes")
		data[containerId] = pids[0]
	}
	return data
}

// To detect changes we have to compare a few properties:
//
// - the Namespace and Name have to be equal
// - the list of containers with pids has to have the same container ids
// - the first pid in every container has to be the same
// - it is allowed for other pids to be different (e.g. child processes might have been spawned in the container which we dont care about)
func (self *PodInfo) Equals(other *PodInfo) bool {
	if self.Namespace != other.Namespace {
		return false
	}

	if self.Name != other.Name {
		return false
	}

	containerIds := []ContainerId{}
	for containerId := range self.Containers {
		containerIds = append(containerIds, containerId)
	}
	for containerId := range other.Containers {
		containerIds = append(containerIds, containerId)
	}
	slices.Sort(containerIds)
	containerIds = slices.Compact(containerIds)

	for _, containerId := range containerIds {
		pids, ok := self.Containers[containerId]
		if !ok {
			return false
		}
		assert.Assert(len(pids) > 0, "podinfo should never contain information about containers with 0 processes")

		otherPids, ok := other.Containers[containerId]
		if !ok {
			return false
		}
		assert.Assert(len(otherPids) > 0, "podinfo should never contain information about containers with 0 processes")

		if pids[0] != otherPids[0] {
			return false
		}
	}

	return true
}
