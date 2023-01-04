package dtos

type PersistentFileRequestDto struct {
	Root      string `json:"root" validate:"required"`
	Path      string `json:"path" validate:"required"`
	ClusterId string `json:"clusterId,omitempty"`
}
