package services

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"math"
	"mime/multipart"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func List(r FilesListRequest) []dtos.PersistentFileDto {
	result := []dtos.PersistentFileDto{}
	pathToFile, err := verify(&r.Folder)
	if err != nil {
		return result
	}
	result, err = listFiles(pathToFile, 0)
	if err != nil {
		logger.Log.Errorf("Files List Error: %s", err.Error())
	}
	return result
}

func Download(r FilesDownloadRequest) interface{} {
	result := FilesDownloadResponse{
		SizeInBytes: 0,
	}
	pathToFile, err := verify(&r.File)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	file, err := os.Open(pathToFile)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		result.Error = err.Error()
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
			logger.Log.Errorf("directory zip walk files error: %s", err.Error())
			result.Error = err.Error()
			return result
		}

		// Close the zip archive
		err = zipWriter.Close()
		if err != nil {
			logger.Log.Errorf("zip error: %s", err.Error())
			result.Error = err.Error()
			return result
		}
	} else {
		// SEND FILE TO HTTP
		if err != nil {
			fmt.Printf("Error creating form file: %s", err)
			result.Error = err.Error()
			return result
		}

		_, err = io.Copy(w, file)
		if err != nil {
			fmt.Printf("Error copying file: %s", err)
			result.Error = err.Error()
			return result
		}
	}

	result.SizeInBytes = int64(buf.Len())

	multiPartWriter.Close()

	// Upload the file
	response, err := http.Post(r.PostTo, multiPartWriter.FormDataContentType(), buf)
	if err != nil {
		fmt.Printf("Error sending request: %s", err)
		result.Error = err.Error()
		return result
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		result.Error = fmt.Sprintf("%s - '%s'.", r.PostTo, response.Status)
	}

	return result
}

func Uploaded(tempZipFileSrc string, fileReq FilesUploadRequest) interface{} {
	// 1: VERIFY
	targetDestination, err := verify(&fileReq.File)
	if err != nil {
		logger.Log.Error(err)
	}
	fmt.Printf("\n%s: %s (%s) -> %s\n", fileReq.File.VolumeName, targetDestination, utils.BytesToHumanReadable(fileReq.SizeInBytes), fileReq.File.Path)

	//2: UNZIP FILE TO TEMP
	files, err := utils.ZipExtract(tempZipFileSrc, targetDestination)
	if err != nil {
		logger.Log.Error(err)
	}
	for _, file := range files {
		fmt.Println("uncompress: " + file)
	}
	return nil
}

func CreateFolder(r FilesCreateFolderRequest) error {
	pathToDir, err := verify(&r.Folder)
	if err != nil {
		return err
	}
	err = os.Mkdir(pathToDir, 0777)
	if err != nil {
		return err
	}
	return nil
}

func Rename(r FilesRenameRequest) error {
	pathToFile, err := verify(&r.File)
	if err != nil {
		return err
	}

	dir, _ := filepath.Split(pathToFile)
	newPath := filepath.Join(dir, r.NewName)

	err = os.Rename(pathToFile, newPath)
	if err != nil {
		return err
	}
	return nil
}

func Chown(r FilesChownRequest) interface{} {
	pathToDir, err := verify(&r.File)
	if err != nil {
		return utils.CreateError(err)
	}

	gid, err := strconv.Atoi(r.Gid)
	if err != nil {
		return utils.CreateError(err)
	}
	uid, err := strconv.Atoi(r.Uid)
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

func Chmod(r FilesChmodRequest) interface{} {
	pathToDir, err := verify(&r.File)
	if err != nil {
		return utils.CreateError(err)
	}

	// padding left leading zero if missing
	var mod = fmt.Sprintf("%0*s", 4, r.Mode)
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

func Delete(r FilesDeleteRequest) interface{} {
	pathToDir, err := verify(&r.File)
	if err != nil {
		return err
	}
	err = os.RemoveAll(pathToDir)
	if err != nil {
		return err
	}
	return nil
}

type FilesListRequest struct {
	Folder dtos.PersistentFileRequestDto `json:"folder"`
}

func FilesListRequestExampleData() FilesListRequest {
	return FilesListRequest{
		Folder: dtos.PersistentFileRequestDtoExampleData(),
	}
}

type FilesDownloadRequest struct {
	File   dtos.PersistentFileRequestDto `json:"file"`
	PostTo string                        `json:"postTo"`
}

func FilesDownloadRequestExampleData() FilesDownloadRequest {
	return FilesDownloadRequest{
		File:   dtos.PersistentFileDownloadDtoExampleData(),
		PostTo: "http://localhost:8080/path/to/send/data?id=E694180D-4E18-41EC-A4CC-F402EA825D60",
	}
}

func FilesDownloadDirectoryRequestExampleData() FilesDownloadRequest {
	return FilesDownloadRequest{
		File:   dtos.PersistentFileRequestNewFolderDtoExampleData(),
		PostTo: "http://localhost:8080/path/to/send/data?id=E694180D-4E18-41EC-A4CC-F402EA825D60",
	}
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

func FilesUploadRequestExampleData() FilesUploadRequest {
	return FilesUploadRequest{
		File:        dtos.PersistentFileUploadDtoExampleData(),
		SizeInBytes: 21217588,
		Id:          "1234567890",
	}
}

type FilesCreateFolderRequest struct {
	Folder dtos.PersistentFileRequestDto `json:"folder"`
}

func FilesCreateFolderRequestExampleData() FilesCreateFolderRequest {
	return FilesCreateFolderRequest{
		Folder: dtos.PersistentFileRequestNewFolderDtoExampleData(),
	}
}

type FilesRenameRequest struct {
	File    dtos.PersistentFileRequestDto `json:"file"`
	NewName string                        `json:"newName"`
}

func FilesRenameRequestExampleData() FilesRenameRequest {
	return FilesRenameRequest{
		File:    dtos.PersistentFileRequestNewFolderDtoExampleData(),
		NewName: "newName",
	}
}

type FilesChownRequest struct {
	File dtos.PersistentFileRequestDto `json:"file"`
	Uid  string                        `json:"uid"`
	Gid  string                        `json:"gid"`
}

func FilesChownRequestExampleData() FilesChownRequest {
	return FilesChownRequest{
		File: dtos.PersistentFileRequestNewFolderDtoExampleData(),
		Uid:  "1234",
		Gid:  "2344",
	}
}

type FilesChmodRequest struct {
	File dtos.PersistentFileRequestDto `json:"file"`
	Mode string                        `json:"mode"`
}

func FilesChmodRequestExampleData() FilesChmodRequest {
	return FilesChmodRequest{
		File: dtos.PersistentFileRequestDtoExampleData(),
		Mode: "777",
	}
}

type FilesDeleteRequest struct {
	File dtos.PersistentFileRequestDto `json:"file"`
}

func FilesDeleteRequestExampleData() FilesDeleteRequest {
	return FilesDeleteRequest{
		File: dtos.PersistentFileRequestNewFolderDtoExampleData(),
	}
}

func listFiles(rootDir string, maxDepth int) ([]dtos.PersistentFileDto, error) {
	result := []dtos.PersistentFileDto{}
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(rootDir, path)
		//fmt.Printf("%d %s\n", strings.Count(relPath, string(os.PathSeparator)), path)
		if strings.Count(relPath, string(os.PathSeparator)) > maxDepth {
			return fs.SkipDir
		}
		result = append(result, dtos.PersistentFileDtoFrom(rootDir, path, d))
		return nil
	})
	return result, err
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

	mountPath := utils.MountPath(data.VolumeNamespace, data.VolumeName, "/")
	pathToFile := ""

	_, mountPathExists := os.Stat(mountPath)
	if os.IsNotExist(mountPathExists) {
		return "", fmt.Errorf("The volume '%s' does not exist.", data.VolumeName)
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
