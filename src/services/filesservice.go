package services

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mogenius-operator/src/dtos"
	mokubernetes "mogenius-operator/src/kubernetes"
	"mogenius-operator/src/utils"
	"net/http"
	"net/textproto"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// fileExecTarget is the resolved exec substrate for one file operation: the
// container to exec in and the mount root all request paths resolve against.
type fileExecTarget struct {
	Namespace string
	Pod       string
	Container string
	MountRoot string
}

// resolvePvcFileTarget resolves the v2 exec target: any running pod that
// mounts the PVC without subPath, chosen by ResolvePvcTarget.
func resolvePvcFileTarget(namespace, pvcName string) (fileExecTarget, error) {
	target, err := ResolvePvcTarget(namespace, pvcName)
	if err != nil {
		return fileExecTarget{}, err
	}
	return fileExecTarget{
		Namespace: target.Namespace,
		Pod:       target.PodName,
		Container: target.ContainerName,
		MountRoot: target.MountPath,
	}, nil
}

// ── entry points (files/v2/* patterns, any mounted PVC or running pod) ────────

// List lists the entries below folder.Path; maxDepth 1 (or less) is the
// folder itself, larger values descend that many levels.
func List(folder dtos.PvcFileRequestDto, maxDepth int) ([]dtos.PersistentFileDto, error) {
	target, err := resolveFileTarget(folder)
	if err != nil {
		return nil, err
	}
	return listImpl(target, folder.Path, maxDepth)
}

// Search finds entries by name below folder.Path; glob switches from substring to shell-glob matching.
func Search(folder dtos.PvcFileRequestDto, query string, maxResults int, glob bool) (FilesSearchResult, error) {
	target, err := resolveFileTarget(folder)
	if err != nil {
		return FilesSearchResult{}, err
	}
	return searchImpl(target, folder.Path, query, maxResults, glob)
}

func Info(r dtos.PvcFileRequestDto) (dtos.PersistentFileDto, error) {
	target, err := resolveFileTarget(r)
	if err != nil {
		return dtos.PersistentFileDto{}, err
	}
	return infoImpl(target, r.Path)
}

func Download(pfile dtos.PvcFileRequestDto, postTo string) (FilesDownloadResponse, error) {
	target, err := resolveFileTarget(pfile)
	if err != nil {
		return FilesDownloadResponse{Error: err.Error()}, err
	}
	return downloadImpl(target, pfile.Path, postTo)
}

func Uploaded(tempZipFileSrc string, fileReq FilesUploadRequest) error {
	target, err := resolveFileTarget(fileReq.File)
	if err != nil {
		return fmt.Errorf("error verifying file %s: %w", fileReq.File.Path, err)
	}
	return uploadedImpl(target, tempZipFileSrc, fileReq.File.Path, fileReq.SizeInBytes)
}

// CreateFolder creates the folder and its parents; mode (octal, e.g. "755")
// is applied when given.
func CreateFolder(folder dtos.PvcFileRequestDto, mode string) error {
	target, err := resolveFileTarget(folder)
	if err != nil {
		return err
	}
	return createFolderImpl(target, folder.Path, mode)
}

// Rename renames within the folder (newName) or moves to another path
// (newPath, resolved like every request path). Exactly one of the two.
func Rename(file dtos.PvcFileRequestDto, newName string, newPath string) error {
	target, err := resolveFileTarget(file)
	if err != nil {
		return err
	}
	return renameImpl(target, file.Path, newName, newPath)
}

func Chown(file dtos.PvcFileRequestDto, uidString string, gidString string) error {
	target, err := resolveFileTarget(file)
	if err != nil {
		return err
	}
	return chownImpl(target, file.Path, uidString, gidString)
}

func Chmod(file dtos.PvcFileRequestDto, mode string) error {
	target, err := resolveFileTarget(file)
	if err != nil {
		return err
	}
	return chmodImpl(target, file.Path, mode)
}

// Delete removes the path. recursive=false removes only a file or an empty
// folder, as Daytona's deleteFile does by default.
func Delete(file dtos.PvcFileRequestDto, recursive bool) error {
	target, err := resolveFileTarget(file)
	if err != nil {
		return err
	}
	return deleteImpl(target, file.Path, recursive)
}

// ── target-based implementations ──────────────────────────────────────────────

// statFormat is the tab-separated stat -c format listImpl/infoImpl/searchImpl
// share; parseStatLine reads it back.
const statFormat = "%n\t%F\t%s\t%u\t%g\t%a\t%Y"

// statRecordScript is the `sh -c` body find hands its matches to: one
// statFormat record per path, each closed by a NUL byte. A newline inside a
// file name therefore stays inside its record instead of starting a new one.
// POSIX sh, busybox included: `printf '\0'` writes the NUL.
var statRecordScript = "for f; do stat -c '" + statFormat + "' -- \"$f\"; printf '\\0'; done"

// statExecArgs is the find tail that prints every match as a NUL-closed
// statFormat record; splitStatRecords reads the output back.
func statExecArgs() []string {
	return []string{"-exec", "sh", "-c", statRecordScript, "sh", "{}", "+"}
}

// splitStatRecords splits find+stat output into statFormat records. Records
// are NUL-separated (statExecArgs); plain newline-separated output, as a
// single `stat` prints it, is accepted too.
func splitStatRecords(output string) []string {
	separator := "\x00"
	if !strings.Contains(output, separator) {
		separator = "\n"
	}
	var records []string
	for record := range strings.SplitSeq(output, separator) {
		record = strings.TrimSuffix(record, "\n")
		if strings.TrimSpace(record) == "" {
			continue
		}
		records = append(records, record)
	}
	return records
}

// filesSearchDefaultMaxResults caps a files/v2/search answer when the caller
// sends no limit.
const filesSearchDefaultMaxResults = 500

// FilesSearchResult is the wire shape of files/v2/search.
type FilesSearchResult struct {
	Items     []dtos.PersistentFileDto `json:"items"`
	Truncated bool                     `json:"truncated"`
}

// searchFindArgs builds the find invocation of a free-text name search below
// containerPath. The pattern travels as one argv element - there is no shell,
// so the query cannot inject commands; glob characters in it act as wildcards.
// lost+found is skipped like the listing does.
// searchFindArgs builds the name search: a case-insensitive substring match
// by default, or the query as a shell glob (`*.py`, `data-??.csv`) when glob
// is set — Daytona's searchFiles passes globs.
func searchFindArgs(containerPath, query string, glob bool) []string {
	nameTest := []string{"-iname", "*" + query + "*"}
	if glob {
		nameTest = []string{"-name", query}
	}
	args := []string{
		"find", containerPath,
		"-mindepth", "1",
		"!", "-name", "lost+found",
		"!", "-path", "*/lost+found/*",
	}
	args = append(args, nameTest...)
	return append(args, statExecArgs()...)
}

// searchImpl runs one find over the subtree below requestPath and returns the
// matching entries with paths relative to that subtree. Results are capped at
// maxResults; Truncated tells the caller the list is incomplete.
func searchImpl(target fileExecTarget, requestPath, query string, maxResults int, glob bool) (FilesSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return FilesSearchResult{}, fmt.Errorf("query cannot be empty")
	}
	if maxResults <= 0 {
		maxResults = filesSearchDefaultMaxResults
	}

	containerPath, err := resolvePath(target.MountRoot, requestPath)
	if err != nil {
		return FilesSearchResult{}, err
	}

	output, err := mokubernetes.ExecInPod(
		target.Namespace, target.Pod, target.Container,
		searchFindArgs(containerPath, query, glob),
		nil,
	)
	// find exits non-zero when a subfolder is unreadable but still prints
	// every match it reached - keep those instead of failing the search
	if err != nil && strings.TrimSpace(output) == "" {
		return FilesSearchResult{}, err
	}

	result := FilesSearchResult{Items: []dtos.PersistentFileDto{}}
	for _, line := range splitStatRecords(output) {
		if !strings.Contains(line, "\t") {
			continue
		}
		if len(result.Items) >= maxResults {
			result.Truncated = true
			break
		}
		item, parseErr := parseStatLine(containerPath, line)
		if parseErr != nil {
			serviceLogger.Warn("Search: parseStatLine error", "line", line, "error", parseErr)
			continue
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func listImpl(target fileExecTarget, requestPath string, maxDepth int) ([]dtos.PersistentFileDto, error) {
	containerPath, err := resolvePath(target.MountRoot, requestPath)
	if err != nil {
		return nil, err
	}
	if maxDepth < 1 {
		maxDepth = 1
	}

	args := []string{"find", containerPath, "-maxdepth", strconv.Itoa(maxDepth), "-mindepth", "1"}
	output, err := mokubernetes.ExecInPod(
		target.Namespace, target.Pod, target.Container,
		append(args, statExecArgs()...),
		nil,
	)
	if err != nil {
		return nil, err
	}

	var result []dtos.PersistentFileDto
	for _, line := range splitStatRecords(output) {
		item, parseErr := parseStatLine(containerPath, line)
		if parseErr != nil {
			serviceLogger.Warn("List: parseStatLine error", "line", line, "error", parseErr)
			continue
		}
		result = append(result, item)
	}
	if result == nil {
		result = []dtos.PersistentFileDto{}
	}
	return result, nil
}

func infoImpl(target fileExecTarget, requestPath string) (dtos.PersistentFileDto, error) {
	containerPath, err := resolvePath(target.MountRoot, requestPath)
	if err != nil {
		return dtos.PersistentFileDto{}, err
	}

	output, err := mokubernetes.ExecInPod(
		target.Namespace, target.Pod, target.Container,
		[]string{"stat", "-c", statFormat, containerPath},
		nil,
	)
	if err != nil {
		return dtos.PersistentFileDto{}, err
	}
	line := strings.TrimSpace(output)
	info, err := parseStatLine(target.MountRoot, line)
	if err != nil {
		return dtos.PersistentFileDto{}, err
	}
	// only regular files are sniffed: `head` on a fifo or a device would block
	// the exec, and the exec has no deadline
	if isRegularFileStatLine(line) {
		info.MimeType, info.ContentType = detectContentType(target, containerPath)
	}
	return info, nil
}

// isRegularFileStatLine reports whether the %F field of a statFormat line is
// "regular file" or "regular empty file".
func isRegularFileStatLine(line string) bool {
	parts := strings.Split(line, "\t")
	return len(parts) > 1 && strings.HasPrefix(parts[1], "regular")
}

// mimeSniffBytes is how much of a file detectContentType reads; it is the
// window http.DetectContentType looks at, more would be wasted transfer.
const mimeSniffBytes = 512

// detectContentType sniffs the media type of a regular file from its first
// bytes, the same way a browser would. It answers ("text/plain",
// "text/plain; charset=utf-8") for an extension-less text file and
// ("application/octet-stream", ...) for binary data. Best effort: a failing
// exec yields empty strings, callers fall back to extension-based guesses.
// Only used on single-file paths (info, download), never per list entry -
// that would be one exec per file.
func detectContentType(target fileExecTarget, containerPath string) (mimeType string, contentType string) {
	head, err := mokubernetes.ExecInPod(
		target.Namespace, target.Pod, target.Container,
		[]string{"head", "-c", strconv.Itoa(mimeSniffBytes), containerPath},
		nil,
	)
	if err != nil {
		serviceLogger.Debug("content type sniff failed", "path", containerPath, "error", err)
		return "", ""
	}
	contentType = http.DetectContentType([]byte(head))
	mimeType, _, err = mime.ParseMediaType(contentType)
	if err != nil {
		mimeType = contentType
	}
	return mimeType, contentType
}

// FilesDownloadStreamInfo is the datagram answer of files/v2/download-stream:
// what the browser needs for its headers, sent before the first byte flows.
// The resolved exec target rides along for the stream socket's headers but
// stays out of the wire format.
type FilesDownloadStreamInfo struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	// -1 for a directory: its tar.gz size is only known at the end.
	SizeInBytes int64 `json:"sizeInBytes"`
	IsDirectory bool  `json:"isDirectory"`

	Namespace string `json:"-"`
	Pod       string `json:"-"`
	Container string `json:"-"`
}

// DownloadStreamInfo resolves the target and stats the path. Every error a
// download can fail with before its first byte (pod gone, path missing, no
// exec tooling) surfaces here, so the API can answer the browser with a
// status code instead of a broken stream.
func DownloadStreamInfo(pfile dtos.PvcFileRequestDto) (FilesDownloadStreamInfo, error) {
	target, err := resolveFileTarget(pfile)
	if err != nil {
		return FilesDownloadStreamInfo{}, err
	}
	info, err := infoImpl(target, pfile.Path)
	if err != nil {
		return FilesDownloadStreamInfo{}, err
	}
	name, contentType := downloadNameAndType(info)
	result := FilesDownloadStreamInfo{
		Name:        name,
		ContentType: contentType,
		SizeInBytes: info.SizeInBytes,
		IsDirectory: info.Type == "directory",
		Namespace:   target.Namespace,
		Pod:         target.Pod,
		Container:   target.Container,
	}
	if result.IsDirectory {
		result.SizeInBytes = -1
	}
	return result, nil
}

// downloadNameAndType is the file name and content type a download carries:
// folders go out as <name>.tar.gz, files keep their sniffed type so the UI
// can preview an extension-less text file without asking.
func downloadNameAndType(info dtos.PersistentFileDto) (string, string) {
	filename := info.Name
	contentType := info.ContentType
	if info.Type == "directory" {
		filename = info.Name + ".tar.gz"
		contentType = "application/gzip"
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return filename, contentType
}

func downloadImpl(target fileExecTarget, requestPath string, postTo string) (FilesDownloadResponse, error) {
	result := FilesDownloadResponse{}

	containerPath, err := resolvePath(target.MountRoot, requestPath)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	info, err := infoImpl(target, requestPath)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	buf := new(bytes.Buffer)
	multiPartWriter := multipart.NewWriter(buf)

	// CreateFormFile would stamp every part application/octet-stream; the
	// platform passes the part's Content-Type straight through to the browser.
	filename, contentType := downloadNameAndType(info)
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, headerFilename(filename)))
	partHeader.Set("Content-Type", contentType)

	w, err := multiPartWriter.CreatePart(partHeader)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	if info.Type == "directory" {
		err = mokubernetes.ExecInPodToWriter(
			target.Namespace, target.Pod, target.Container,
			[]string{"tar", "czf", "-", "-C", path.Dir(containerPath), path.Base(containerPath)},
			nil, w,
		)
	} else {
		err = mokubernetes.ExecInPodToWriter(
			target.Namespace, target.Pod, target.Container,
			[]string{"cat", containerPath},
			nil, w,
		)
	}
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	result.SizeInBytes = int64(buf.Len())
	_ = multiPartWriter.Close()

	serviceLogger.Debug("Uploading file", "size", result.SizeInBytes, "filename", filename, "postTo", postTo)
	req, err := http.NewRequest("POST", postTo, buf)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	req.Header = utils.HttpHeader("")
	req.Header.Set("Content-Type", multiPartWriter.FormDataContentType())

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		serviceLogger.Error("Error sending request", "status", response.Status)
		result.Error = fmt.Sprintf("%s - '%s'.", postTo, response.Status)
	}

	return result, nil
}

func uploadedImpl(target fileExecTarget, tempZipFileSrc string, requestPath string, sizeInBytes int64) error {
	containerPath, err := resolvePath(target.MountRoot, requestPath)
	if err != nil {
		return fmt.Errorf("error verifying file %s: %w", requestPath, err)
	}
	serviceLogger.Info(
		"verified file",
		"pod", target.Pod,
		"container", target.Container,
		"targetDestination", containerPath,
		"size", utils.BytesToHumanReadable(sizeInBytes),
		"path", requestPath,
	)

	// Convert zip → tar in-memory, then stream into the target pod via exec stdin.
	tarBuf, err := zipToTar(tempZipFileSrc)
	if err != nil {
		return fmt.Errorf("error converting zip to tar for %s: %w", requestPath, err)
	}

	_, err = mokubernetes.ExecInPod(
		target.Namespace, target.Pod, target.Container,
		[]string{"sh", "-c", fmt.Sprintf("mkdir -p '%s' && tar xf - -C '%s'", containerPath, containerPath)},
		tarBuf,
	)
	return err
}

func createFolderImpl(target fileExecTarget, requestPath string, mode string) error {
	containerPath, err := resolvePath(target.MountRoot, requestPath)
	if err != nil {
		return err
	}
	args := []string{"mkdir", "-p"}
	if mode != "" {
		normalized, err := normalizeMode(mode)
		if err != nil {
			return err
		}
		args = append(args, "-m", normalized)
	}
	args = append(args, "--", containerPath)
	_, err = mokubernetes.ExecInPod(target.Namespace, target.Pod, target.Container, args, nil)
	return err
}

func renameImpl(target fileExecTarget, requestPath string, newName string, newRequestPath string) error {
	containerPath, err := resolvePath(target.MountRoot, requestPath)
	if err != nil {
		return err
	}
	var destination string
	switch {
	case newRequestPath != "" && newName != "":
		return fmt.Errorf("give either newName or newPath, not both")
	case newRequestPath != "":
		destination, err = resolvePath(target.MountRoot, newRequestPath)
		if err != nil {
			return err
		}
	case newName != "":
		if strings.ContainsAny(newName, "/\\") || strings.ContainsRune(newName, 0) {
			return fmt.Errorf("newName must be a plain file name")
		}
		destination = path.Join(path.Dir(containerPath), newName)
	default:
		return fmt.Errorf("newName or newPath is required")
	}
	_, err = mokubernetes.ExecInPod(
		target.Namespace, target.Pod, target.Container,
		[]string{"mv", "--", containerPath, destination},
		nil,
	)
	return err
}

func chownImpl(target fileExecTarget, requestPath string, uidString string, gidString string) error {
	containerPath, err := resolvePath(target.MountRoot, requestPath)
	if err != nil {
		return err
	}

	if err := validateOwnerPart(uidString); err != nil {
		return fmt.Errorf("uid: %w", err)
	}
	if err := validateOwnerPart(gidString); err != nil {
		return fmt.Errorf("gid: %w", err)
	}

	_, err = mokubernetes.ExecInPod(
		target.Namespace, target.Pod, target.Container,
		[]string{"chown", fmt.Sprintf("%s:%s", uidString, gidString), "--", containerPath},
		nil,
	)
	return err
}

func chmodImpl(target fileExecTarget, requestPath string, mode string) error {
	containerPath, err := resolvePath(target.MountRoot, requestPath)
	if err != nil {
		return err
	}

	mod, err := normalizeMode(mode)
	if err != nil {
		return err
	}

	_, err = mokubernetes.ExecInPod(
		target.Namespace, target.Pod, target.Container,
		[]string{"chmod", mod, "--", containerPath},
		nil,
	)
	return err
}

func deleteImpl(target fileExecTarget, requestPath string, recursive bool) error {
	containerPath, err := resolvePath(target.MountRoot, requestPath)
	if err != nil {
		return err
	}
	if recursive {
		_, err = mokubernetes.ExecInPod(target.Namespace, target.Pod, target.Container, []string{"rm", "-rf", "--", containerPath}, nil)
		return err
	}
	// Not recursive: a file goes with rm, a folder only when empty (rmdir
	// refuses otherwise) — busybox rm has no -d, so the type decides the tool.
	info, err := infoImpl(target, requestPath)
	if err != nil {
		return err
	}
	tool := []string{"rm", "-f", "--", containerPath}
	if info.Type == "directory" {
		tool = []string{"rmdir", "--", containerPath}
	}
	_, err = mokubernetes.ExecInPod(target.Namespace, target.Pod, target.Container, tool, nil)
	return err
}

// normalizeMode accepts "755", "0755" or "u+x"-style symbolic modes and
// returns what chmod/mkdir -m take. Numeric modes are zero-padded to four
// digits and checked to be octal.
func normalizeMode(mode string) (string, error) {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return "", fmt.Errorf("mode cannot be empty")
	}
	if symbolicMode.MatchString(mode) {
		return mode, nil
	}
	padded := fmt.Sprintf("%0*s", 4, mode)
	if _, err := strconv.ParseUint(padded, 8, 32); err != nil {
		return "", fmt.Errorf("failed to parse oct permissions: %s %w", mode, err)
	}
	return padded, nil
}

// symbolicMode is chmod's symbolic form, e.g. u+x or go-w,u=rwx.
var symbolicMode = regexp.MustCompile(`^[ugoa]*[-+=][rwxXst]+(,[ugoa]*[-+=][rwxXst]+)*$`)

// ownerPart is a numeric id or a user/group name as the system accepts it.
var ownerPart = regexp.MustCompile(`^([0-9]{1,10}|[a-z_][a-z0-9_.-]{0,31}\$?)$`)

func validateOwnerPart(value string) error {
	if !ownerPart.MatchString(value) {
		return fmt.Errorf("%q is neither a numeric id nor a valid name", value)
	}
	if n, err := strconv.ParseUint(value, 10, 64); err == nil && n >= 1<<32 {
		return fmt.Errorf("%q is out of range", value)
	}
	return nil
}

// ── types ─────────────────────────────────────────────────────────────────────

type FilesDownloadResponse struct {
	SizeInBytes int64  `json:"sizeInBytes"`
	Error       string `json:"error,omitempty"`
}

type FilesUploadRequest struct {
	File        dtos.PvcFileRequestDto `json:"file"`
	SizeInBytes int64                  `json:"sizeInBytes"`
	Id          string                 `json:"id"`
}

// ── helpers ───────────────────────────────────────────────────────────────────

// resolvePath validates the request path and returns the absolute path inside
// the container, rooted at mountRoot. Legacy NFS callers pass "/exports".
func resolvePath(mountRoot, requestPath string) (string, error) {
	if requestPath == "" {
		return "", fmt.Errorf("path cannot be empty. Must at least contain '/'")
	}
	if strings.Contains(requestPath, "..") {
		return "", fmt.Errorf("path cannot contain '..'")
	}
	if strings.Contains(requestPath, "./") {
		return "", fmt.Errorf("path cannot contain './'")
	}
	if strings.Contains(requestPath, "~") {
		return "", fmt.Errorf("path cannot contain '~'")
	}

	relPath := strings.TrimPrefix(requestPath, "/")
	if relPath == "" {
		return mountRoot, nil
	}
	// A pod's own filesystem has mount root "/"; joining naively would yield "//x".
	joined := strings.TrimSuffix(mountRoot, "/") + "/" + relPath

	// Defense in depth on top of the rejections above: the cleaned result must
	// stay inside mountRoot. The uncleaned join is returned so legacy paths
	// stay byte-for-byte identical (e.g. trailing slashes survive).
	cleanedRoot := filepath.Clean(mountRoot)
	prefix := cleanedRoot + "/"
	if cleanedRoot == "/" {
		prefix = "/"
	}
	if cleaned := filepath.Clean(joined); cleaned != cleanedRoot && !strings.HasPrefix(cleaned, prefix) {
		return "", fmt.Errorf("path escapes mount root")
	}
	return joined, nil
}

// parseStatLine parses one line of `stat -c '%n\t%F\t%s\t%u\t%g\t%a\t%Y'` output.
// headerFilename makes a file name safe for a Content-Disposition header:
// quotes and backslashes are escaped, control characters (a newline would end
// the header and lose the whole part) become underscores.
func headerFilename(name string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return '_'
		}
		return r
	}, name)
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(cleaned)
}

func parseStatLine(rootContainerPath, line string) (dtos.PersistentFileDto, error) {
	parts := strings.Split(line, "\t")
	if len(parts) < 7 {
		return dtos.PersistentFileDto{}, fmt.Errorf("unexpected stat output: %q", line)
	}

	fullPath := parts[0]
	fileType := "file"
	if strings.Contains(parts[1], "directory") {
		fileType = "directory"
	}

	size, _ := strconv.ParseInt(parts[2], 10, 64)
	uid := parts[3]
	gid := parts[4]
	mode := parts[5]
	modEpoch, _ := strconv.ParseInt(parts[6], 10, 64)

	name := path.Base(fullPath)
	relPath := strings.TrimPrefix(fullPath, rootContainerPath+"/")
	if relPath == fullPath {
		relPath = name
	}

	sizeBytes := size
	if fileType == "directory" {
		sizeBytes = -1
	}

	return dtos.PersistentFileDto{
		Name:         name,
		Type:         fileType,
		RelativePath: relPath,
		Extension:    path.Ext(name),
		SizeInBytes:  sizeBytes,
		Size:         utils.BytesToHumanReadable(sizeBytes),
		Hash:         utils.QuickHash(fullPath),
		ModifiedAt:   time.Unix(modEpoch, 0).Format(time.RFC3339),
		Uid_gid:      uid + ":" + gid,
		Mode:         mode,
	}, nil
}

// zipToTar reads a zip archive and re-encodes it as a tar stream in memory.
func zipToTar(zipPath string) (*bytes.Buffer, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	for _, f := range r.File {
		hdr, err := tar.FileInfoHeader(f.FileInfo(), "")
		if err != nil {
			return nil, err
		}
		hdr.Name = f.Name
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			_, err = io.Copy(tw, rc)
			_ = rc.Close()
			if err != nil {
				return nil, err
			}
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	return &buf, nil
}
