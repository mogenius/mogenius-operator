package containerenumerator

import (
	"errors"
	"fmt"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/store"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	v1 "k8s.io/api/core/v1"
)

// Pre-compiled regular expressions for container ID extraction from cgroups
var cgroupRegexes = []*regexp.Regexp{
	regexp.MustCompile(`docker[/-]([a-f0-9]{64})`),
	regexp.MustCompile(`containerd[/-]([a-f0-9]{64})`),
	regexp.MustCompile(`crio-([a-f0-9]{64})`),
	regexp.MustCompile(`cri-containerd-([a-f0-9]{64})`),
	regexp.MustCompile(`kubepods.*/([a-f0-9]{64})$`),
	regexp.MustCompile(`kubepods.*pod([a-f0-9\-]{36})`),
	regexp.MustCompile(`libpod-([a-f0-9]{64})`),
	regexp.MustCompile(`garden-([a-f0-9\-]{36})`),
	regexp.MustCompile(`lxc[/-]([a-zA-Z0-9\-_]+)`),
	// Talos on VMs (linuxkit) uses cgroup v2 paths without the kubepods prefix.
	// The path ends with pod<uid>/<container-id-64hex> where the pod UID is either
	// a 32-char hex string (UUID without hyphens) or a 36-char UUID with hyphens.
	regexp.MustCompile(`pod[a-f0-9\-]{32,36}/([a-f0-9]{64})$`),
}

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

	// cgroupMatches caches the regex result per distinct cgroup file content
	// from the previous enumeration pass. Only the worker goroutine touches it.
	cgroupMatches map[string]cgroupMatch

	// The node's pods as of podsVersion (see store.NodePodsVersion), already
	// filtered. Only the worker goroutine touches these.
	nodePods         []v1.Pod
	nodePodsVersion  string
	nodePodsLoadedAt time.Time

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
	// Use pre-compiled regexes from package variable
	self.cgroupRegexes = cgroupRegexes
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
		containers := self.collectContainers()
		pods := self.generateCurrentPodList(ownNodeName, containers)

		updateTicker := time.NewTicker(5 * time.Second)
		defer updateTicker.Stop()

		for {
			select {
			case <-updateTicker.C:
				containers = self.collectContainers()
				pods = self.generateCurrentPodList(ownNodeName, containers)
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
//
// Every process of a container has the same cgroup file, so the regexes run
// once per distinct content instead of once per PID. The cache is keyed by
// the content, not the PID, so a reused PID can never inherit a stale match;
// it only keeps the contents seen in this pass.
func (self *containerEnumerator) collectContainers() map[ContainerId][]ProcessId {
	data := map[ContainerId][]ProcessId{}
	matches := make(map[string]cgroupMatch, len(self.cgroupMatches))

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
		match, cached := matches[cgroup]
		if !cached {
			match, cached = self.cgroupMatches[cgroup]
		}
		if !cached {
			containerId, err := self.GetContainerIdFromCgroupWithPid(cgroup)
			match = cgroupMatch{containerId: containerId, ok: err == nil}
		}
		matches[cgroup] = match
		if !match.ok {
			continue
		}
		data[match.containerId] = append(data[match.containerId], pidN)
	}
	self.cgroupMatches = matches

	for key := range data {
		slices.Sort(data[key])
	}

	return data
}

func (self *containerEnumerator) readCgroupFile(pid ProcessId) (string, error) {
	filePath := path.Join(self.procPath, strconv.FormatUint(pid, 10), "cgroup")

	file, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf(`failed to read cgroup file Path("%s"): %w`, filePath, err)
	}
	filestring := string(file)

	if strings.TrimSpace(filestring) == "" {
		return "", fmt.Errorf("found empty cgroup file")
	}

	return filestring, nil
}

// cgroupMatch is the cached outcome of GetContainerIdFromCgroupWithPid.
type cgroupMatch struct {
	containerId ContainerId
	ok          bool
}

var ErrorNoMatchFound = errors.New("no match found in cgroup")

func (self *containerEnumerator) GetContainerIdFromCgroupWithPid(cgroupFileData string) (ContainerId, error) {
	// Track the best match (highest position in its line) directly instead
	// of collecting every regex match into a slice first. This runs for
	// every PID on the node on every enumeration pass, so the per-call
	// slice/struct churn added up. Selection semantics are unchanged: all
	// regexes are evaluated (they extract different things, e.g. container
	// id vs. pod uid) and the right-most extraction wins.
	bestPos := -1
	bestData := ""
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
			if idx > bestPos {
				bestPos = idx
				bestData = submatch
			}
		}
	}
	if bestPos < 0 {
		return "", ErrorNoMatchFound
	}

	return bestData, nil
}

func (self *containerEnumerator) generateCurrentPodList(
	nodeName string,
	containers map[ContainerId][]ProcessId,
) []PodInfo {
	// merge and normalize a list of pods with containers and processes
	return NewPodInfoList(self.logger, self.podsOnNode(nodeName), containers)
}

// nodePodsMaxAge bounds how long an unchanged pod list is reused. The
// version counter catches every write; this only covers keys that expire
// without one (e.g. the watcher stopped refreshing the store).
const nodePodsMaxAge = 5 * time.Minute

// podsOnNode returns the pods on nodeName without hostNetwork and outside
// kube-system. It reloads them only when the node's pod version changed (or
// is unknown) or the list is older than nodePodsMaxAge; fetching every pod
// of the node every 5 seconds was the largest Valkey read load of
// node-metrics. The version is read before the pods, so a write racing the
// reload bumps it again and the next pass reloads.
func (self *containerEnumerator) podsOnNode(nodeName string) []v1.Pod {
	version := store.NodePodsVersion(nodeName)
	if version != "" && version == self.nodePodsVersion && time.Since(self.nodePodsLoadedAt) < nodePodsMaxAge {
		return self.nodePods
	}

	// important step: Remove all pods with HostNetwork=true and kube-system pods
	filteredItems := []v1.Pod{}
	for _, pod := range store.GetPodProcessViewsOnNode(nodeName) {
		if !pod.Spec.HostNetwork && pod.Namespace != "kube-system" && pod.Spec.NodeName == nodeName {
			filteredItems = append(filteredItems, pod)
		}
	}

	self.nodePods = filteredItems
	self.nodePodsVersion = version
	self.nodePodsLoadedAt = time.Now()
	return filteredItems
}
