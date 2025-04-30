package networkmonitor

import (
	"fmt"
	"log"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

type ContainerNetworkEnumerator interface {
	List(procPath string) map[ContainerId]InterfaceDescription
	FindProcessesWithContainerIds(procPath string) map[ContainerId][]ProcessId
	GetContainerIdFromCgroupWithPid(cgroupFileData string) (ContainerId, error)
	RequestInterfaceDescription(procPath string) ([]IpLinkInfo, error)
}

type InterfaceDescription struct {
	LinkInfo []IpLinkInfo
	Pids     []ProcessId
}

type IpLinkInfo struct {
	Ifindex     int      `json:"ifindex,omitempty"`
	Ifname      string   `json:"ifname"`
	Flags       []string `json:"flags,omitempty"`
	Mtu         int      `json:"mtu,omitempty"`
	Qdisc       string   `json:"qdisc,omitempty"`
	Operstate   string   `json:"operstate,omitempty"`
	Linkmode    string   `json:"linkmode,omitempty"`
	Group       string   `json:"group,omitempty"`
	Txqlen      int      `json:"txqlen,omitempty"`
	LinkType    string   `json:"link_type"`
	Address     string   `json:"address,omitempty"`
	Broadcast   string   `json:"broadcast,omitempty"`
	LinkIndex   int      `json:"link_index,omitempty"`
	Master      string   `json:"master,omitempty"`
	LinkNetnsid int      `json:"link_netnsid,omitempty"`
}

func (self *IpLinkInfo) IsUp() bool {
	return slices.Contains(self.Flags, "UP")
}

func (self *IpLinkInfo) IsLoopback() bool {
	return slices.Contains(self.Flags, "LOOPBACK")
}

func (self *IpLinkInfo) ToJson() string {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary

	data, err := json.Marshal(&self)
	assert.Assert(err == nil, err)

	return string(data)
}

func (self *IpLinkInfo) ToJsonPretty() string {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary

	data, err := json.MarshalIndent(&self, "", "  ")
	assert.Assert(err == nil, err)

	return string(data)
}

type containerNetworkEnumerator struct {
	logger *slog.Logger
	// Regular expressions to match and capture Container IDs
	cgroupRegexes []*regexp.Regexp
}

func NewContainerNetworkEnumerator(logger *slog.Logger) ContainerNetworkEnumerator {
	self := &containerNetworkEnumerator{}

	self.logger = logger
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

	return self
}

func (self *containerNetworkEnumerator) List(procPath string) map[ContainerId]InterfaceDescription {
	processesWithContainerIds := self.FindProcessesWithContainerIds(procPath)
	networkInterfaceList := map[ContainerId]InterfaceDescription{}
	for containerId, pids := range processesWithContainerIds {
		ifDesc, ok := networkInterfaceList[containerId]
		if !ok {
			ifDesc = InterfaceDescription{}
			ifDesc.LinkInfo = []IpLinkInfo{}
			ifDesc.Pids = []ProcessId{}
		}
		ifDesc = InterfaceDescription{}
		assert.Assert(len(pids) > 0, "no container should exist without running processes")
		assert.Assert(pids[0] != 0, "the first pid in this list should be defined")
		interfaces, err := self.requestNamespacedInterfaceDescription(procPath, pids[0])
		if err != nil {
			self.logger.Error("failed to request network interfaces", "procPath", procPath, "pid", pids[0], "error", err)
			break
		}
		ifDesc.LinkInfo = interfaces
		ifDesc.Pids = pids
		networkInterfaceList[containerId] = ifDesc
	}
	return networkInterfaceList
}

//  1. Scan **all** processes running on the system.
//  2. If they are assigned to a cgroup the cgroup is being parsed.
//  3. Parsing happens using a list of regexes. If a known container engine is
//     found the regex extracts the container id.
//  4. The returned map consists of all found container ids as a key to all process ids running within that container.
func (self *containerNetworkEnumerator) FindProcessesWithContainerIds(procPath string) map[ContainerId][]ProcessId {
	data := map[ContainerId][]ProcessId{}

	// to start get a list of all system processes
	files, err := os.ReadDir(procPath)
	if err != nil {
		log.Fatal(err)
	}

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
		cgroup, err := self.readCgroupFile(procPath, pidN)
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

func (self *containerNetworkEnumerator) readCgroupFile(procPath string, pid ProcessId) (string, error) {
	filePath := path.Join(procPath, strconv.FormatUint(pid, 10), "cgroup")

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

var NoMatchFound error = fmt.Errorf("failed to find valid container id")

func (self *containerNetworkEnumerator) GetContainerIdFromCgroupWithPid(cgroupFileData string) (ContainerId, error) {
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
		return "", NoMatchFound
	}

	result := &allMatches[0]
	for _, match := range allMatches {
		if match.pos > result.pos {
			result = &match
		}
	}

	return result.data, nil
}

func (self *containerNetworkEnumerator) RequestInterfaceDescription(procPath string) ([]IpLinkInfo, error) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return []IpLinkInfo{}, fmt.Errorf("os not supported: %s", runtime.GOOS)
	}
	var json = jsoniter.ConfigCompatibleWithStandardLibrary

	cmd := exec.Command(
		"ip",
		"--json",
		"link",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		self.logger.Error("failed to execute command", "hostProc", procPath, "output", strings.TrimSpace(string(out)), "error", err)
		return []IpLinkInfo{}, err
	}

	var ipOutput []IpLinkInfo
	err = json.Unmarshal(out, &ipOutput)
	if err != nil {
		self.logger.Error("failed to unmarshal output of nsenter with ip link", "output", string(out), "error", err)
		return []IpLinkInfo{}, err
	}

	return ipOutput, nil
}

func (self *containerNetworkEnumerator) requestNamespacedInterfaceDescription(procPath string, pid ProcessId) ([]IpLinkInfo, error) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return []IpLinkInfo{}, fmt.Errorf("os not supported: %s", runtime.GOOS)
	}
	var json = jsoniter.ConfigCompatibleWithStandardLibrary

	cmd := exec.Command(
		"nsenter",
		"--target="+strconv.FormatUint(pid, 10),
		"--net="+procPath+"/"+strconv.FormatUint(pid, 10)+"/ns/net",
		"ip",
		"--json",
		"link",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		self.logger.Error("failed to execute command", "pid", pid, "hostProc", procPath, "output", strings.TrimSpace(string(out)), "error", err)
		return []IpLinkInfo{}, err
	}

	var ipOutput []IpLinkInfo
	err = json.Unmarshal(out, &ipOutput)
	if err != nil {
		self.logger.Error("failed to unmarshal output of nsenter with ip link", "output", string(out), "error", err)
		return []IpLinkInfo{}, err
	}

	return ipOutput, nil
}
