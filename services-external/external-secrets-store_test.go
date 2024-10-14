package servicesExternal

import (
	"fmt"
	"mogenius-k8s-manager/kubernetes"
	"time"

	"mogenius-k8s-manager/utils"
	"testing"

	"github.com/mogenius/punq/logger"
	"sigs.k8s.io/yaml"
)

const (
	DisplayName = "Vault Secret Store 1"
	NamePrefix  = "4jdh7e9dk7"
	ProjectId   = "djsajfh74-23423-234123-32fdsf"
	SecretPath  = "mogenius-external-secrets/data/backend-project"
	Role        = "db-access-role"
)

type SecretStoreSchema struct {
	Metadata struct {
		Name        string            `yaml:"name"`
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
	Spec struct {
		Provider struct {
			Vault struct {
				Server string `yaml:"server"`
				Auth   struct {
					Kubernetes struct {
						Role string `yaml:"role"`
					} `yaml:"kubernetes"`
				} `yaml:"auth"`
			} `yaml:"vault"`
		} `yaml:"provider"`
	} `yaml:"spec"`
}

func TestSecretStoreRender(t *testing.T) {

	yamlTemplate := utils.InitExternalSecretsStoreYaml()

	secretStore := externalSecretStorePropsExample()
	secretStore.Name = "mo-ex-secr-test-003"
	secretStore.NamePrefix = "4jdh7e9dk7"
	secretStore.ProjectId = "djsajfh74-23423-234123-32fdsf"
	secretStore.Role = "mo-external-secrets-002"
	yamlDataUpdated := renderClusterSecretStore(yamlTemplate, secretStore)

	if yamlTemplate == yamlDataUpdated {
		t.Errorf("Error updating yaml data: %s", yamlTemplate)
	} else {
		logger.Log.Info("Yaml data updated ✅")
	}

	expectedPath := "secrets/data/mo-ex-secr-test-003"
	secretStore.SecretPath = expectedPath
	yamlDataUpdated = renderClusterSecretStore(yamlTemplate, secretStore)

	// check if the values are replaced
	var data SecretStoreSchema
	err := yaml.Unmarshal([]byte(yamlDataUpdated), &data)
	if err != nil {
		t.Fatalf("Error parsing YAML: %v", err)
	}

	parsedPath := data.Metadata.Annotations["mogenius-external-secrets/shared-path"]
	if parsedPath != expectedPath {
		t.Errorf("Error updating SecretPath: expected: %s, got: %s", expectedPath, parsedPath)
	} else {
		logger.Log.Info("SecretPath updated ✅")
	}
}

func TestSecretStoreCreate(t *testing.T) {
	utils.CONFIG.Kubernetes.OwnNamespace = "mogenius"

	props := externalSecretStorePropsExample()

	// assume composed name: 4jdh7e9dk7-vault-secret-store
	props.DisplayName = DisplayName
	props.NamePrefix = NamePrefix
	props.SecretPath = SecretPath
	props.ProjectId = ProjectId
	props.Role = Role

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

	stores, err := kubernetes.ListExternalSecretsStores(ProjectId)
	if err != nil {
		t.Errorf("Error listing secret stores: %s", err.Error())
	}

	if len(stores) == 0 {
		t.Errorf("Error listing secret stores: No secret stores found")
	} else {
		found := false
		for _, store := range stores {
			if store.Prefix == NamePrefix && store.ProjectId == ProjectId {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Error: Expected to find secret store %s but none was found", utils.GetSecretStoreName(NamePrefix))
		} else {
			logger.Log.Info("Secret stores listed ✅")
		}
	}
}
func TestListAvailSecrets(t *testing.T) {
	t.Skip("Skipping TestListAvailSecrets temporarily, these only make sense with vault properly set up")

	utils.CONFIG.Kubernetes.OwnNamespace = "mogenius"
	// prereq
	// _, err := kubernetes.CreateSecret(utils.CONFIG.Kubernetes.OwnNamespace, &v1.Secret{
	// 	Data: map[string][]byte{
	// 		"backend-project": []byte("{\"postgresURL\":\"postgres\",\"postgressPW\":\"fjksdhf7\"}"),
	// 	},
	// })
	// if err != nil {
	// 	logger.Log.Info("Secret list already exists.")
	// } else {
	// 	logger.Log.Info("Secret list created ✅")
	// }
	availSecrets := ListAvailableExternalSecrets(NamePrefix)

	if len(availSecrets) == 0 {
		t.Errorf("Error listing available secrets: No secrets found")
	} else {
		logger.Log.Info(fmt.Sprintf("Available secrets list ✅: %v", availSecrets))
	}
}

func TestSecretStoreDelete(t *testing.T) {
	utils.CONFIG.Kubernetes.OwnNamespace = "mogenius"
	name := utils.GetSecretStoreName(NamePrefix)

	err := DeleteExternalSecretsStore(name)
	if err != nil {
		t.Errorf("Error: Expected secret store %s to be deleted, but got this error instead: %s", utils.GetSecretStoreName(NamePrefix), err.Error())
	} else {
		logger.Log.Info("Secret store deletion confirmed ✅")
	}
}
