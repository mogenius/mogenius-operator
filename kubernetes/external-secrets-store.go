package kubernetes

import (
	"encoding/json"
	"fmt"
)

func ListExternalSecretsStores(ProjectId string) ([]SecretStore, error) {
	response, err := ListResources("external-secrets.io", "v1beta1", "clustersecretstores", "", true)
	if err != nil {
		K8sLogger.Info("ListResources failed")
	}

	jsonOutput, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, err
	}
	stores, err := parseSecretStoresListing(string(jsonOutput))
	if err != nil {
		return nil, err
	}
	filteredStores := []SecretStore{}
	for _, store := range stores {
		if store.ProjectId == ProjectId {
			filteredStores = append(filteredStores, store)
		}
	}

	return filteredStores, nil
}

func GetExternalSecretsStore(name string) (*SecretStore, error) {
	response, err := GetResource("external-secrets.io", "v1beta1", "clustersecretstores", name, "", true)
	if err != nil {
		K8sLogger.Info("GetResource failed for SecretStore: " + name)
		return nil, err
	}

	K8sLogger.Info(fmt.Sprintf("SecretStore retrieved name: %s", response.GetName()))

	jsonOutput, err := json.Marshal(response.Object)
	if err != nil {
		return nil, err
	}
	secretStore := SecretStoreSchema{}
	err = json.Unmarshal([]byte(jsonOutput), &secretStore)
	if err != nil {
		return nil, err
	}
	return &SecretStore{
		Name:       secretStore.Metadata.Name,
		Prefix:     secretStore.Metadata.Annotations.Prefix,
		ProjectId:  secretStore.Metadata.Annotations.ProjectId,
		SharedPath: secretStore.Metadata.Annotations.SharedPath,
		Role:       secretStore.Spec.Provider.Vault.Auth.Kubernetes.Role,
		VaultURL:   secretStore.Spec.Provider.Vault.Server,
	}, err
}

func ReadSecretPathFromSecretStore(name string) (string, error) {
	secretStore, err := GetExternalSecretsStore(name)
	if err != nil {
		return "", err
	}
	return secretStore.SharedPath, nil
}

type SecretStoreListingSchema struct {
	Items []struct {
		SecretStoreSchema
	} `json:"items"`
}
type SecretStoreSchema struct {
	Metadata struct {
		Name        string `json:"name"`
		Annotations struct {
			Prefix     string `json:"mogenius-external-secrets/prefix"`
			SharedPath string `json:"mogenius-external-secrets/shared-path"`
			ProjectId  string `json:"mogenius-external-secrets/project-id"`
		} `json:"annotations"`
	} `json:"metadata"`
	Spec struct {
		Provider struct {
			Vault struct {
				Server string `json:"server"`
				Auth   struct {
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
}

func parseSecretStoresListing(jsonStr string) ([]SecretStore, error) {
	var secretStores SecretStoreListingSchema
	err := json.Unmarshal([]byte(jsonStr), &secretStores)
	if err != nil {
		return nil, err
	}

	var stores = []SecretStore{}
	for _, item := range secretStores.Items {
		stores = append(stores, SecretStore{
			Name:       item.Metadata.Name,
			Prefix:     item.Metadata.Annotations.Prefix,
			ProjectId:  item.Metadata.Annotations.ProjectId,
			SharedPath: item.Metadata.Annotations.SharedPath,
			Role:       item.Spec.Provider.Vault.Auth.Kubernetes.Role,
			VaultURL:   item.Spec.Provider.Vault.Server,
		})
	}
	return stores, nil
}

type SecretStore struct {
	Name       string `json:"name"`
	Prefix     string `json:"prefix"`
	ProjectId  string `json:"project-id"`
	SharedPath string `json:"shared-path"`
	Role       string `json:"role"`
	VaultURL   string `json:"vault-url"`
}
