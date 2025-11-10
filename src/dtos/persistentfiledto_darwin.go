package dtos

import (
	"fmt"
	"io/fs"
	"mogenius-operator/src/utils"
	"os"
	"path/filepath"
	"syscall"
	"time"
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

func PersistentFileDtoFrom(rootDir string, path string) PersistentFileDto {
	info, err := os.Stat(path)
	if err != nil {
		fmt.Println("Darwin FileStatErr", err.Error())
		return PersistentFileDto{}
	}

	fileType := "file"
	if info.IsDir() {
		fileType = "directory"
	}

	var uid int = 0
	var gid int = 0
	var size int64 = 0
	var createTime = time.Now().Format(time.RFC3339)
	var modTime = time.Now().Format(time.RFC3339)
	var filemode fs.FileMode = info.Mode().Perm()
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		uid = int(stat.Uid)
		gid = int(stat.Gid)
		size = stat.Size
	}
	uidGid := fmt.Sprintf("%d:%d", uid, gid)

	relPath, _ := filepath.Rel(rootDir, path)

	return PersistentFileDto{
		Name:         info.Name(),
		Type:         fileType,
		RelativePath: relPath,
		Extension:    filepath.Ext(path),
		SizeInBytes:  size,
		Size:         utils.BytesToHumanReadable(size),
		Hash:         utils.QuickHash(path),
		CreatedAt:    createTime,
		ModifiedAt:   modTime,
		Uid_gid:      uidGid,
		Mode:         fmt.Sprintf("%o", filemode),
	}
}
