package services

import (
	"encoding/json"
	"fmt"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/utils"

	"strings"

	"github.com/mogenius/punq/logger"
)

type ExternalSecretProps struct {
	CreateExternalSecretRequest
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
	Project         string
	SecretStoreName string
	MoSharedPath    string
}

func CreateExternalSecretList(ExternalSecretListProps) (CreateExternalSecretResponse, error) {

	return CreateExternalSecretResponse{
		Status: "SUCCESS",
	}, nil
}

func CreateExternalSecret(data CreateExternalSecretRequest) CreateSecretsStoreResponse {

	props := NewExternalSecret(data)

	mokubernetes.CreateServiceAccount(props.ServiceAccount, utils.CONFIG.Kubernetes.OwnNamespace)

	err := mokubernetes.ApplyResource(
		renderClusterExternalSecret(
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

func ListExternalSecretsStores() ListSecretsStoresResponse {
	// LIST
	response, err := mokubernetes.ListResources("external-secrets.io", "v1beta1", "clusterExternalSecrets", "", true)
	if err != nil {
		logger.Log.Info("ListResources failed")
	}

	jsonOutput, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		fmt.Println(err)
		return ListSecretsStoresResponse{}
	}
	stores, err := parseExternalSecretsListing(string(jsonOutput))
	if err != nil {
		fmt.Println(err)
		return ListSecretsStoresResponse{}
	}

	return ListSecretsStoresResponse{
		StoresInCluster: stores,
	}
}

func DeleteExternalSecretsStore(data DeleteSecretsStoreRequest) DeleteSecretsStoreResponse {

	err := mokubernetes.DeleteResource("external-secrets.io", "v1beta1", "clusterExternalSecrets", data.Name, "", true)
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

func renderClusterExternalSecret(yamlTemplateString string, props ExternalExternalSecretProps) string {
	yamlTemplateString = strings.Replace(yamlTemplateString, "<VAULT_STORE_NAME>", props.Name, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<MO_SHARED_PATH>", props.MoSharedPath, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<VAULT_SERVER_URL>", props.VaultServerUrl, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<ROLE>", props.Role, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<SERVICE_ACC>", props.ServiceAccount, -1)

	return yamlTemplateString
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
		store := ExternalSecretListing{
			Name:    item.Metadata.Name,
			Role:    item.Spec.Provider.Vault.Auth.Kubernetes.Role,
			Message: item.Status.Conditions[0].Message,
		}
		stores = append(stores, store)
	}
	return stores, nil
}
