package dtos

// PvcFileRequestDto addresses a path on any PVC that is mounted by a running
// pod (files/v2/* patterns). Path is relative to the PVC's mount root.
type PvcFileRequestDto struct {
	Namespace string `json:"namespace" validate:"required"`
	PvcName   string `json:"pvcName" validate:"required"`
	Path      string `json:"path" validate:"required"`
}
