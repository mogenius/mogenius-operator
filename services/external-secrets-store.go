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
	SecretStoreSuffix = "-vault-secret-store"
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
		Name:                      data.NamePrefix + SecretStoreSuffix,
		ServiceAccount:            ExternalSecretsSA,
	}
}

func CreateExternalSecretsStore(data CreateSecretsStoreRequest) CreateSecretsStoreResponse {
	props := NewExternalSecretStore(data)

	err := mokubernetes.CreateServiceAccount(props.ServiceAccount, utils.CONFIG.Kubernetes.OwnNamespace)
	if err != nil {
		logger.Log.Info("CreateServiceAccount apply failed")
		return CreateSecretsStoreResponse{
			Status: "ERROR",
		}
	}

	// create the secret store which connects to the vault and is able to fetch secrets
	err = mokubernetes.ApplyResource(
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
	externalSecretList := CreateExternalSecretList(ExternalSecretListProps{
		NamePrefix:      props.NamePrefix,
		Project:         props.Project,
		SecretStoreName: props.Name,
		MoSharedPath:    props.MoSharedPath,
	})
	return CreateSecretsStoreResponse(externalSecretList)
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
	// delerte the external secrets list
	err := mokubernetes.DeleteResource("external-secrets.io", "v1beta1", "externalsecrets", getSecretListName(data.NamePrefix, data.Project), utils.CONFIG.Kubernetes.OwnNamespace, false)
	if err != nil {
		return DeleteSecretsStoreResponse{
			Status: "ERROR",
		}
	}
	// delete the secret store
	err = mokubernetes.DeleteResource("external-secrets.io", "v1beta1", "clustersecretstores", data.Name, "", true)
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
	// secret stores are currently bound to the project settings
	yamlTemplateString = strings.Replace(yamlTemplateString, "<MO_SHARED_PATH_COMBINED>", getMoSharedPath(props.MoSharedPath, props.Project), -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<VAULT_SERVER_URL>", props.VaultServerUrl, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<ROLE>", props.Role, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<SERVICE_ACC>", props.ServiceAccount, -1)

	return yamlTemplateString
}

func getMoSharedPath(moSharedPath string, project string) string {
	return fmt.Sprintf("%s/%s", moSharedPath, project)
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
