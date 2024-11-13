package servicesExternal_test

import (
	"mogenius-k8s-manager/config"
	"mogenius-k8s-manager/interfaces"
	"mogenius-k8s-manager/kubernetes"
	servicesExternal "mogenius-k8s-manager/services-external"
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

func externalSecretStorePropsExample() servicesExternal.ExternalSecretStoreProps {
	return servicesExternal.ExternalSecretStoreProps{
		DisplayName:    "Vault Secret Store 1",
		ProjectId:      "jkhdfjk66-lkj4fdklfj-lkdsjfkl-4rt645-dalksf",
		Role:           "mogenius-external-secrets",
		VaultServerUrl: "http://vault.default.svc.cluster.local:8200",
		SecretPath:     "mogenius-external-secrets/data/phoenix",
	}
}

func TestSecretStoreRender(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config := config.NewConfig()
	servicesExternal.Setup(config)
	config.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
	})

	yamlTemplate := utils.InitExternalSecretsStoreYaml()

	secretStore := externalSecretStorePropsExample()
	secretStore.Name = "mo-ex-secr-test-003"
	secretStore.NamePrefix = "4jdh7e9dk7"
	secretStore.ProjectId = "djsajfh74-23423-234123-32fdsf"
	secretStore.Role = "mo-external-secrets-002"
	yamlDataUpdated := servicesExternal.RenderClusterSecretStore(yamlTemplate, secretStore)

	if yamlTemplate == yamlDataUpdated {
		t.Errorf("Error updating yaml data: %s", yamlTemplate)
	} else {
		logger.Log.Info("Yaml data updated ✅")
	}

	expectedPath := "secrets/data/mo-ex-secr-test-003"
	secretStore.SecretPath = expectedPath
	yamlDataUpdated = servicesExternal.RenderClusterSecretStore(yamlTemplate, secretStore)

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
	if testing.Short() {
		t.Skip()
	}

	logManager := interfaces.NewMockSlogManager()
	config := config.NewConfig()
	servicesExternal.Setup(config)
	config.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
	})
	kubernetes.Setup(logManager, config)

	props := externalSecretStorePropsExample()

	// assume composed name: 4jdh7e9dk7-vault-secret-store
	props.DisplayName = DisplayName
	props.NamePrefix = NamePrefix
	props.SecretPath = SecretPath
	props.ProjectId = ProjectId
	props.Role = Role

	err := servicesExternal.CreateExternalSecretsStore(props)
	if err != nil {
		t.Errorf("Error creating secret store: %s", err.Error())
	}
}

// don't move this test as it is dependent on the previous test to create the secret store!
func TestSecretStoreList(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
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
		}
	}
}
func TestListAvailSecrets(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Skip("Skipping TestListAvailSecrets temporarily, these only make sense with vault properly set up")

	availSecrets := servicesExternal.ListAvailableExternalSecrets(NamePrefix)

	if len(availSecrets) == 0 {
		t.Errorf("Error listing available secrets: No secrets found")
	}
}

func TestSecretStoreDelete(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	name := utils.GetSecretStoreName(NamePrefix)

	err := servicesExternal.DeleteExternalSecretsStore(name)
	if err != nil {
		t.Errorf("Error: Expected secret store %s to be deleted, but got this error instead: %s", utils.GetSecretStoreName(NamePrefix), err.Error())
	}
}
