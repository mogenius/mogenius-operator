package dtos

import "time"

type PersistentFileDto struct {
	Name           string `json:"name" validate:"required"`
	Type           string `json:"type" validate:"required"` // "directory", "file"
	RelativePath   string `json:"relativePath" validate:"required"`
	AbsolutePath   string `json:"absolutePath" validate:"required"`
	IsSymbolicLink bool   `json:"isSymbolicLink" validate:"required"`
	Extension      string `json:"extension,omitempty"`
	SizeInBytes    int    `json:"sizeInBytes" validate:"required"`
	Size           string `json:"size" validate:"required"`
	Hash           string `json:"hash" validate:"required"`
	MimeType       string `json:"mimeType,omitempty"`
	ContentType    string `json:"contentType,omitempty"`
	CreatedAt      string `json:"createdAt,omitempty"`
	ModifiedAt     string `json:"modifiedAt,omitempty"`
	Uid_gid        string `json:"uid_gid,omitempty"`
	Mode           int    `json:"mode,omitempty"`
	New            bool   `json:"new,omitempty"`
}

func PersistentFileDtoExampleData() PersistentFileDto {
	return PersistentFileDto{
		Name:           "name",
		Type:           "directory",
		RelativePath:   "relativePath",
		AbsolutePath:   "absolutePath",
		IsSymbolicLink: true,
		SizeInBytes:    1,
		Size:           "size",
		Hash:           "hash",
		CreatedAt:      time.Now().Format(time.RFC3339),
		ModifiedAt:     time.Now().Format(time.RFC3339),
		Uid_gid:        "uid_gid",
		Mode:           1,
		New:            true,
	}
}
