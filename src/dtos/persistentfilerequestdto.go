package dtos

type PersistentFileRequestDto struct {
	Path            string `json:"path" validate:"required"`
	VolumeNamespace string `json:"volumeNamespace" validate:"required"`
	VolumeName      string `json:"volumeName" validate:"required"`
}
