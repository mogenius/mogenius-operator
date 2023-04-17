package dtos

type PersistentFileRequestDto struct {
	Path            string `json:"path" validate:"required"`
	VolumeNamespace string `json:"volumeNamespace" validate:"required"`
	VolumeName      string `json:"volumeName" validate:"required"`
}

func PersistentFileRequestDtoExampleData() PersistentFileRequestDto {
	return PersistentFileRequestDto{
		Path:            "/",
		VolumeNamespace: "mogenius",
		VolumeName:      "my-fancy-volume-name",
	}
}

func PersistentFileRequestNewFolderDtoExampleData() PersistentFileRequestDto {
	return PersistentFileRequestDto{
		Path:            "/blaaaa",
		VolumeNamespace: "mogenius",
		VolumeName:      "my-fancy-volume-name",
	}
}

func PersistentFileDownloadDtoExampleData() PersistentFileRequestDto {
	return PersistentFileRequestDto{
		Path:            "/README.md",
		VolumeNamespace: "mogenius",
		VolumeName:      "my-fancy-volume-name",
	}
}

func PersistentFileUploadDtoExampleData() PersistentFileRequestDto {
	return PersistentFileRequestDto{
		Path:            "/",
		VolumeNamespace: "mogenius",
		VolumeName:      "my-fancy-volume-name",
	}
}
