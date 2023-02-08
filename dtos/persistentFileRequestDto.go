package dtos

type PersistentFileRequestDto struct {
	Root      string `json:"root" validate:"required"`
	Path      string `json:"path" validate:"required"`
	ClusterId string `json:"clusterId,omitempty"`
}

func PersistentFileRequestDtoExampleData() PersistentFileRequestDto {
	return PersistentFileRequestDto{
		Root:      "/",
		Path:      "/",
		ClusterId: "clusterId",
	}
}

func PersistentFileRequestNewFolderDtoExampleData() PersistentFileRequestDto {
	return PersistentFileRequestDto{
		Root:      "/",
		Path:      "/blaaaa",
		ClusterId: "clusterId",
	}
}

func PersistentFileDownloadDtoExampleData() PersistentFileRequestDto {
	return PersistentFileRequestDto{
		Root:      "/",
		Path:      "/README.md",
		ClusterId: "clusterId",
	}
}

func PersistentFileUploadDtoExampleData() PersistentFileRequestDto {
	return PersistentFileRequestDto{
		Root:      "/",
		Path:      "/",
		ClusterId: "clusterId",
	}
}
