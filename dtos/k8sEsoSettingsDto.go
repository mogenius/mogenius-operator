package dtos

type K8sEsoSettingsDto struct {
	SecretStoreNamePrefix string `json:"secretStoreNamePrefix" validate:"required"`
}
