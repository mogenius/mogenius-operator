package kubernetes

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mogenius/punq/logger"
	"gopkg.in/yaml.v2"
)

func ListExternalSecretsStores(projectName string) ([]string, error) {
	response, err := ListResources("external-secrets.io", "v1beta1", "clustersecretstores", "", true)
	if err != nil {
		logger.Log.Info("ListResources failed")
	}

	jsonOutput, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, err
	}
	stores, err := parseSecretStoresListing(string(jsonOutput))
	if err != nil {
		return nil, err
	}
	filteredStores := []string{}
	for _, store := range stores {
		if strings.Contains(store, projectName) {
			filteredStores = append(filteredStores, store)
		}
	}

	return stores, nil
}

func GetExternalSecretsStore(name string) (*SecretStoreSchema, error) {
	response, err := GetResource("external-secrets.io", "v1beta1", "clustersecretstores", name, "", true)
	if err != nil {
		logger.Log.Info("GetResource failed for SecretStore: " + name)
		return nil, err
	}

	logger.Log.Info(fmt.Sprintf("SecretStore retrieved name: %s", response.GetName()))

	yamlOutput, err := yaml.Marshal(response.Object)
	if err != nil {
		return nil, err
	}
	secretStore := SecretStoreSchema{}
	err = yaml.Unmarshal([]byte(yamlOutput), &secretStore)
	if err != nil {
		return nil, err
	}
	return &secretStore, err
}

func ReadSecretPathFromSecretStore(name string) (string, error) {
	secretStore, err := GetExternalSecretsStore(name)
	if err != nil {
		return "", err
	}
	return secretStore.Metadata.Annotations.SharedPath, nil
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

func parseSecretStoresListing(jsonStr string) ([]string, error) {
	var secretStores SecretStoreListingSchema
	err := json.Unmarshal([]byte(jsonStr), &secretStores)
	if err != nil {
		return nil, err
	}

	var stores = []string{}
	for _, item := range secretStores.Items {
		stores = append(stores, item.Metadata.Name)
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
