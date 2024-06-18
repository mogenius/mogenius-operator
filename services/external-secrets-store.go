package services

import (
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/utils"

	"strings"
)

type ExternalSecretStoreProps struct {
	CreateSecretsStoreRequest
	ServiceAccount string
}

// NewExternalSecretStore creates a new NewExternalSecretStore with default values.
func NewExternalSecretStore() *ExternalSecretStoreProps {

	return &ExternalSecretStoreProps{
		CreateSecretsStoreRequest: CreateSecretsStoreRequestExample(),
		ServiceAccount:            "external-secrets-sa",
	}
}

func CreateExternalSecretsStore(data CreateSecretsStoreRequest) CreateSecretsStoreResponse {
	props := NewExternalSecretStore()

	mokubernetes.CreateServiceAccount(props.ServiceAccount, utils.CONFIG.Kubernetes.OwnNamespace)

	err := mokubernetes.ApplyResource(
		renderClusterSecretStore(
			utils.InitExternalSecretsStoreYaml(),
			*props,
		),
		true,
	)
	if err != nil {
		return CreateSecretsStoreResponse{
			Status: "ERROR",
		}
	} else {
		return CreateSecretsStoreResponse{
			Status: "SUCCESS",
		}
	}
}

func renderClusterSecretStore(yamlTemplateString string, props ExternalSecretStoreProps) string {
	yamlTemplateString = strings.Replace(yamlTemplateString, "<MO_SHARED_PATH>", props.MoSharedPath, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<VAULT_SERVER_URL>", props.VaultServerUrl, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<ROLE>", props.Role, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<SERVICE_ACC>", props.ServiceAccount, -1)

	return yamlTemplateString
}
