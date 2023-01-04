package dtos

type K8sStageDto struct {
	Id              string          `json:"id" validate:"required"`
	DisplayName     string          `json:"displayName" validate:"required"`
	K8sName         string          `json:"k8sName" validate:"required"`
	Hostname        string          `json:"hostname" validate:"required"`
	StorageSizeInMb int             `json:"storageSizeInMb" validate:"required"`
	Services        []K8sServiceDto `json:"services" validate:"required"`
}
