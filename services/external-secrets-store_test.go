package services

import (
	"testing"

	"github.com/mogenius/punq/logger"
	"gopkg.in/yaml.v2"
)

func TestSecretStoreRender(t *testing.T) {

	yamlTemplate := `apiVersion: external-secrets.io/v1beta1
	kind: ClusterSecretStore
	metadata:
	name: secret-store-vault-role-based
	annotations:
    mogenius-external-secrets/shared-path: <MO_SHARED_PATH>
	spec:
	provider:
    vault:
	server: <VAULT_SERVER_URL>
	version: "v2"
	auth:
	kubernetes:
	mountPath: "kubernetes"
	role: <ROLE>
	serviceAccountRef:
	name: <SERVICE_ACC>
	`

	secretStore := externalSecretStoreExample()
	secretStore.Role = "mo-external-secrets-002"
	yamlDataUpdated := renderClusterSecretStore(yamlTemplate, *secretStore)

	if yamlTemplate == yamlDataUpdated {
		t.Errorf("Error updating yaml data: %s", yamlTemplate)
	} else {
		logger.Log.Info("Yaml data updated ✅")
	}

	expectedPath := "secret/mo-ex-secr-test-003"
	secretStore.MoSharedPath = expectedPath
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
	response := CreateExternalSecretsStore(CreateSecretsStoreRequestExample())

	if response.Status != "SUCCESS" {
		t.Errorf("Error creating secret store: %s", response.Status)
	} else {
		logger.Log.Info("Secret store created ✅")
	}
}
