package services

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"mogenius-operator/src/dtos"
	mokubernetes "mogenius-operator/src/kubernetes"
	"mogenius-operator/src/utils"
	"net/http"
	"path"
	"path/filepath"
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

// resolveNfsFileTarget resolves the legacy NFS exec target: the nfs-server pod
// of a mogenius volume, always container "nfs-server" with mount root /exports.
func resolveNfsFileTarget(volumeNamespace, volumeName string) (fileExecTarget, error) {
	podNames := mokubernetes.AllPodNamesForLabel(volumeNamespace, "app", fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName))
	if len(podNames) == 0 {
		return fileExecTarget{}, fmt.Errorf("NFS server pod not found for %s/%s", volumeNamespace, volumeName)
	}
	return fileExecTarget{
		Namespace: volumeNamespace,
		Pod:       podNames[0],
		Container: "nfs-server",
		MountRoot: "/exports",
	}, nil
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

// ── legacy NFS entry points (deprecated patterns files/*) ─────────────────────

func List(folder dtos.PersistentFileRequestDto) ([]dtos.PersistentFileDto, error) {
	target, err := resolveNfsFileTarget(folder.VolumeNamespace, folder.VolumeName)
	if err != nil {
		return nil, err
	}
	return listImpl(target, folder.Path)
}

func Info(r dtos.PersistentFileRequestDto) (dtos.PersistentFileDto, error) {
	target, err := resolveNfsFileTarget(r.VolumeNamespace, r.VolumeName)
	if err != nil {
		return dtos.PersistentFileDto{}, err
	}
	return infoImpl(target, r.Path)
}

func Download(pfile dtos.PersistentFileRequestDto, postTo string) (FilesDownloadResponse, error) {
	target, err := resolveNfsFileTarget(pfile.VolumeNamespace, pfile.VolumeName)
	if err != nil {
		return FilesDownloadResponse{Error: err.Error()}, err
	}
	return downloadImpl(target, pfile.Path, postTo)
}

func Uploaded(tempZipFileSrc string, fileReq FilesUploadRequest) error {
	target, err := resolveNfsFileTarget(fileReq.File.VolumeNamespace, fileReq.File.VolumeName)
	if err != nil {
		return fmt.Errorf("error verifying file %s: %w", fileReq.File.Path, err)
	}
	return uploadedImpl(target, tempZipFileSrc, fileReq.File.Path, fileReq.SizeInBytes)
}

func CreateFolder(folder dtos.PersistentFileRequestDto) error {
	target, err := resolveNfsFileTarget(folder.VolumeNamespace, folder.VolumeName)
	if err != nil {
		return err
	}
	return createFolderImpl(target, folder.Path)
}

func Rename(file dtos.PersistentFileRequestDto, newName string) error {
	target, err := resolveNfsFileTarget(file.VolumeNamespace, file.VolumeName)
	if err != nil {
		return err
	}
	return renameImpl(target, file.Path, newName)
}

func Chown(file dtos.PersistentFileRequestDto, uidString string, gidString string) error {
	target, err := resolveNfsFileTarget(file.VolumeNamespace, file.VolumeName)
	if err != nil {
		return err
	}
	return chownImpl(target, file.Path, uidString, gidString)
}

func Chmod(file dtos.PersistentFileRequestDto, mode string) error {
	target, err := resolveNfsFileTarget(file.VolumeNamespace, file.VolumeName)
	if err != nil {
		return err
	}
	return chmodImpl(target, file.Path, mode)
}

func Delete(file dtos.PersistentFileRequestDto) error {
	target, err := resolveNfsFileTarget(file.VolumeNamespace, file.VolumeName)
	if err != nil {
		return err
	}
	return deleteImpl(target, file.Path)
}

// ── v2 entry points (files/v2/* patterns, any mounted PVC) ────────────────────

func ListV2(folder dtos.PvcFileRequestDto) ([]dtos.PersistentFileDto, error) {
	target, err := resolvePvcFileTarget(folder.Namespace, folder.PvcName)
	if err != nil {
		return nil, err
	}
	return listImpl(target, folder.Path)
}

func InfoV2(r dtos.PvcFileRequestDto) (dtos.PersistentFileDto, error) {
	target, err := resolvePvcFileTarget(r.Namespace, r.PvcName)
	if err != nil {
		return dtos.PersistentFileDto{}, err
	}
	return infoImpl(target, r.Path)
}

func DownloadV2(pfile dtos.PvcFileRequestDto, postTo string) (FilesDownloadResponse, error) {
	target, err := resolvePvcFileTarget(pfile.Namespace, pfile.PvcName)
	if err != nil {
		return FilesDownloadResponse{Error: err.Error()}, err
	}
	return downloadImpl(target, pfile.Path, postTo)
}

func UploadedV2(tempZipFileSrc string, fileReq FilesUploadRequestV2) error {
	target, err := resolvePvcFileTarget(fileReq.File.Namespace, fileReq.File.PvcName)
	if err != nil {
		return fmt.Errorf("error verifying file %s: %w", fileReq.File.Path, err)
	}
	return uploadedImpl(target, tempZipFileSrc, fileReq.File.Path, fileReq.SizeInBytes)
}

func CreateFolderV2(folder dtos.PvcFileRequestDto) error {
	target, err := resolvePvcFileTarget(folder.Namespace, folder.PvcName)
	if err != nil {
		return err
	}
	return createFolderImpl(target, folder.Path)
}

func RenameV2(file dtos.PvcFileRequestDto, newName string) error {
	target, err := resolvePvcFileTarget(file.Namespace, file.PvcName)
	if err != nil {
		return err
	}
	return renameImpl(target, file.Path, newName)
}

func ChownV2(file dtos.PvcFileRequestDto, uidString string, gidString string) error {
	target, err := resolvePvcFileTarget(file.Namespace, file.PvcName)
	if err != nil {
		return err
	}
	return chownImpl(target, file.Path, uidString, gidString)
}

func ChmodV2(file dtos.PvcFileRequestDto, mode string) error {
	target, err := resolvePvcFileTarget(file.Namespace, file.PvcName)
	if err != nil {
		return err
	}
	return chmodImpl(target, file.Path, mode)
}

func DeleteV2(file dtos.PvcFileRequestDto) error {
	target, err := resolvePvcFileTarget(file.Namespace, file.PvcName)
	if err != nil {
		return err
	}
	return deleteImpl(target, file.Path)
}

// ── target-based implementations ──────────────────────────────────────────────

func listImpl(target fileExecTarget, requestPath string) ([]dtos.PersistentFileDto, error) {
	containerPath, err := resolvePath(target.MountRoot, requestPath)
	if err != nil {
		return nil, err
	}

	output, err := mokubernetes.ExecInPod(
		target.Namespace, target.Pod, target.Container,
		[]string{
			"find", containerPath,
			"-maxdepth", "1", "-mindepth", "1",
			"-exec", "stat", "-c", "%n\t%F\t%s\t%u\t%g\t%a\t%Y", "{}", ";",
		},
		nil,
	)
	if err != nil {
		return nil, err
	}

	var result []dtos.PersistentFileDto
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
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
		[]string{"stat", "-c", "%n\t%F\t%s\t%u\t%g\t%a\t%Y", containerPath},
		nil,
	)
	if err != nil {
		return dtos.PersistentFileDto{}, err
	}
	return parseStatLine(target.MountRoot, strings.TrimSpace(output))
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

	var filename string
	if info.Type == "directory" {
		filename = info.Name + ".tar.gz"
	} else {
		filename = info.Name
	}

	w, err := multiPartWriter.CreateFormFile("file", filename)
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

func createFolderImpl(target fileExecTarget, requestPath string) error {
	containerPath, err := resolvePath(target.MountRoot, requestPath)
	if err != nil {
		return err
	}
	_, err = mokubernetes.ExecInPod(
		target.Namespace, target.Pod, target.Container,
		[]string{"mkdir", "-p", containerPath},
		nil,
	)
	return err
}

func renameImpl(target fileExecTarget, requestPath string, newName string) error {
	containerPath, err := resolvePath(target.MountRoot, requestPath)
	if err != nil {
		return err
	}
	newPath := path.Join(path.Dir(containerPath), newName)
	_, err = mokubernetes.ExecInPod(
		target.Namespace, target.Pod, target.Container,
		[]string{"mv", containerPath, newPath},
		nil,
	)
	return err
}

func chownImpl(target fileExecTarget, requestPath string, uidString string, gidString string) error {
	containerPath, err := resolvePath(target.MountRoot, requestPath)
	if err != nil {
		return err
	}

	uid, err := strconv.Atoi(uidString)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(gidString)
	if err != nil {
		return err
	}
	maxInt := int(math.Pow(2, 32))
	if uid <= 0 || uid >= maxInt || gid <= 0 || gid >= maxInt {
		return fmt.Errorf("gid/uid > 0 and < 2^32")
	}

	_, err = mokubernetes.ExecInPod(
		target.Namespace, target.Pod, target.Container,
		[]string{"chown", fmt.Sprintf("%s:%s", uidString, gidString), containerPath},
		nil,
	)
	return err
}

func chmodImpl(target fileExecTarget, requestPath string, mode string) error {
	containerPath, err := resolvePath(target.MountRoot, requestPath)
	if err != nil {
		return err
	}

	mod := fmt.Sprintf("%0*s", 4, mode)
	if _, err = strconv.ParseUint(mod, 0, 32); err != nil {
		return fmt.Errorf("failed to parse oct permissions: %s %w", mod, err)
	}

	_, err = mokubernetes.ExecInPod(
		target.Namespace, target.Pod, target.Container,
		[]string{"chmod", mod, containerPath},
		nil,
	)
	return err
}

func deleteImpl(target fileExecTarget, requestPath string) error {
	containerPath, err := resolvePath(target.MountRoot, requestPath)
	if err != nil {
		return err
	}
	_, err = mokubernetes.ExecInPod(
		target.Namespace, target.Pod, target.Container,
		[]string{"rm", "-rf", containerPath},
		nil,
	)
	return err
}

// ── types ─────────────────────────────────────────────────────────────────────

type FilesDownloadResponse struct {
	SizeInBytes int64  `json:"sizeInBytes"`
	Error       string `json:"error,omitempty"`
}

type FilesUploadRequest struct {
	File        dtos.PersistentFileRequestDto `json:"file"`
	SizeInBytes int64                         `json:"sizeInBytes"`
	Id          string                        `json:"id"`
}

type FilesUploadRequestV2 struct {
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
	joined := mountRoot + "/" + relPath

	// Defense in depth on top of the rejections above: the cleaned result must
	// stay inside mountRoot. The uncleaned join is returned so legacy paths
	// stay byte-for-byte identical (e.g. trailing slashes survive).
	cleanedRoot := filepath.Clean(mountRoot)
	if cleaned := filepath.Clean(joined); cleaned != cleanedRoot && !strings.HasPrefix(cleaned, cleanedRoot+"/") {
		return "", fmt.Errorf("path escapes mount root")
	}
	return joined, nil
}

// parseStatLine parses one line of `stat -c '%n\t%F\t%s\t%u\t%g\t%a\t%Y'` output.
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
