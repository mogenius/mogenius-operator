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
	ExternalSecretsSA     = "mo-eso-serviceaccount"
	SecretStoreSuffix     = "vault-secret-store"
	StoreAnnotationPrefix = "used-by-mogenius/"
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
		Name:                      getSecretStoreName(data.NamePrefix, data.Project),
		ServiceAccount:            getServiceAccountName(data.MoSharedPath),
	}
}

func CreateExternalSecretsStore(data CreateSecretsStoreRequest) CreateSecretsStoreResponse {
	props := NewExternalSecretStore(data)

	// create unique service account tag per project
	annotations := make(map[string]string)
	key := fmt.Sprintf("%s%s", StoreAnnotationPrefix, data.Project)
	annotations[key] = fmt.Sprintf("Used to read secrets from vault path: %s", data.MoSharedPath)

	err := mokubernetes.ApplyServiceAccount(props.ServiceAccount, utils.CONFIG.Kubernetes.OwnNamespace, annotations)
	if err != nil {
		logger.Log.Info("ServiceAccount apply failed")
		return CreateSecretsStoreResponse{
			Status:       "ERROR",
			ErrorMessage: err.Error(),
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
			Status:       "ERROR",
			ErrorMessage: err.Error(),
		}
	}
	// create the external secret which will fetch all available secrets from vault
	// so that we can use them to offer them as UI options before binding them to a mogenius service
	err = CreateExternalSecretList(ExternalSecretListProps{
		NamePrefix:      props.NamePrefix,
		Project:         props.Project,
		SecretStoreName: props.Name,
		MoSharedPath:    props.MoSharedPath,
	})
	if err != nil {
		return CreateSecretsStoreResponse{
			Status:       "ERROR",
			ErrorMessage: err.Error(),
		}
	}
	return CreateSecretsStoreResponse{
		Status: "SUCCESS",
	}
}

func GetExternalSecretsStore(name string) (SecretStoreSchema, error) {
	response, err := mokubernetes.GetResource("external-secrets.io", "v1beta1", "clustersecretstores", name, "", true)
	if err != nil {
		logger.Log.Info(fmt.Sprintf("SecretStore retrieved name: %s", response.GetName()))
	} else {
		logger.Log.Info("GetResource failed for SecretStore: " + name)
	}
	jsonOutput, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return SecretStoreSchema{}, err
	}
	secretStore := SecretStoreSchema{}
	err = json.Unmarshal([]byte(string(jsonOutput)), &secretStore)
	if err != nil {
		return SecretStoreSchema{}, err
	}
	return secretStore, err
}

func ReadSecretPathFromSecretStore(name string) (string, error) {
	secretStore, err := GetExternalSecretsStore(name)
	if err != nil {
		return "", err
	}
	return secretStore.Metadata.Annotations.SharedPath, nil
}

func ListExternalSecretsStores() ListSecretsStoresResponse {
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

func ListAvailableExternalSecrets() ListSecretsResponse {
	// LIST
	//TODO implement
	return ListSecretsResponse{}
}

func DeleteExternalSecretsStore(data DeleteSecretsStoreRequest) DeleteSecretsStoreResponse {
	// delete the external secrets list
	err := DeleteExternalSecretList(data.NamePrefix, data.Project)
	if err != nil {
		return DeleteSecretsStoreResponse{
			Status:       "ERROR",
			ErrorMessage: err.Error(),
		}
	}
	// delete the secret store
	err = mokubernetes.DeleteResource("external-secrets.io", "v1beta1", "clustersecretstores", getSecretStoreName(data.NamePrefix, data.Project), "", true)
	if err != nil {
		return DeleteSecretsStoreResponse{
			Status:       "ERROR",
			ErrorMessage: err.Error(),
		}
	}
	// delete the service account if it has no annotations from another SecretStore
	err = deleteUnusedServiceAccount(data)
	if err != nil {
		return DeleteSecretsStoreResponse{
			Status:       "ERROR",
			ErrorMessage: err.Error(),
		}
	}
	return DeleteSecretsStoreResponse{
		Status: "SUCCESS",
	}
}

func deleteUnusedServiceAccount(data DeleteSecretsStoreRequest) error {
	serviceAccount, err := mokubernetes.GetServiceAccount(getServiceAccountName(data.MoSharedPath), utils.CONFIG.Kubernetes.OwnNamespace)
	if err != nil {
		return err
	} else {
		logger.Log.Info(fmt.Sprintf("ServiceAccount retrieved ns: %s - name: %s", serviceAccount.GetNamespace(), serviceAccount.GetName()))
	}
	if serviceAccount.Annotations != nil {
		// remove current claim of using this service account
		removeKey := ""
		for key := range serviceAccount.Annotations {
			myKey := fmt.Sprintf("%s%s", StoreAnnotationPrefix, data.Project)
			if key == myKey {
				removeKey = key
			}
		}
		if removeKey != "" {
			delete(serviceAccount.Annotations, removeKey)
		}

		// check if there are any other claims
		canBeDeleted := true
		for key := range serviceAccount.Annotations {
			if strings.HasPrefix(key, StoreAnnotationPrefix) {
				canBeDeleted = false
			}
		}
		if canBeDeleted {
			// no annotations left that indicate other usage, delete the sa
			err = mokubernetes.DeleteServiceAccount(serviceAccount.Name, serviceAccount.Namespace)
			if err != nil {
				return err
			}
		} else {
			// there are still claims, don't delete the service account
			err = mokubernetes.UpdateServiceAccount(serviceAccount)
			if err != nil {
				return err
			}
		}
	}
	return nil
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

func getServiceAccountName(moSharedPath string) string {
	return fmt.Sprintf("%s-%s", ExternalSecretsSA, moSharedPath)
}

func getMoSharedPath(moSharedPath string, project string) string {
	return fmt.Sprintf("%s/%s", moSharedPath, project)
}

func getSecretStoreName(namePrefix string, project string) string {
	return fmt.Sprintf("%s-%s-%s", namePrefix, project, SecretStoreSuffix)
}

type SecretStoreListing struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Message string `json:"message"`
}

type SecretListing struct {
	SecretKey string `json:"secretKey"`
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

type SecretStoreSchema struct {
	Metadata struct {
		Annotations struct {
			SharedPath string `yaml:"mogenius-external-secrets/shared-path"`
		} `yaml:"annotations"`
	} `yaml:"metadata"`
}
