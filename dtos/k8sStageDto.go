package dtos

type K8sStageDto struct {
	Id              string          `json:"id" validate:"required"`
	DisplayName     string          `json:"displayName" validate:"required"`
	K8sName         string          `json:"k8sName" validate:"required"`
	Hostname        string          `json:"hostname" validate:"required"`
	StorageSizeInMb int             `json:"storageSizeInMb" validate:"required"`
	Services        []K8sServiceDto `json:"services" validate:"required"`
}

func K8sStageDtoExampleData() K8sStageDto {
	return K8sStageDto{
		Id:              "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		DisplayName:     "displayName",
		K8sName:         "k8sname",
		Hostname:        "hostname",
		StorageSizeInMb: 1,
		Services:        []K8sServiceDto{K8sServiceDtoExampleData()},
	}
}
