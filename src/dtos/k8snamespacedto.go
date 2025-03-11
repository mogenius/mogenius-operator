package dtos

type K8sNamespaceDto struct {
	Id          string `json:"id" validate:"required"`
	DisplayName string `json:"displayName" validate:"required"`
	Name        string `json:"Name" validate:"required"`
}
