package services

import "mogenius-k8s-manager/dtos"

func AllFiles() dtos.PersistentFileStatsDto {
	// TODO: Implement
	return dtos.PersistentFileStatsDto{}
}

func List(r FilesListRequest) []dtos.PersistentFileDto {
	// TODO: Implement
	return []dtos.PersistentFileDto{}
}

func Download(r FilesDownloadRequest) interface{} {
	// TODO: Implement
	return nil
}

func Upload(r FilesUploadRequest) interface{} {
	// TODO: Implement
	return nil
}

func Update(r FilesUpdateRequest) interface{} {
	// TODO: Implement
	return nil
}

func CreateFolder(r FilesCreateFolderRequest) bool {
	// TODO: Implement
	return false
}

func Rename(r FilesRenameRequest) bool {
	// TODO: Implement
	return false
}

func Chown(r FilesChownRequest) interface{} {
	// TODO: Implement
	return nil
}

func Chmod(r FilesChmodRequest) interface{} {
	// TODO: Implement
	return nil
}

func Delete(r FilesDeleteRequest) bool {
	// TODO: Implement
	return false
}

// files/storage-stats GET

// files/list POST
type FilesListRequest struct {
	Folder dtos.PersistentFileRequestDto `json:"folder"`
}

// files/download POST
type FilesDownloadRequest struct {
	File dtos.PersistentFileRequestDto `json:"file"` // TODO: how? before go we simply used the response to write binary stream
}

// files/upload POST
type FilesUploadRequest struct {
	File        dtos.PersistentFileRequestDto `json:"file"`
	NewFileData string                        `json:"newFileData"` // TODO: base64? was originally Multer File Upload
}

// files/update PATCH
type FilesUpdateRequest struct {
	File        dtos.PersistentFileRequestDto `json:"file"`
	NewFileData string                        `json:"newFileData"` // TODO: base64? was originally Multer File Upload
}

// files/create-folder POST
type FilesCreateFolderRequest struct {
	Folder dtos.PersistentFileRequestDto `json:"folder"`
}

// files/rename POST
type FilesRenameRequest struct {
	File    dtos.PersistentFileRequestDto `json:"file"`
	NewName string                        `json:"newName"`
}

// files/chown POST
type FilesChownRequest struct {
	File dtos.PersistentFileRequestDto `json:"file"`
	Uid  string                        `json:"uid"`
	Gid  string                        `json:"gid"`
}

// files/chmod POST
type FilesChmodRequest struct {
	File dtos.PersistentFileRequestDto `json:"file"`
	Mode string                        `json:"mode"`
}

// files/delete POST
type FilesDeleteRequest struct {
	File dtos.PersistentFileRequestDto `json:"file"`
}
