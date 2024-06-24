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
	MoSharedPath    string
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
		SecretStoreName: "team-blue-secrets" + SecretStoreSuffix,
		MoSharedPath:    "mogenius-external-secrets",
	}
}

func CreateExternalSecretList(props ExternalSecretListProps) CreateExternalSecretResponse {
	err := mokubernetes.ApplyResource(
		renderExternalSecretList(
			utils.InitExternalSecretListYaml(),
			props,
		),
		false,
	)
	if err != nil {
		return CreateExternalSecretResponse{
			Status:       "ERROR",
			ErrorMessage: err.Error(),
		}
	} else {
		return CreateExternalSecretResponse{
			Status: "SUCCESS",
		}
	}
}

// func CreateExternalSecret(data CreateExternalSecretRequest) CreateSecretsStoreResponse {

// 	props := NewExternalSecret(data)

// 	mokubernetes.CreateServiceAccount(props.ServiceAccount, utils.CONFIG.Kubernetes.OwnNamespace)

// 	err := mokubernetes.ApplyResource(
// 		renderClusterExternalSecret(
// 			utils.InitExternalSecretsStoreYaml(),
// 			*props,
// 		),
// 		true,
// 	)
// 	if err != nil {
// 		return CreateSecretsStoreResponse{
// 			Status: "ERROR",
// 		}
// 	} else {
// 		return CreateSecretsStoreResponse{
// 			Status: "SUCCESS",
// 		}
// 	}
// }

// func ListExternalSecretsStores() ListSecretsStoresResponse {
// 	// LIST
// 	response, err := mokubernetes.ListResources("external-secrets.io", "v1beta1", "clusterExternalSecrets", "", true)
// 	if err != nil {
// 		logger.Log.Info("ListResources failed")
// 	}

// 	jsonOutput, err := json.MarshalIndent(response, "", "  ")
// 	if err != nil {
// 		fmt.Println(err)
// 		return ListSecretsStoresResponse{}
// 	}
// 	stores, err := parseExternalSecretsListing(string(jsonOutput))
// 	if err != nil {
// 		fmt.Println(err)
// 		return ListSecretsStoresResponse{}
// 	}

// 	return ListSecretsStoresResponse{
// 		StoresInCluster: stores,
// 	}
// }

// func DeleteExternalSecretsStore(data DeleteSecretsStoreRequest) DeleteSecretsStoreResponse {

// 	err := mokubernetes.DeleteResource("external-secrets.io", "v1beta1", "clusterExternalSecrets", data.Name, "", true)
// 	if err != nil {
// 		return DeleteSecretsStoreResponse{
// 			Status: "ERROR",
// 		}
// 	} else {
// 		return DeleteSecretsStoreResponse{
// 			Status: "SUCCESS",
// 		}
// 	}
// }

func renderExternalSecretList(yamlTemplateString string, props ExternalSecretListProps) string {
	yamlTemplateString = strings.Replace(yamlTemplateString, "<NAME>", getSecretListName(props.NamePrefix, props.Project), -1)
	// the list of all available secrets for a project is only ever read by the operator
	yamlTemplateString = strings.Replace(yamlTemplateString, "<NAMESPACE>", utils.CONFIG.Kubernetes.OwnNamespace, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<SECRET_STORE_NAME>", props.SecretStoreName, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<MO_SHARED_PATH>", props.MoSharedPath, -1)

	return yamlTemplateString
}

func renderExternalSecret(yamlTemplateString string, props ExternalSecretProps) string {
	yamlTemplateString = strings.Replace(yamlTemplateString, "<NAME>", props.Name, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<NAMESPACE>", props.Namespace, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<SECRET_STORE_NAME>", props.SecretStoreName, -1)
	yamlTemplateString = strings.Replace(yamlTemplateString, "<MO_SHARED_PATH>", props.MoSharedPath, -1)

	return yamlTemplateString
}

func getSecretListName(customerPrefix string, project string) string {
	return fmt.Sprintf("%s-%s-%s", customerPrefix, project, SecretListSuffix)
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
