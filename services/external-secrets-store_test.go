package services

import (
	"fmt"
	"mogenius-k8s-manager/utils"
	"strings"
	"testing"

	"github.com/mogenius/punq/logger"
	"gopkg.in/yaml.v2"
)

const (
	NamePrefix   = "customer-blue"
	Project      = "backend-project"
	MoSharedPath = "mogenius-external-secrets"
)

func TestSecretStoreRender(t *testing.T) {

	yamlTemplate := utils.InitExternalSecretsStoreYaml()

	secretStore := externalSecretStoreExample()
	secretStore.Role = "mo-external-secrets-002"
	yamlDataUpdated := renderClusterSecretStore(yamlTemplate, *secretStore)

	if yamlTemplate == yamlDataUpdated {
		t.Errorf("Error updating yaml data: %s", yamlTemplate)
	} else {
		logger.Log.Info("Yaml data updated ✅")
	}

	expectedPath := "secret-mo-ex-secr-test-003"
	secretStore.MoSharedPath = expectedPath
	expectedPath = fmt.Sprintf("%s/%s", expectedPath, secretStore.Project) // the rendering adds the project name to the path to reflect the corresponding secret store
	yamlDataUpdated = renderClusterSecretStore(yamlTemplate, *secretStore)

	// check if the values are replaced
	var data YamlData
	err := yaml.Unmarshal([]byte(yamlDataUpdated), &data)
	if err != nil {
		t.Fatalf("Error parsing YAML: %v", err)
	}

	if data.Metadata.Annotations.SharedPath != expectedPath {
		t.Errorf("Error updating MoSharedPath: expected: %s, got: %s", expectedPath, data.Metadata.Annotations.SharedPath)
	} else {
		logger.Log.Info("MoSharedPath updated ✅")
	}
}

type YamlData struct {
	Metadata struct {
		Annotations struct {
			SharedPath string `yaml:"mogenius-external-secrets/shared-path"`
		} `yaml:"annotations"`
	} `yaml:"metadata"`
}

func TestSecretStoreCreate(t *testing.T) {
	utils.CONFIG.Kubernetes.OwnNamespace = "mogenius"

	testReq := CreateSecretsStoreRequestExample()

	// assume composed name: team-blue-secrets-vault-secret-store
	testReq.NamePrefix = NamePrefix
	testReq.Project = Project

	response := CreateExternalSecretsStore(testReq)
	if response.Status != "SUCCESS" {
		t.Errorf("Error creating secret store: %s", response.Status)
	} else {
		logger.Log.Info("Secret store created ✅")
	}
}

// don't move this test as it is dependent on the previous test to create the secret store!
func TestSecretStoreList(t *testing.T) {
	response := ListExternalSecretsStores()

	if len(response.StoresInCluster) == 0 {
		t.Errorf("Error listing secret stores: No secret stores found")
	} else {
		found := false
		for _, store := range response.StoresInCluster {
			if strings.HasPrefix(store.Name, NamePrefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Error: Expected to find secret store %s but none was found", getSecretStoreName(NamePrefix, Project))
		} else {
			logger.Log.Info("Secret stores listed ✅")
		}
	}
}

func TestSecretStoreDelete(t *testing.T) {
	utils.CONFIG.Kubernetes.OwnNamespace = "mogenius"

	response := DeleteExternalSecretsStore(DeleteSecretsStoreRequest{
		NamePrefix:   NamePrefix,
		Project:      Project,
		MoSharedPath: MoSharedPath,
	})
	if response.Status != "SUCCESS" {
		t.Errorf("Error: Expected secret store %s to be deleted, but got this error instead: %s", getSecretStoreName(NamePrefix, Project), response.ErrorMessage)
	} else {
		logger.Log.Info("Secret store deletion confirmed ✅")
	}
}
