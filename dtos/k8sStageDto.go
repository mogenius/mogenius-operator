package dtos

type K8sNamespaceDto struct {
	Id                string          `json:"id" validate:"required"`
	DisplayName       string          `json:"displayName" validate:"required"`
	Name              string          `json:"Name" validate:"required"`
	StorageSizeInMb   int             `json:"storageSizeInMb" validate:"required"`
	Services          []K8sServiceDto `json:"services" validate:"required"`
	CloudflareProxied bool            `json:"cloudflareProxied"`
}

func K8sNamespaceDtoExampleData() K8sNamespaceDto {
	return K8sNamespaceDto{
		Id:                "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		DisplayName:       "displayName",
		Name:              "name",
		StorageSizeInMb:   1028,
		Services:          []K8sServiceDto{K8sServiceDtoExampleData()},
		CloudflareProxied: false,
	}
}
