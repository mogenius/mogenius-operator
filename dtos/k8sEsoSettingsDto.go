package dtos

type K8sEsoSettingsDto struct {
	SecretStoreNamePrefix string `json:"secretStoreNamePrefix"`
	ProjectName           string `json:"projectName"`
}

type K8sEsoDeleteSettingDto struct {
	SecretStoreNamePrefix string `json:"secretStoreNamePrefix"`
	ProjectName           string `json:"projectName"`
	KeyName               string `json:"keyName"`
}
