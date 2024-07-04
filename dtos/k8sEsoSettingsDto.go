package dtos

type K8sEsoSettingsDto struct {
	SecretStoreNamePrefix string `json:"secretStoreNamePrefix"`
	ProjectName           string `json:"projectName"`
}
