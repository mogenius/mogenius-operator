package kubernetes

import (
	"mogenius-k8s-manager/utils"
	"strings"
)

type ExternalSecretStoreProps struct {
	Role           string
	VaultServerUrl string
	MoSharedPath   string
	ServiceAccount string
}

// NewExternalSecretStore creates a new NewExternalSecretStore with default values.
func NewExternalSecretStore() *ExternalSecretStoreProps {
	return &ExternalSecretStoreProps{
		Role:           "mogenius-external-secrets",
		VaultServerUrl: "http://vault.default.svc.cluster.local:8200",
		MoSharedPath:   "secret/mogenius-external-secrets",
		ServiceAccount: "external-secrets-sa",
	}
}

func CreateExternalSecretsStore() {
	props := NewExternalSecretStore()

	ApplyResource(
		renderClusterSecretStore(
			utils.InitExternalSecretsStoreYaml(),
			*props,
		),
		true,
	)
}

func renderClusterSecretStore(yamlTemplateString string, props ExternalSecretStoreProps) string {
	yamlTemplateString = strings.Replace(yamlTemplateString, "<MO_SHARED_PATH>", props.MoSharedPath, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<VAULT_SERVER_URL>", props.VaultServerUrl, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<ROLE>", props.Role, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<SERVICE_ACC>", props.ServiceAccount, -1)

	return yamlTemplateString
}
