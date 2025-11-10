package services

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"mogenius-operator/src/dtos"
	"mogenius-operator/src/utils"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

func List(folder dtos.PersistentFileRequestDto) []dtos.PersistentFileDto {
	result := []dtos.PersistentFileDto{}
	pathToFile, err := verify(&folder)
	if err != nil {
		return result
	}
	result, err = ListDirWithTimeout(pathToFile, 250*time.Millisecond)
	if err != nil {
		serviceLogger.Error("Files List Error", "error", err)
	}
	return result
}

func Info(r dtos.PersistentFileRequestDto) dtos.PersistentFileDto {
	result := dtos.PersistentFileDto{}
	pathToFile, err := verify(&r)
	if err != nil {
		serviceLogger.Error("file info verify error", "error", err)
		return result
	}
	return dtos.PersistentFileDtoFrom(pathToFile, pathToFile)
}

func Download(pfile dtos.PersistentFileRequestDto, postTo string) FilesDownloadResponse {
	result := FilesDownloadResponse{
		SizeInBytes: 0,
	}
	pathToFile, err := verify(&pfile)
	if err != nil {
		result.Error = err.Error()
		serviceLogger.Debug("file download verify error", "error", err)
		return result
	}
	file, err := os.Open(pathToFile)
	if err != nil {
		result.Error = err.Error()
		serviceLogger.Debug("file download open error", "error", err)
		return result
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		result.Error = err.Error()
		serviceLogger.Debug("file download stat error", "error", err)
		return result
	}

	// Generate filename
	filename := file.Name()
	if info.IsDir() {
		filename = file.Name() + ".zip"
	}

	// Create writer  and form-data header for zip and non-zip
	buf := new(bytes.Buffer)
	multiPartWriter := multipart.NewWriter(buf)
	w, err := multiPartWriter.CreateFormFile("file", filename)
	if err != nil {
		serviceLogger.Error("Error creating form file", "error", err)
		result.Error = err.Error()
		return result
	}

	if info.IsDir() {
		// SEND ZIPPED DIR TO HTTP
		zipWriter := zip.NewWriter(w)

		// Add all files in a directory to the archive
		err = filepath.Walk(pathToFile, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(pathToFile, filePath)
			if err != nil {
				return err
			}

			zipFile, err := zipWriter.Create(relPath)
			if err != nil {
				return err
			}

			srcFile, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer srcFile.Close()

			_, err = io.Copy(zipFile, srcFile)
			return err
		})
		if err != nil {
			serviceLogger.Error("directory zip walk files error", "error", err)
			result.Error = err.Error()
			return result
		}

		// Close the zip archive
		err = zipWriter.Close()
		if err != nil {
			serviceLogger.Error("zip error", "error", err)
			result.Error = err.Error()
			return result
		}
	} else {
		// SEND FILE TO HTTP
		_, err = io.Copy(w, file)
		if err != nil {
			serviceLogger.Error("Error copying file", "error", err)
			result.Error = err.Error()
			return result
		}
	}

	result.SizeInBytes = int64(buf.Len())

	multiPartWriter.Close()

	// Upload the file
	serviceLogger.Debug("Uploading file", "size", result.SizeInBytes, "filename", filename, "postTo", postTo)
	req, err := http.NewRequest("POST", postTo, buf)
	if err != nil {
		serviceLogger.Error("Error sending request", "error", err)
		result.Error = err.Error()
		return result
	}
	req.Header = utils.HttpHeader("")
	req.Header.Set("Content-Type", multiPartWriter.FormDataContentType())

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		serviceLogger.Error("Error sending request", "error", err)
		result.Error = err.Error()
		return result
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		serviceLogger.Error("Error sending request", "status", response.Status)
		result.Error = fmt.Sprintf("%s - '%s'.", postTo, response.Status)
	}

	return result
}

func Uploaded(tempZipFileSrc string, fileReq FilesUploadRequest) error {
	// 1: VERIFY
	targetDestination, err := verify(&fileReq.File)
	if err != nil {
		return fmt.Errorf("Error verifying file %s: %s", fileReq.File.Path, err.Error())
	}
	serviceLogger.Info(
		"verified file",
		"VolumeName", fileReq.File.VolumeName,
		"targetDestionation", targetDestination,
		"size", utils.BytesToHumanReadable(fileReq.SizeInBytes),
		"path", fileReq.File.Path,
	)

	//2: UNZIP FILE TO TEMP
	files, err := utils.ZipExtract(tempZipFileSrc, targetDestination)
	if err != nil {
		return fmt.Errorf("Error extracting file %s: %s", fileReq.File.Path, err.Error())
	}
	for _, file := range files {
		serviceLogger.Info("uncompress: " + file)
	}
	return nil
}

func CreateFolder(folder dtos.PersistentFileRequestDto) error {
	pathToDir, err := verify(&folder)
	if err != nil {
		return err
	}
	err = os.Mkdir(pathToDir, 0777)
	if err != nil {
		return err
	}
	return nil
}

func Rename(file dtos.PersistentFileRequestDto, newName string) error {
	pathToFile, err := verify(&file)
	if err != nil {
		return err
	}

	dir, _ := filepath.Split(pathToFile)
	newPath := filepath.Join(dir, newName)

	err = os.Rename(pathToFile, newPath)
	if err != nil {
		return err
	}
	return nil
}

func Chown(file dtos.PersistentFileRequestDto, uidString string, gidString string) any {
	pathToDir, err := verify(&file)
	if err != nil {
		return utils.CreateError(err)
	}

	gid, err := strconv.Atoi(gidString)
	if err != nil {
		return utils.CreateError(err)
	}
	uid, err := strconv.Atoi(uidString)
	if err != nil {
		return utils.CreateError(err)
	}

	maxInt := int(math.Pow(2, 32))
	if gid > 0 && gid < maxInt && uid > 0 && uid < maxInt {
		err = os.Chown(pathToDir, uid, gid)
		if err != nil {
			return utils.CreateError(err)
		}
	} else {
		return utils.CreateError(fmt.Errorf("gid/uid > 0 and < 2^32"))
	}
	return nil
}

func Chmod(file dtos.PersistentFileRequestDto, mode string) any {
	pathToDir, err := verify(&file)
	if err != nil {
		return utils.CreateError(err)
	}

	// padding left leading zero if missing
	var mod = fmt.Sprintf("%0*s", 4, mode)
	// Convert to base 8 (which is the traditional base for unix file modes)
	// base 0, and it'll automatically choose base 8 due to the leading 0
	permissions, err := strconv.ParseUint(mod, 0, 32)
	if err != nil {
		return fmt.Errorf("failed to parse oct permissions: %s %w", mod, err)
	}

	// Set the file permissions using the integer value
	err = os.Chmod(pathToDir, os.FileMode(permissions))
	if err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}

func Delete(file dtos.PersistentFileRequestDto) any {
	pathToDir, err := verify(&file)
	if err != nil {
		return err
	}
	err = os.RemoveAll(pathToDir)
	if err != nil {
		return err
	}
	return nil
}

type FilesDownloadResponse struct {
	SizeInBytes int64  `json:"sizeInBytes"`
	Error       string `json:"error,omitempty"`
}

type FilesUploadRequest struct {
	File        dtos.PersistentFileRequestDto `json:"file"`
	SizeInBytes int64                         `json:"sizeInBytes"`
	Id          string                        `json:"id"`
}

func ListDirWithTimeout(root string, timeout time.Duration) ([]dtos.PersistentFileDto, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	items, err := ListDir(ctx, root)
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		// If the context has timed out, set all directory sizes to zero
		for i := range items {
			if items[i].Type == "directory" {
				items[i].SizeInBytes = -1
				items[i].Size = "-1"
			}
		}
	default:
		// If the context has not timed out, return the items as is
	}

	return items, nil
}

func ListDir(ctx context.Context, root string) ([]dtos.PersistentFileDto, error) {
	var items []dtos.PersistentFileDto
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		item := dtos.PersistentFileDtoFrom(root, path)

		if entry.IsDir() {
			wg.Go(func() {
				select {
				case <-ctx.Done():
					return
				default:
					size, err := DirSize(ctx, path)
					if err != nil {
						mu.Lock()
						errs = append(errs, err)
						mu.Unlock()
						return
					}
					item.SizeInBytes = size
					item.Size = utils.BytesToHumanReadable(size)
					mu.Lock()
					items = append(items, item)
					mu.Unlock()
				}
			})
		} else {
			mu.Lock()
			items = append(items, item)
			mu.Unlock()
		}
	}

	wg.Wait()

	if len(errs) > 0 {
		return nil, fmt.Errorf("encountered errors: %v", errs)
	}

	return items, nil
}

func DirSize(ctx context.Context, path string) (int64, error) {
	var size int64
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && p != path {
			wg.Go(func() {
				select {
				case <-ctx.Done():
					return
				default:
					dirSize, err := DirSize(ctx, p)
					if err != nil {
						mu.Lock()
						errs = append(errs, err)
						mu.Unlock()
						return
					}
					mu.Lock()
					size += dirSize
					mu.Unlock()
				}
			})
			return filepath.SkipDir
		}

		select {
		case <-ctx.Done():
			return context.Canceled
		default:
			mu.Lock()
			size += info.Size()
			mu.Unlock()
		}
		return nil
	})

	wg.Wait()

	if len(errs) > 0 {
		return 0, fmt.Errorf("encountered errors: %v", errs)
	}

	if err != nil && err != context.Canceled {
		return 0, err
	}

	return size, nil
}

func verify(data *dtos.PersistentFileRequestDto) (string, error) {
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

	if strings.HasPrefix(data.Path, "/") {
		data.Path = data.Path[1:len(data.Path)]
	}
	if data.Path == "/" {
		data.Path = ""
	}

	mountPath := utils.MountPath(data.VolumeNamespace, data.VolumeName, "/", clientProvider.RunsInCluster())
	pathToFile := ""

	_, mountPathExists := os.Stat(mountPath)
	if os.IsNotExist(mountPathExists) {
		return "", fmt.Errorf("the volume '%s' does not exist", data.VolumeName)
	}

	if strings.HasSuffix(mountPath, "/") {
		pathToFile = fmt.Sprintf("%s%s", mountPath, data.Path)
	} else {
		if data.Path == "" {
			pathToFile = mountPath
		} else {
			pathToFile = fmt.Sprintf("%s/%s", mountPath, data.Path)
		}
	}

	return pathToFile, nil
}
