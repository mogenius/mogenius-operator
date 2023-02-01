package services

import (
	"bufio"
	"fmt"
	"io/fs"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/websocket"
)

func AllFiles() dtos.PersistentFileStatsDto {
	// TODO: Implement
	logger.Log.Error("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return dtos.PersistentFileStatsDto{}
}

func List(r FilesListRequest, c *websocket.Conn) []dtos.PersistentFileDto {
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

func Download(r FilesDownloadRequest, c *websocket.Conn) (*bufio.Reader, error) {
	pathToFile, err := verify(&r.File)
	if err != nil {
		return nil, fmt.Errorf("Download Error %s", err.Error())
	}
	file, err := os.Open(pathToFile)
	reader := bufio.NewReader(file)
	return reader, err
}

func Upload(r FilesUploadRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Error("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func Update(r FilesUpdateRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Error("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func CreateFolder(r FilesCreateFolderRequest, c *websocket.Conn) bool {
	// TODO: Implement
	logger.Log.Error("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return false
}

func Rename(r FilesRenameRequest, c *websocket.Conn) bool {
	// TODO: Implement
	logger.Log.Error("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return false
}

func Chown(r FilesChownRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Error("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func Chmod(r FilesChmodRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Error("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func Delete(r FilesDeleteRequest, c *websocket.Conn) bool {
	// TODO: Implement
	logger.Log.Error("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return false
}

// files/storage-stats GET

// files/list POST
type FilesListRequest struct {
	Folder dtos.PersistentFileRequestDto `json:"folder"`
}

func FilesListRequestExampleData() FilesListRequest {
	return FilesListRequest{
		Folder: dtos.PersistentFileRequestDtoExampleData(),
	}
}

// files/download POST
type FilesDownloadRequest struct {
	File dtos.PersistentFileRequestDto `json:"file"` // TODO: how? before go we simply used the response to write binary stream
}

func FilesDownloadRequestExampleData() FilesDownloadRequest {
	return FilesDownloadRequest{
		File: dtos.PersistentFileDownloadDtoExampleData(),
	}
}

// files/upload POST
type FilesUploadRequest struct {
	File        dtos.PersistentFileRequestDto `json:"file"`
	NewFileData string                        `json:"newFileData"` // TODO: base64? was originally Multer File Upload
}

func FilesUploadRequestExampleData() FilesUploadRequest {
	return FilesUploadRequest{
		File:        dtos.PersistentFileRequestDtoExampleData(),
		NewFileData: "",
	}
}

// files/update PATCH
type FilesUpdateRequest struct {
	File        dtos.PersistentFileRequestDto `json:"file"`
	NewFileData string                        `json:"newFileData"` // TODO: base64? was originally Multer File Upload
}

func FilesUpdateRequestExampleData() FilesUpdateRequest {
	return FilesUpdateRequest{
		File:        dtos.PersistentFileRequestDtoExampleData(),
		NewFileData: "",
	}
}

// files/create-folder POST
type FilesCreateFolderRequest struct {
	Folder dtos.PersistentFileRequestDto `json:"folder"`
}

func FilesCreateFolderRequestExampleData() FilesCreateFolderRequest {
	return FilesCreateFolderRequest{
		Folder: dtos.PersistentFileRequestDtoExampleData(),
	}
}

// files/rename POST
type FilesRenameRequest struct {
	File    dtos.PersistentFileRequestDto `json:"file"`
	NewName string                        `json:"newName"`
}

func FilesRenameRequestExampleData() FilesRenameRequest {
	return FilesRenameRequest{
		File:    dtos.PersistentFileRequestDtoExampleData(),
		NewName: "newName",
	}
}

// files/chown POST
type FilesChownRequest struct {
	File dtos.PersistentFileRequestDto `json:"file"`
	Uid  string                        `json:"uid"`
	Gid  string                        `json:"gid"`
}

func FilesChownRequestExampleData() FilesChownRequest {
	return FilesChownRequest{
		File: dtos.PersistentFileRequestDtoExampleData(),
		Uid:  "1234",
		Gid:  "2344",
	}
}

// files/chmod POST
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

// files/delete POST
type FilesDeleteRequest struct {
	File dtos.PersistentFileRequestDto `json:"file"`
}

func FilesDeleteRequestExampleData() FilesDeleteRequest {
	return FilesDeleteRequest{
		File: dtos.PersistentFileRequestDtoExampleData(),
	}
}

func listFiles(rootDir string, maxDepth int) ([]dtos.PersistentFileDto, error) {
	result := []dtos.PersistentFileDto{}
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		result = append(result, dtos.PersistentFileDtoFrom(path, d))
		if d.IsDir() && strings.Count(path, string(os.PathSeparator)) > maxDepth {
			fmt.Println("skip", path)
			return fs.SkipDir
		}
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
	if strings.Contains(data.Root, "..") {
		return "", fmt.Errorf("root cannot contain '..'")
	}
	if strings.Contains(data.Path, "./") {
		return "", fmt.Errorf("path cannot contain './'")
	}
	if strings.Contains(data.Root, "./") {
		return "", fmt.Errorf("root cannot begin with './'")
	}
	if strings.Contains(data.Path, "~") {
		return "", fmt.Errorf("path cannot contain '~'")
	}
	if strings.Contains(data.Root, "~") {
		return "", fmt.Errorf("root cannot begin with '~'")
	}
	if data.Root == "/" {
		data.Root = ""
	}
	if data.Path == "/" {
		data.Path = ""
	}

	dataRoot := utils.CONFIG.Misc.DefaultMountPath
	if utils.CONFIG.Misc.Debug {
		dataRoot = "."
	}
	pathToFile := fmt.Sprintf("%s%s%s", dataRoot, data.Root, data.Path)
	return pathToFile, nil
}
