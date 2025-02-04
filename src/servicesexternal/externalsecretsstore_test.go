package servicesexternal_test

import (
	"mogenius-k8s-manager/src/assert"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/logging"
	servicesExternal "mogenius-k8s-manager/src/servicesexternal"
	"mogenius-k8s-manager/src/utils"
	"testing"

	"sigs.k8s.io/yaml"
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
	t.Parallel()
	config := cfg.NewConfig()
	slogManager := logging.NewMockSlogManager(t)

	servicesExternal.Setup(slogManager, config)
	config.Declare(cfg.ConfigDeclaration{
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

	assert.AssertT(t, yamlTemplate != yamlDataUpdated, "yaml data should be different")

	expectedPath := "secrets/data/mo-ex-secr-test-003"
	secretStore.SecretPath = expectedPath
	yamlDataUpdated = servicesExternal.RenderClusterSecretStore(yamlTemplate, secretStore)

	// check if the values are replaced
	var data SecretStoreSchema
	err := yaml.Unmarshal([]byte(yamlDataUpdated), &data)
	assert.AssertT(t, err == nil, "unmarshal should work", err)

	parsedPath := data.Metadata.Annotations["mogenius-external-secrets/shared-path"]
	assert.AssertT(t, parsedPath == expectedPath, "parsedPath should match the expectedPath", "expected: "+expectedPath, "got: "+parsedPath)
}
