package dtos

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
