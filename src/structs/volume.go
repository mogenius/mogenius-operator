package structs

type Volume struct {
	Namespace  string `json:"namespace"`
	VolumeName string `json:"volumeName"`
	SizeInGb   int    `json:"sizeInGb"`
}
