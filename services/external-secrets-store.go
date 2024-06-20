package services

import (
	"encoding/json"
	"fmt"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/utils"

	"strings"

	"github.com/mogenius/punq/logger"
)

const (
	ExternalSecretsSA = "mo-eso-serviceaccount"
)

type ExternalSecretStoreProps struct {
	CreateSecretsStoreRequest
	Name           string
	ServiceAccount string
}

func externalSecretStoreExample() *ExternalSecretStoreProps {
	return NewExternalSecretStore(CreateSecretsStoreRequestExample())
}

// NewExternalSecretStore creates a new NewExternalSecretStore with default values.
func NewExternalSecretStore(data CreateSecretsStoreRequest) *ExternalSecretStoreProps {
	return &ExternalSecretStoreProps{
		CreateSecretsStoreRequest: data,
		Name:                      data.NamePrefix + "-vault-secret-store",
		ServiceAccount:            ExternalSecretsSA,
	}
}

func CreateExternalSecretsStore(data CreateSecretsStoreRequest) CreateSecretsStoreResponse {
	props := NewExternalSecretStore(data)

	mokubernetes.CreateServiceAccount(props.ServiceAccount, utils.CONFIG.Kubernetes.OwnNamespace)
	// create the secret store which connects to the vault and is able to fetch secrets
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
	}
	// create the external secrets which will fetch all available secrets from vault
	// so that we can use them to offer them as UI options before binding them to a mogenius service
	externalSecretList, err := CreateExternalSecretList(ExternalSecretListProps{
		Project:         data.Project,
		SecretStoreName: props.Name,
		MoSharedPath:    data.MoSharedPath,
	})
	if err != nil {
		return CreateSecretsStoreResponse{
			Status: "ERROR",
		}
	} else {
		return CreateSecretsStoreResponse{
			Status: externalSecretList.Status,
		}
	}
}

func ListExternalSecretsStores() ListSecretsStoresResponse {
	// LIST
	response, err := mokubernetes.ListResources("external-secrets.io", "v1beta1", "clustersecretstores", "", true)
	if err != nil {
		logger.Log.Info("ListResources failed")
	}

	jsonOutput, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		fmt.Println(err)
		return ListSecretsStoresResponse{}
	}
	stores, err := parseSecretStoresListing(string(jsonOutput))
	if err != nil {
		fmt.Println(err)
		return ListSecretsStoresResponse{}
	}

	return ListSecretsStoresResponse{
		StoresInCluster: stores,
	}
}

func DeleteExternalSecretsStore(data DeleteSecretsStoreRequest) DeleteSecretsStoreResponse {

	err := mokubernetes.DeleteResource("external-secrets.io", "v1beta1", "clustersecretstores", data.Name, "", true)
	if err != nil {
		return DeleteSecretsStoreResponse{
			Status: "ERROR",
		}
	} else {
		return DeleteSecretsStoreResponse{
			Status: "SUCCESS",
		}
	}
}

func renderClusterSecretStore(yamlTemplateString string, props ExternalSecretStoreProps) string {
	yamlTemplateString = strings.Replace(yamlTemplateString, "<VAULT_STORE_NAME>", props.Name, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<MO_SHARED_PATH>", props.MoSharedPath, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<VAULT_SERVER_URL>", props.VaultServerUrl, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<ROLE>", props.Role, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<SERVICE_ACC>", props.ServiceAccount, -1)

	return yamlTemplateString
}

type SecretStoreListing struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Message string `json:"message"`
}

type SecretStoreListingSchema struct {
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

func parseSecretStoresListing(jsonStr string) ([]SecretStoreListing, error) {
	var secretStores SecretStoreListingSchema
	err := json.Unmarshal([]byte(jsonStr), &secretStores)
	if err != nil {
		return nil, err
	}

	var stores []SecretStoreListing
	for _, item := range secretStores.Items {
		statusMessage := ""
		if len(item.Status.Conditions) > 0 {
			statusMessage = item.Status.Conditions[0].Message
		}

		store := SecretStoreListing{
			Name:    item.Metadata.Name,
			Role:    item.Spec.Provider.Vault.Auth.Kubernetes.Role,
			Message: statusMessage,
		}
		stores = append(stores, store)
	}
	return stores, nil
}
