package servicesExternal

import (
	"fmt"
	"mogenius-k8s-manager/kubernetes"
	"time"

	"mogenius-k8s-manager/utils"
	"strings"
	"testing"

	"github.com/mogenius/punq/logger"
	"gopkg.in/yaml.v2"
)

const (
	NamePrefix   = "customer-blue"
	ProjectName  = "backend-project"
	MoSharedPath = "mogenius-external-secrets"
)

func TestSecretStoreRender(t *testing.T) {

	yamlTemplate := utils.InitExternalSecretsStoreYaml()

	secretStore := externalSecretStorePropsExample()
	secretStore.Role = "mo-external-secrets-002"
	yamlDataUpdated := renderClusterSecretStore(yamlTemplate, secretStore)

	if yamlTemplate == yamlDataUpdated {
		t.Errorf("Error updating yaml data: %s", yamlTemplate)
	} else {
		logger.Log.Info("Yaml data updated ✅")
	}

	expectedPath := "secret-mo-ex-secr-test-003"
	secretStore.MoSharedPath = expectedPath
	expectedPath = fmt.Sprintf("%s/%s", expectedPath, secretStore.ProjectName) // the rendering adds the project name to the path to reflect the corresponding secret store
	yamlDataUpdated = renderClusterSecretStore(yamlTemplate, secretStore)

	// check if the values are replaced
	var data kubernetes.SecretStoreSchema
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

func TestSecretStoreCreate(t *testing.T) {
	utils.CONFIG.Kubernetes.OwnNamespace = "mogenius"

	props := externalSecretStorePropsExample()

	// assume composed name: team-blue-secrets-vault-secret-store
	props.NamePrefix = NamePrefix
	props.ProjectName = ProjectName

	err := CreateExternalSecretsStore(props)
	if err != nil {
		t.Errorf("Error creating secret store: %s", err.Error())
	} else {
		logger.Log.Info("Secret store created ✅")
	}
}

// don't move this test as it is dependent on the previous test to create the secret store!
func TestSecretStoreList(t *testing.T) {
	// wait for create to finish
	time.Sleep(3 * time.Second)

	stores, err := kubernetes.ListExternalSecretsStores(ProjectName)
	if err != nil {
		t.Errorf("Error listing secret stores: %s", err.Error())
	}

	if len(stores) == 0 {
		t.Errorf("Error listing secret stores: No secret stores found")
	} else {
		found := false
		for _, store := range stores {
			if strings.HasPrefix(store, NamePrefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Error: Expected to find secret store %s but none was found", utils.GetSecretStoreName(NamePrefix, ProjectName))
		} else {
			logger.Log.Info("Secret stores listed ✅")
		}
	}
}
func TestListAvailSecrets(t *testing.T) {
	utils.CONFIG.Kubernetes.OwnNamespace = "mogenius"
	// prereq
	_, err := kubernetes.CreateSecret(utils.CONFIG.Kubernetes.OwnNamespace, nil)
	if err != nil {
		logger.Log.Info("Secret list already exists.")
	} else {
		logger.Log.Info("Secret list created ✅")
	}
	availSecrets := ListAvailableExternalSecrets(NamePrefix, ProjectName)

	if len(availSecrets) == 0 {
		t.Errorf("Error listing available secrets: No secrets found")
	} else {
		logger.Log.Info(fmt.Sprintf("Available secrets list ✅: %v", availSecrets))
	}
}

func TestSecretStoreDelete(t *testing.T) {
	utils.CONFIG.Kubernetes.OwnNamespace = "mogenius"

	err := DeleteExternalSecretsStore(NamePrefix, ProjectName, MoSharedPath)
	if err != nil {
		t.Errorf("Error: Expected secret store %s to be deleted, but got this error instead: %s", utils.GetSecretStoreName(NamePrefix, ProjectName), err.Error())
	} else {
		logger.Log.Info("Secret store deletion confirmed ✅")
	}
}
