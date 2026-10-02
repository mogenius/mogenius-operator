package services

import (
	"context"
	"fmt"
	"mogenius-operator/src/debugcontainer"
	"mogenius-operator/src/dtos"
	"mogenius-operator/src/k8sexec"
	mokubernetes "mogenius-operator/src/kubernetes"
	"mogenius-operator/src/store"
	"regexp"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
)

// Pod-addressed file operations: the files/v2 patterns on a running pod's own
// filesystem rather than on a volume. This is what the sandbox toolbox uses
// (Daytona's fs.*). Everything below shares the exec substrate and the
// *Impl functions of filesservice.go; only how the target is found differs.

const (
	// filesFindDefaultMaxResults caps a text search that names no limit.
	filesFindDefaultMaxResults = 1000
	// filesFindTimeout bounds one grep over a sandbox filesystem.
	filesFindTimeout = 60 * time.Second
	// filesFindMaxOutputBytes caps grep's output; past it the result is truncated.
	filesFindMaxOutputBytes = 4 << 20
	// filesReplaceMaxBytes is the largest file replaceImpl rewrites in one go.
	filesReplaceMaxBytes = 16 << 20
)

// grepExitNoMatch is grep's exit code for "nothing matched"; anything above 1 is a failure.
const grepExitNoMatch = 1

// FileMatch is one line a text search found.
type FileMatch struct {
	// File is the path inside the container, as the request addresses paths.
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// FilesFindResult is the outcome of files/v2/find.
type FilesFindResult struct {
	Matches []FileMatch `json:"matches"`
	// Truncated is set when more lines matched than were returned.
	Truncated bool `json:"truncated"`
}

// FileReplaceResult is the per-file outcome of files/v2/replace.
type FileReplaceResult struct {
	File    string `json:"file"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// resolveFileTarget finds the exec substrate for a request: a pod by name, or
// a pod that mounts the named volume.
func resolveFileTarget(request dtos.PvcFileRequestDto) (fileExecTarget, error) {
	if request.AddressesPod() {
		return resolvePodFileTarget(request.Namespace, request.Pod, request.Container)
	}
	return resolvePvcFileTarget(request.Namespace, request.PvcName)
}

// resolvePodFileTarget targets a running pod's container. Images without
// exec tooling (distroless) are served through the ephemeral debug container,
// where the target's root is /proc/1/root — the same fallback the web
// terminal, the SSH gateway and exec-request use.
func resolvePodFileTarget(namespace, podName, container string) (fileExecTarget, error) {
	pod := store.GetPod(namespace, podName)
	if pod == nil {
		return fileExecTarget{}, fmt.Errorf("pod %s/%s not found", namespace, podName)
	}
	if pod.Status.Phase != v1.PodRunning {
		return fileExecTarget{}, fmt.Errorf("pod %s/%s is %s, not Running", namespace, podName, pod.Status.Phase)
	}
	names := make([]string, 0, len(pod.Spec.Containers))
	for _, c := range pod.Spec.Containers {
		names = append(names, c.Name)
	}
	if len(names) == 0 {
		return fileExecTarget{}, fmt.Errorf("pod %s/%s has no containers", namespace, podName)
	}
	if container == "" {
		container = names[0]
	} else if !containsString(names, container) && !debugcontainer.IsDebugContainer(container) {
		return fileExecTarget{}, fmt.Errorf("pod %s/%s has no container %q (available: %s)", namespace, podName, container, strings.Join(names, ", "))
	}

	// The probe is the same `stat` the volume path uses: present → the image
	// has coreutils or busybox and every *Impl works directly.
	if err := cachedProbe(defaultExecProbe)(namespace, podName, container, "/"); err == nil {
		return fileExecTarget{Namespace: namespace, Pod: podName, Container: container, MountRoot: "/"}, nil
	} else {
		serviceLogger.Info("container has no file tooling; using a debug container",
			"namespace", namespace, "pod", podName, "container", container, "error", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), execDebugContainerTimeout)
	defer cancel()
	debugName, err := debugcontainer.Ensure(ctx, clientProvider.K8sClientSet(), namespace, podName, debugcontainer.Options{
		Image:           debugcontainer.ImageFromConfig(config),
		TargetContainer: container,
		Timeout:         execDebugContainerTimeout,
	})
	if err != nil {
		return fileExecTarget{}, fmt.Errorf("container %q ships no file tooling and no debug container could be attached: %w", container, err)
	}
	return fileExecTarget{Namespace: namespace, Pod: podName, Container: debugName, MountRoot: debugContainerRootPath}, nil
}

// ── find (text search) ──────────────────────────────────────────────────────

// FindV2 searches file contents below folder.Path for pattern (grep regular
// expression) and returns matching lines. Binary files are skipped. "Nothing
// matched" is an empty result, not an error; grep's other failures are.
func FindV2(folder dtos.PvcFileRequestDto, pattern string, maxResults int) (FilesFindResult, error) {
	if strings.TrimSpace(pattern) == "" {
		return FilesFindResult{}, fmt.Errorf("pattern cannot be empty")
	}
	if maxResults <= 0 {
		maxResults = filesFindDefaultMaxResults
	}
	target, err := resolveFileTarget(folder)
	if err != nil {
		return FilesFindResult{}, err
	}
	containerPath, err := resolvePath(target.MountRoot, folder.Path)
	if err != nil {
		return FilesFindResult{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), filesFindTimeout)
	defer cancel()
	result, err := k8sexec.Run(ctx, operatorExecClients(), k8sexec.RunRequest{
		Namespace:      target.Namespace,
		Pod:            target.Pod,
		Container:      target.Container,
		Command:        findGrepArgs(containerPath, pattern),
		MaxOutputBytes: filesFindMaxOutputBytes,
	})
	if err != nil {
		return FilesFindResult{}, fmt.Errorf("search in %s: %w", folder.Path, err)
	}
	if result.ExitCode > grepExitNoMatch {
		return FilesFindResult{}, fmt.Errorf("search in %s failed: %s", folder.Path, strings.TrimSpace(result.Stderr))
	}
	matches, truncated := parseGrepOutput(result.Stdout, target.MountRoot, maxResults)
	return FilesFindResult{Matches: matches, Truncated: truncated || result.Truncated}, nil
}

// findGrepArgs is the argv of the search: recursive, line numbers, binaries
// skipped, the pattern after -e so a leading dash cannot become an option,
// and `--` before the path for the same reason.
func findGrepArgs(containerPath, pattern string) []string {
	return []string{"grep", "-rnI", "-e", pattern, "--", containerPath}
}

// grepLine is `file:line:content`. A file name containing ":<digits>:" is
// ambiguous here; grep -Z would settle it but busybox grep lacks it.
var grepLine = regexp.MustCompile(`^(.+?):(\d+):(.*)$`)

// parseGrepOutput turns grep output into matches, trimming the mount root
// off the file paths so they read as the request addressed them.
func parseGrepOutput(output, mountRoot string, maxResults int) ([]FileMatch, bool) {
	matches := make([]FileMatch, 0)
	truncated := false
	for line := range strings.SplitSeq(output, "\n") {
		if line == "" {
			continue
		}
		parts := grepLine.FindStringSubmatch(line)
		if parts == nil {
			continue
		}
		if len(matches) >= maxResults {
			truncated = true
			break
		}
		lineNo, _ := strconv.Atoi(parts[2])
		matches = append(matches, FileMatch{File: requestPathOf(mountRoot, parts[1]), Line: lineNo, Content: parts[3]})
	}
	return matches, truncated
}

// requestPathOf maps an absolute container path back to the path space of the
// request: for a debug container that means stripping /proc/1/root.
func requestPathOf(mountRoot, containerPath string) string {
	root := strings.TrimSuffix(mountRoot, "/")
	if root == "" {
		return containerPath
	}
	if containerPath == root {
		return "/"
	}
	return strings.TrimPrefix(containerPath, root)
}

// ── replace ─────────────────────────────────────────────────────────────────

// ReplaceV2 replaces every occurrence of pattern (a literal, not a regular
// expression) with newValue in each named file and reports per file. A file
// without an occurrence counts as success and is not rewritten. `base` names
// pod or volume; its Path is not used.
func ReplaceV2(base dtos.PvcFileRequestDto, files []string, pattern, newValue string) ([]FileReplaceResult, error) {
	if pattern == "" {
		return nil, fmt.Errorf("pattern cannot be empty")
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no files given")
	}
	target, err := resolveFileTarget(base)
	if err != nil {
		return nil, err
	}

	results := make([]FileReplaceResult, 0, len(files))
	for _, file := range files {
		results = append(results, replaceInFile(target, file, pattern, newValue))
	}
	return results, nil
}

func replaceInFile(target fileExecTarget, file, pattern, newValue string) FileReplaceResult {
	containerPath, err := resolvePath(target.MountRoot, file)
	if err != nil {
		return FileReplaceResult{File: file, Error: err.Error()}
	}
	info, err := infoImpl(target, file)
	if err != nil {
		return FileReplaceResult{File: file, Error: err.Error()}
	}
	if info.Type != "file" {
		return FileReplaceResult{File: file, Error: "not a regular file"}
	}
	if info.SizeInBytes > filesReplaceMaxBytes {
		return FileReplaceResult{File: file, Error: fmt.Sprintf("file is larger than %d bytes", filesReplaceMaxBytes)}
	}

	content, err := mokubernetes.ExecInPod(target.Namespace, target.Pod, target.Container, []string{"cat", "--", containerPath}, nil)
	if err != nil {
		return FileReplaceResult{File: file, Error: fmt.Sprintf("read: %v", err)}
	}
	replaced := strings.ReplaceAll(content, pattern, newValue)
	if replaced == content {
		return FileReplaceResult{File: file, Success: true}
	}
	// `cat > file` keeps the file's mode and owner; the path travels as $1 so
	// nothing in it can become shell syntax.
	if _, err := mokubernetes.ExecInPod(
		target.Namespace, target.Pod, target.Container,
		[]string{"sh", "-c", `cat > "$1"`, "sh", containerPath},
		strings.NewReader(replaced),
	); err != nil {
		return FileReplaceResult{File: file, Error: fmt.Sprintf("write: %v", err)}
	}
	return FileReplaceResult{File: file, Success: true}
}

// operatorExecClients are the clients the file patterns exec with — the
// operator's own, as the volume path does. Impersonation for sandbox file
// access is a follow-up (the exec-request pattern already has it).
func operatorExecClients() *k8sexec.Clients {
	return &k8sexec.Clients{RestConfig: clientProvider.ClientConfig(), Clientset: clientProvider.K8sClientSet()}
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
