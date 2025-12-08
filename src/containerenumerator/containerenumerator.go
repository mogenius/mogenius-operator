package containerenumerator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/k8sclient"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ContainerEnumerator interface {
	GetPodsWithContainerIds() []PodInfo
	GetProcessesWithContainerIds() map[ContainerId][]ProcessId
	GetContainerIdFromCgroupWithPid(cgroupFileData string) (ContainerId, error)
}

type ContainerId = string
type ProcessId = uint64

type containerEnumerator struct {
	logger   *slog.Logger
	config   config.ConfigModule
	procPath string

	running atomic.Bool

	// Regular expressions to match and capture Container IDs
	cgroupRegexes []*regexp.Regexp

	getProcessesWithContainerIdsRx chan struct{}
	getProcessesWithContainerIdsTx chan map[ContainerId][]ProcessId

	getPodsWithContainerIdsRx chan struct{}
	getPodsWithContainerIdsTx chan []PodInfo

	clientProvider k8sclient.K8sClientProvider
}

func NewContainerEnumerator(
	logger *slog.Logger,
	config config.ConfigModule,
	clientProvider k8sclient.K8sClientProvider,
) ContainerEnumerator {
	self := &containerEnumerator{}

	self.logger = logger
	self.config = config
	self.procPath = config.Get("MO_HOST_PROC_PATH")
	self.clientProvider = clientProvider
	self.cgroupRegexes = []*regexp.Regexp{
		regexp.MustCompile(`cri-containerd-([0-9a-fA-F]+)\.scope`),
		regexp.MustCompile(`crio-([0-9a-fA-F]+)\.scope`),
		regexp.MustCompile(`/docker/([0-9a-fA-F]+)\.scope`),
		regexp.MustCompile(`docker-([0-9a-fA-F]+)\.scope`),
		regexp.MustCompile(`kubepods[^/]*/pod[^/]+/([0-9a-fA-F]+)`),
		regexp.MustCompile(`containerd:([0-9a-fA-F]+)`),
		regexp.MustCompile(`burstable/pod[^/]+/([0-9a-fA-F]+)`),
		regexp.MustCompile(`/pod[^/]+/([0-9a-fA-F]+)`),
	}
	self.running = atomic.Bool{}

	self.getProcessesWithContainerIdsRx = make(chan struct{})
	self.getProcessesWithContainerIdsTx = make(chan map[ContainerId][]ProcessId)
	self.getPodsWithContainerIdsRx = make(chan struct{})
	self.getPodsWithContainerIdsTx = make(chan []PodInfo)

	return self
}

func (self *containerEnumerator) startWorker() {
	wasRunning := self.running.Swap(true)
	if wasRunning {
		return
	}

	go func() {
		ownNodeName := self.config.Get("OWN_NODE_NAME")
		assert.Assert(ownNodeName != "", "OWN_NODE_NAME has to be defined and non-empty", ownNodeName)
		fieldSelector := fmt.Sprintf("metadata.namespace!=kube-system,spec.nodeName=%s", ownNodeName)

		containers := self.collectContainers()
		pods := self.generateCurrentPodList(fieldSelector, containers)

		updateTicker := time.NewTicker(5 * time.Second)
		defer updateTicker.Stop()

		for {
			select {
			case <-updateTicker.C:
				containers = self.collectContainers()
				pods = self.generateCurrentPodList(fieldSelector, containers)
			case <-self.getProcessesWithContainerIdsRx:
				self.getProcessesWithContainerIdsTx <- containers
			case <-self.getPodsWithContainerIdsRx:
				self.getPodsWithContainerIdsTx <- pods
			}
		}
	}()
}

func (self *containerEnumerator) GetProcessesWithContainerIds() map[ContainerId][]ProcessId {
	self.startWorker()

	self.getProcessesWithContainerIdsRx <- struct{}{}
	containers := <-self.getProcessesWithContainerIdsTx

	return containers
}

func (self *containerEnumerator) GetPodsWithContainerIds() []PodInfo {
	self.startWorker()

	self.getPodsWithContainerIdsRx <- struct{}{}
	pods := <-self.getPodsWithContainerIdsTx

	return pods
}

//  1. Scan **all** processes running on the system.
//  2. If they are assigned to a cgroup the cgroup is being parsed.
//  3. Parsing happens using a list of regexes. If a known container engine is
//     found the regex extracts the container id.
//  4. The returned map consists of all found container ids as a key to all process ids running within that container.
func (self *containerEnumerator) collectContainers() map[ContainerId][]ProcessId {
	data := map[ContainerId][]ProcessId{}

	// to start get a list of all system processes
	files, err := os.ReadDir(self.procPath)
	assert.Assert(err == nil, "procPath has to be readable", self.procPath, err)

	for _, file := range files {
		// we are only interested in the contents of /proc/$pid/cgroup for PIDs which are numeric
		if !file.IsDir() {
			continue
		}
		pid := file.Name()
		var pidN ProcessId
		pidN, err := strconv.ParseUint(pid, 10, 64)
		if err != nil {
			continue
		}
		cgroup, err := self.readCgroupFile(pidN)
		if err != nil {
			continue
		}
		// check if the pid is actually from a container and get its container id if it is
		containerId, err := self.GetContainerIdFromCgroupWithPid(cgroup)
		if err != nil {
			continue
		}
		data[containerId] = append(data[containerId], pidN)
	}

	for key := range data {
		slices.Sort(data[key])
	}

	return data
}

func (self *containerEnumerator) readCgroupFile(pid ProcessId) (string, error) {
	filePath := path.Join(self.procPath, strconv.FormatUint(pid, 10), "cgroup")

	file, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf(`failed to read cgroup file Path("%s"): %v`, filePath, err)
	}
	filestring := string(file)

	if strings.TrimSpace(filestring) == "" {
		return "", fmt.Errorf("found empty cgroup file")
	}

	return filestring, nil
}

// var ErrorNoMatchFound error = fmt.Errorf("failed to find valid container id")

var (
	ErrorNoMatchFound = errors.New("no match found in cgroup")
	// Alternativ, wenn Sie den alten Namen behalten wollen:
	NoMatchFound = errors.New("no match found in cgroup")
)

func (self *containerEnumerator) GetContainerIdFromCgroupWithPid(cgroupFileData string) (ContainerId, error) {
	type PatternMatch struct {
		pos  int
		data string
	}
	allMatches := []PatternMatch{}
	for line := range strings.SplitSeq(cgroupFileData, "\n") {
		for _, regex := range self.cgroupRegexes {
			matches := regex.FindAllStringSubmatch(line, -1)
			if len(matches) == 0 {
				continue
			}

			submatches := matches[len(matches)-1]
			assert.Assert(submatches != nil)
			assert.Assert(len(submatches) > 1)
			submatch := submatches[len(submatches)-1]
			idx := strings.LastIndex(line, submatch)
			assert.Assert(idx != -1)
			match := PatternMatch{pos: idx, data: submatch}
			allMatches = append(allMatches, match)
		}
	}
	if len(allMatches) == 0 {
		return "", ErrorNoMatchFound
	}

	result := &allMatches[0]
	for _, match := range allMatches {
		if match.pos > result.pos {
			result = &match
		}
	}

	return result.data, nil
}

func (self *containerEnumerator) generateCurrentPodList(
	fieldSelector string,
	containers map[ContainerId][]ProcessId,
) []PodInfo {
	// query for all pods on current node
	listOpts := metav1.ListOptions{FieldSelector: fieldSelector}
	newPodList, err := self.clientProvider.K8sClientSet().CoreV1().Pods("").List(context.Background(), listOpts)
	if err != nil {
		self.logger.Error("failed to list pods", "listOpts", listOpts, "error", err)
		return []PodInfo{}
	}

	// important step: Remove all pods with HostNetwork=true
	filteredItems := []v1.Pod{}
	for idx := range len(newPodList.Items) {
		pod := newPodList.Items[idx]
		if !pod.Spec.HostNetwork {
			filteredItems = append(filteredItems, pod)
		}
	}

	// merge and normalize a list of pods with containers and processes
	podInfoList := NewPodInfoList(self.logger, filteredItems, containers)

	return podInfoList
}
