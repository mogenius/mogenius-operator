package services

import (
	"testing"

	mokubernetes "mogenius-k8s-manager/kubernetes"

	"github.com/mogenius/punq/logger"
	"gopkg.in/yaml.v2"
)

func TestSecretStoreResource(t *testing.T) {
	yamlData := `apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: test-secret-store
spec:
  provider:
    vault:
      server: "http://vault.default.svc.cluster.local:8200"
      version: "v2"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "mogenius-external-secrets"
          serviceAccountRef:
            name: "external-secrets-sa"
`
	// CREATE
	err := mokubernetes.ApplyResource(yamlData, true)
	if err != nil {
		t.Errorf("Error applying resource: %s", err.Error())
	} else {
		logger.Log.Info("Resource applied ✅")
	}

	// UPDATE (same resource), on second call the update client call is tested
	err = mokubernetes.ApplyResource(yamlData, true)
	if err != nil {
		t.Errorf("Error applying resource: %s", err.Error())
	} else {
		logger.Log.Info("Resource updated ✅")
	}

	// LIST
	_, err = mokubernetes.ListResources("external-secrets.io", "v1beta1", "clustersecretstores", "", true)
	if err != nil {
		t.Errorf("Error listing resources: %s", err.Error())
	} else {
		logger.Log.Info("Resources listed ✅")
	}

	// GET
	_, err = mokubernetes.GetResource("external-secrets.io", "v1beta1", "clustersecretstores", "test-secret-store", "", true)
	if err != nil {
		t.Errorf("Error getting resource: %s", err.Error())
	} else {
		logger.Log.Info("Resource retrieved ✅")
	}

	// DELETE
	err = mokubernetes.DeleteResource("external-secrets.io", "v1beta1", "clustersecretstores", "test-secret-store", "", true)
	if err != nil {
		t.Errorf("Error deleting resource: %s", err.Error())
	} else {
		logger.Log.Info("Resource deleted ✅")
	}
}

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

	secretStore := NewExternalSecretStore()
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
