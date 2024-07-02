package services

import (
	"encoding/json"
	"fmt"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/utils"

	"strings"
)

const (
	SecretListSuffix = "vault-secret-list"
)

type ExternalSecretProps struct {
	CreateExternalSecretRequest
	SecretStoreName string
	secretPath      string
}

func externalExternalSecretExample() ExternalSecretProps {
	return NewExternalSecret(CreateExternalSecretRequestExample())
}

// NewExternalSecret creates a new NewExternalSecret with default values.
func NewExternalSecret(data CreateExternalSecretRequest) ExternalSecretProps {
	return ExternalSecretProps{
		CreateExternalSecretRequest: data,
	}
}

type ExternalSecretListProps struct {
	NamePrefix      string
	Project         string
	SecretStoreName string
	MoSharedPath    string
}

func externalSecretListExample() ExternalSecretListProps {
	return ExternalSecretListProps{
		NamePrefix:      "team-blue-secrets",
		Project:         "team-blue",
		SecretStoreName: "team-blue-secrets-" + SecretStoreSuffix,
		MoSharedPath:    "mogenius-external-secrets",
	}
}

func CreateExternalSecretList(props ExternalSecretListProps) error {
	return mokubernetes.ApplyResource(
		renderExternalSecretList(
			utils.InitExternalSecretListYaml(),
			props,
		),
		false,
	)
}

func CreateExternalSecret(data CreateExternalSecretRequest) CreateExternalSecretResponse {
	responsibleSecStore := getSecretStoreName(data.SecretStoreNamePrefix, data.ProjectName)

	secretPath, err := ReadSecretPathFromSecretStore(responsibleSecStore)
	if err != nil {
		return CreateExternalSecretResponse{
			Status:       "ERROR",
			ErrorMessage: fmt.Sprintf("Reading secret path from Secret Store failed. Err: %s", err.Error()),
		}
	}

	props := ExternalSecretProps{
		CreateExternalSecretRequest: data,
		SecretStoreName:             responsibleSecStore,
		secretPath:                  secretPath,
	}

	err = mokubernetes.ApplyResource(
		renderExternalSecret(
			utils.InitExternalSecretYaml(),
			props,
		),
		false,
	)
	if err != nil {
		return CreateExternalSecretResponse{
			Status:       "ERROR",
			ErrorMessage: fmt.Sprintf("Creating external secret failed. Err: %s", err.Error()),
		}
	} else {
		return CreateExternalSecretResponse{
			Status: "SUCCESS",
		}
	}
}

func DeleteExternalSecretList(namePrefix string, projectName string) error {
	return DeleteExternalSecret(getSecretListName(namePrefix, projectName))
}

func DeleteExternalSecret(name string) error {
	return mokubernetes.DeleteResource(
		"external-secrets.io",
		"v1beta1",
		"externalsecrets",
		name,
		utils.CONFIG.Kubernetes.OwnNamespace,
		false,
	)
}

func renderExternalSecretList(yamlTemplateString string, props ExternalSecretListProps) string {
	yamlTemplateString = strings.Replace(yamlTemplateString, "<NAME>", getSecretListName(props.NamePrefix, props.Project), -1)
	// the list of all available secrets for a project is only ever read by the operator
	yamlTemplateString = strings.Replace(yamlTemplateString, "<NAMESPACE>", utils.CONFIG.Kubernetes.OwnNamespace, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<SECRET_STORE_NAME>", props.SecretStoreName, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<MO_SHARED_PATH>", props.MoSharedPath, -1)

	return yamlTemplateString
}

func renderExternalSecret(yamlTemplateString string, props ExternalSecretProps) string {
	yamlTemplateString = strings.Replace(yamlTemplateString, "<NAME>", getSecretName(
		props.SecretStoreNamePrefix, props.ProjectName, props.ServiceName, props.PropertyName,
	), -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<SERVICE_NAME>", props.ServiceName, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<PROPERTY_FROM_SECRET>", props.PropertyName, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<NAMESPACE>", props.Namespace, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<SECRET_STORE_NAME>", props.SecretStoreName, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<SECRET_PATH>", props.secretPath, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<PROJECT>", props.ProjectName, -1)

	return yamlTemplateString
}

func getSecretName(namePrefix, project, service, propertyName string) string {
	return fmt.Sprintf("%s-%s-%s-%s",
		strings.ToLower(namePrefix),
		strings.ToLower(project),
		strings.ToLower(service),
		strings.ToLower(propertyName),
	)
}

func getSecretListName(customerPrefix string, projectName string) string {
	return fmt.Sprintf("%s-%s-%s",
		strings.ToLower(customerPrefix),
		strings.ToLower(projectName),
		strings.ToLower(SecretListSuffix),
	)
}

type ExternalSecretListing struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Message string `json:"message"`
}

type ExternalSecretListingSchema struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Provider struct {
				Vault struct {
					Auth struct {
						Kubernetes struct {
							Role string `json:"role"`
						} `json:"kubernetes"`
					} `json:"auth"`
				} `json:"vault"`
			} `json:"provider"`
		} `json:"spec"`
		Status struct {
			Conditions []struct {
				Message string `json:"message"`
			} `json:"conditions"`
		} `json:"status"`
	} `json:"items"`
}

func parseExternalSecretsListing(jsonStr string) ([]ExternalSecretListing, error) {
	var ExternalSecrets ExternalSecretListingSchema
	err := json.Unmarshal([]byte(jsonStr), &ExternalSecrets)
	if err != nil {
		return nil, err
	}

	var stores []ExternalSecretListing
	for _, item := range ExternalSecrets.Items {
		message := ""
		if len(item.Status.Conditions) > 0 {
			message = item.Status.Conditions[0].Message
		}
		store := ExternalSecretListing{
			Name:    item.Metadata.Name,
			Role:    item.Spec.Provider.Vault.Auth.Kubernetes.Role,
			Message: message,
		}
		stores = append(stores, store)
	}
	return stores, nil
}
