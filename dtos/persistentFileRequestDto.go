package dtos

type PersistentFileRequestDto struct {
	Root      string `json:"root" validate:"required"`
	Path      string `json:"path" validate:"required"`
	ClusterId string `json:"clusterId,omitempty"`
}

func PersistentFileRequestDtoExampleData() PersistentFileRequestDto {
	return PersistentFileRequestDto{
		Root:      "root",
		Path:      "path",
		ClusterId: "clusterId",
	}
}
