package kubernetes_test

import (
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/utils"
	"testing"

	"sigs.k8s.io/yaml"
)

const (
	NamePrefix   = "4jdh7e9dk7"
	ProjectId    = "djsajfh74-23423-234123-32fdsf"
	MoSharedPath = "mogenius-external-secrets/data/backend-project"
)

func externalSecretListExample() kubernetes.ExternalSecretListProps {
	return kubernetes.ExternalSecretListProps{
		NamePrefix:      "fsd87fdh",
		SecretName:      "database-credentials",
		SecretStoreName: "fsd87fdh" + utils.SecretStoreSuffix,
		SecretPath:      "mogenius-external-secrets/data/team-blue",
	}
}

func TestSecretListRender(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)
	config.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
	})

	yamlTemplate := utils.InitExternalSecretListYaml()
	secretListProps := externalSecretListExample()

	// rendering overall works
	yamlDataRendered := kubernetes.RenderExternalSecretList(yamlTemplate, secretListProps)
	if yamlTemplate == yamlDataRendered {
		t.Errorf("Error updating yaml data: %s", yamlTemplate)
	}

	// change values and compare
	expectedName := NamePrefix + "-" + utils.SecretListSuffix // lowercase only
	secretListProps.NamePrefix = NamePrefix
	secretListProps.SecretName = "projectMayhem"
	yamlDataRenderedChanged := kubernetes.RenderExternalSecretList(yamlTemplate, secretListProps)
	if yamlDataRenderedChanged == yamlDataRendered {
		t.Errorf("Error updating yaml data: %s", yamlTemplate)
	}

	// check if the values are replaced as expected
	var data YamlDataList
	err := yaml.Unmarshal([]byte(yamlDataRenderedChanged), &data)
	if err != nil {
		t.Fatalf("Error parsing YAML: %v", err)
	}

	if data.Spec.Target.Name != expectedName {
		t.Errorf("Error updating Name: expected: %s, got: %s", expectedName, data.Spec.Target.Name)
	}
}

type YamlDataList struct {
	Spec struct {
		Target struct {
			Name string `yaml:"name"`
		} `yaml:"target"`
	} `yaml:"spec"`
}

func TestCreateExternalSecretList(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := config.NewConfig()
	kubernetes.Setup(logManager, config)
	config.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
	})

	testReq := externalSecretListExample()

	err := kubernetes.CreateExternalSecretList(testReq)
	if err != nil {
		t.Errorf("Error creating external secret list. Err: %s", err.Error())
	}
}
