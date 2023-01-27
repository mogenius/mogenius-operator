package services

import (
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
)

func AllFiles() dtos.PersistentFileStatsDto {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return dtos.PersistentFileStatsDto{}
}

func List(r FilesListRequest) []dtos.PersistentFileDto {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return []dtos.PersistentFileDto{}
}

func Download(r FilesDownloadRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func Upload(r FilesUploadRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func Update(r FilesUpdateRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func CreateFolder(r FilesCreateFolderRequest) bool {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return false
}

func Rename(r FilesRenameRequest) bool {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return false
}

func Chown(r FilesChownRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func Chmod(r FilesChmodRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func Delete(r FilesDeleteRequest) bool {
	// TODO: Implement
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
		File: dtos.PersistentFileRequestDtoExampleData(),
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
