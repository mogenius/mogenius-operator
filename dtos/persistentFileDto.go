package dtos

import (
	"fmt"
	"io/fs"
	"mogenius-k8s-manager/logger"
	"os"
	"path/filepath"
	"syscall"
	"time"

	punq "github.com/mogenius/punq/utils"
)

type PersistentFileDto struct {
	Name         string `json:"name" validate:"required"`
	Type         string `json:"type" validate:"required"` // "directory", "file"
	RelativePath string `json:"relativePath" validate:"required"`
	Extension    string `json:"extension,omitempty"`
	SizeInBytes  int64  `json:"sizeInBytes" validate:"required"`
	Size         string `json:"size" validate:"required"`
	Hash         string `json:"hash" validate:"required"`
	MimeType     string `json:"mimeType,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
	CreatedAt    string `json:"createdAt,omitempty"`
	ModifiedAt   string `json:"modifiedAt,omitempty"`
	Uid_gid      string `json:"uid_gid,omitempty"`
	Mode         string `json:"mode,omitempty"`
}

func PersistentFileDtoExampleData() PersistentFileDto {
	return PersistentFileDto{
		Name:         "name",
		Type:         "directory",
		RelativePath: "relativePath",
		SizeInBytes:  1,
		Size:         "size",
		Hash:         "hash",
		CreatedAt:    time.Now().Format(time.RFC3339),
		ModifiedAt:   time.Now().Format(time.RFC3339),
		Uid_gid:      "uid_gid",
		Mode:         "123",
	}
}

func PersistentFileDtoFrom(rootDir string, path string, d fs.DirEntry) PersistentFileDto {
	fileType := "file"
	if d.IsDir() {
		fileType = "directory"
	}

	info, err := os.Stat(path)
	if err != nil {
		logger.Log.Warningf("FileStat Err: %s", err.Error())
	}
	var uid int = 0
	var gid int = 0
	var size int64 = 0
	var createTime = time.Now().Format(time.RFC3339)
	var modTime = time.Now().Format(time.RFC3339)
	var filemode fs.FileMode = 0
	//var mode = info.Mode().Perm()
	if err == nil {
		filemode = info.Mode().Perm()
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			uid = int(stat.Uid)
			gid = int(stat.Gid)
			size = stat.Size
			//mode = int(stat.Mode)
			// TODO: https://stackoverflow.com/questions/53266940/get-change-date-of-a-folder-in-go
			// RESULTS IN: stat.Ctimespec undefined (type *syscall.Stat_t has no field or method Ctimespec)
			//createTime = time.Unix(stat.Ctimespec.Sec, 0).Format(time.RFC3339)
			//modTime = time.Unix(stat.Mtimespec.Sec, 0).Format(time.RFC3339)
		}
	}
	uidGid := fmt.Sprintf("%d:%d", uid, gid)

	relPath, _ := filepath.Rel(rootDir, path)

	return PersistentFileDto{
		Name:         d.Name(),
		Type:         fileType,
		RelativePath: relPath,
		Extension:    filepath.Ext(path),
		SizeInBytes:  size,
		Size:         punq.BytesToHumanReadable(size),
		Hash:         punq.QuickHash(path),
		CreatedAt:    createTime,
		ModifiedAt:   modTime,
		Uid_gid:      uidGid,
		Mode:         fmt.Sprintf("%o", filemode),
	}
}
