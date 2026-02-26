package services

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	mokubernetes "mogenius-operator/src/kubernetes"
	"mogenius-operator/src/dtos"
	"mogenius-operator/src/utils"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

func List(folder dtos.PersistentFileRequestDto) ([]dtos.PersistentFileDto, error) {
	containerPath, err := resolveNfs(&folder)
	if err != nil {
		return nil, err
	}

	output, err := mokubernetes.ExecInNfsPod(
		folder.VolumeNamespace, folder.VolumeName,
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
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
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

func Info(r dtos.PersistentFileRequestDto) (dtos.PersistentFileDto, error) {
	containerPath, err := resolveNfs(&r)
	if err != nil {
		return dtos.PersistentFileDto{}, err
	}

	output, err := mokubernetes.ExecInNfsPod(
		r.VolumeNamespace, r.VolumeName,
		[]string{"stat", "-c", "%n\t%F\t%s\t%u\t%g\t%a\t%Y", containerPath},
		nil,
	)
	if err != nil {
		return dtos.PersistentFileDto{}, err
	}
	return parseStatLine("/exports", strings.TrimSpace(output))
}

func Download(pfile dtos.PersistentFileRequestDto, postTo string) (FilesDownloadResponse, error) {
	result := FilesDownloadResponse{}

	containerPath, err := resolveNfs(&pfile)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	info, err := Info(pfile)
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
		err = mokubernetes.ExecInNfsPodToWriter(
			pfile.VolumeNamespace, pfile.VolumeName,
			[]string{"tar", "czf", "-", "-C", path.Dir(containerPath), path.Base(containerPath)},
			nil, w,
		)
	} else {
		err = mokubernetes.ExecInNfsPodToWriter(
			pfile.VolumeNamespace, pfile.VolumeName,
			[]string{"cat", containerPath},
			nil, w,
		)
	}
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	result.SizeInBytes = int64(buf.Len())
	multiPartWriter.Close()

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
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		serviceLogger.Error("Error sending request", "status", response.Status)
		result.Error = fmt.Sprintf("%s - '%s'.", postTo, response.Status)
	}

	return result, nil
}

func Uploaded(tempZipFileSrc string, fileReq FilesUploadRequest) error {
	containerPath, err := resolveNfs(&fileReq.File)
	if err != nil {
		return fmt.Errorf("error verifying file %s: %w", fileReq.File.Path, err)
	}
	serviceLogger.Info(
		"verified file",
		"VolumeName", fileReq.File.VolumeName,
		"targetDestination", containerPath,
		"size", utils.BytesToHumanReadable(fileReq.SizeInBytes),
		"path", fileReq.File.Path,
	)

	// Convert zip → tar in-memory, then stream into the NFS pod via exec stdin.
	tarBuf, err := zipToTar(tempZipFileSrc)
	if err != nil {
		return fmt.Errorf("error converting zip to tar for %s: %w", fileReq.File.Path, err)
	}

	_, err = mokubernetes.ExecInNfsPod(
		fileReq.File.VolumeNamespace, fileReq.File.VolumeName,
		[]string{"sh", "-c", fmt.Sprintf("mkdir -p '%s' && tar xf - -C '%s'", containerPath, containerPath)},
		tarBuf,
	)
	return err
}

func CreateFolder(folder dtos.PersistentFileRequestDto) error {
	containerPath, err := resolveNfs(&folder)
	if err != nil {
		return err
	}
	_, err = mokubernetes.ExecInNfsPod(
		folder.VolumeNamespace, folder.VolumeName,
		[]string{"mkdir", "-p", containerPath},
		nil,
	)
	return err
}

func Rename(file dtos.PersistentFileRequestDto, newName string) error {
	containerPath, err := resolveNfs(&file)
	if err != nil {
		return err
	}
	newPath := path.Join(path.Dir(containerPath), newName)
	_, err = mokubernetes.ExecInNfsPod(
		file.VolumeNamespace, file.VolumeName,
		[]string{"mv", containerPath, newPath},
		nil,
	)
	return err
}

func Chown(file dtos.PersistentFileRequestDto, uidString string, gidString string) error {
	containerPath, err := resolveNfs(&file)
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

	_, err = mokubernetes.ExecInNfsPod(
		file.VolumeNamespace, file.VolumeName,
		[]string{"chown", fmt.Sprintf("%s:%s", uidString, gidString), containerPath},
		nil,
	)
	return err
}

func Chmod(file dtos.PersistentFileRequestDto, mode string) error {
	containerPath, err := resolveNfs(&file)
	if err != nil {
		return err
	}

	mod := fmt.Sprintf("%0*s", 4, mode)
	if _, err = strconv.ParseUint(mod, 0, 32); err != nil {
		return fmt.Errorf("failed to parse oct permissions: %s %w", mod, err)
	}

	_, err = mokubernetes.ExecInNfsPod(
		file.VolumeNamespace, file.VolumeName,
		[]string{"chmod", mod, containerPath},
		nil,
	)
	return err
}

func Delete(file dtos.PersistentFileRequestDto) error {
	containerPath, err := resolveNfs(&file)
	if err != nil {
		return err
	}
	_, err = mokubernetes.ExecInNfsPod(
		file.VolumeNamespace, file.VolumeName,
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

// ── helpers ───────────────────────────────────────────────────────────────────

// resolveNfs validates the request path and returns the absolute path inside the
// NFS container (/exports/...).
func resolveNfs(data *dtos.PersistentFileRequestDto) (string, error) {
	if data.Path == "" {
		return "", fmt.Errorf("path cannot be empty. Must at least contain '/'")
	}
	if strings.Contains(data.Path, "..") {
		return "", fmt.Errorf("path cannot contain '..'")
	}
	if strings.Contains(data.Path, "./") {
		return "", fmt.Errorf("path cannot contain './'")
	}
	if strings.Contains(data.Path, "~") {
		return "", fmt.Errorf("path cannot contain '~'")
	}

	relPath := strings.TrimPrefix(data.Path, "/")
	if relPath == "" {
		return "/exports", nil
	}
	return "/exports/" + relPath, nil
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
	defer r.Close()

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
			rc.Close()
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
