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
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
)

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

func Download(r FilesDownloadRequest, c *websocket.Conn) (*bufio.Reader, int64, error) {
	pathToFile, err := verify(&r.File)
	if err != nil {
		return nil, 0, fmt.Errorf("Download Error %s", err.Error())
	}
	file, err := os.Open(pathToFile)
	info, err := file.Stat()
	var totalSize int64 = 0
	if err == nil {
		totalSize = info.Size()
	}
	reader := bufio.NewReader(file)
	return reader, totalSize, err
}

func Uploaded(tempZipFileSrc string, fileReq FilesUploadRequest) interface{} {
	// 1: VERIFY
	targetDestination, err := verify(&fileReq.File)
	if err != nil {
		logger.Log.Error(err)
	}
	fmt.Printf("\n%s: %s (%s) -> %s %s\n", fileReq.File.ClusterId, targetDestination, utils.BytesToHumanReadable(fileReq.SizeInBytes), fileReq.File.Root, fileReq.File.Path)

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

func Update(r FilesUpdateRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Error("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func CreateFolder(r FilesCreateFolderRequest, c *websocket.Conn) error {
	pathToDir, err := verify(&r.Folder)
	if err != nil {
		return err
	}
	err = os.Mkdir(pathToDir, fs.ModeDir)
	if err != nil {
		return err
	}
	return nil
}

func Rename(r FilesRenameRequest, c *websocket.Conn) error {
	pathToFile, err := verify(&r.File)
	if err != nil {
		return err
	}
	err = os.Rename(pathToFile, "asdasd")
	if err != nil {
		return err
	}
	return nil
}

func Chown(r FilesChownRequest, c *websocket.Conn) interface{} {
	pathToDir, err := verify(&r.File)
	if err != nil {
		return err
	}

	gid, err := strconv.Atoi(r.Gid)
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(r.Uid)
	if err != nil {
		return err
	}

	if gid > 0 && gid < 2^32 && uid > 0 && uid < 2^32 {
		err = os.Chown(pathToDir, uid, gid)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("gid/uid > 0 and < 2^32")
	}
	return nil
}

func Chmod(r FilesChmodRequest, c *websocket.Conn) interface{} {
	pathToDir, err := verify(&r.File)
	if err != nil {
		return err
	}
	mode64, err := strconv.ParseUint(r.Mode, 10, 64)
	if err != nil {
		return err
	}
	var mode32 fs.FileMode = fs.FileMode(mode64)
	if mode64 > 0 && mode64 < 777 {
		err = os.Chmod(pathToDir, mode32)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("mode string must be > 0 and < 777")
	}
	return nil
}

func Delete(r FilesDeleteRequest, c *websocket.Conn) interface{} {
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
	File dtos.PersistentFileRequestDto `json:"file"`
}

func FilesDownloadRequestExampleData() FilesDownloadRequest {
	return FilesDownloadRequest{
		File: dtos.PersistentFileDownloadDtoExampleData(),
	}
}

// files/upload POST
type FilesUploadRequest struct {
	File        dtos.PersistentFileRequestDto `json:"file"`
	SizeInBytes int64                         `json:"sizeInBytes"`
}

func FilesUploadRequestExampleData() FilesUploadRequest {
	return FilesUploadRequest{
		File:        dtos.PersistentFileUploadDtoExampleData(),
		SizeInBytes: 21217588,
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
		Folder: dtos.PersistentFileRequestNewFolderDtoExampleData(),
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
		pwd, _ := os.Getwd()
		dataRoot = fmt.Sprintf("%s/temp", pwd)
	}
	pathToFile := fmt.Sprintf("%s%s%s", dataRoot, data.Root, data.Path)
	return pathToFile, nil
}
