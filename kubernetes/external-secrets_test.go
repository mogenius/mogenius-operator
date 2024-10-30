package kubernetes

import (
	"mogenius-k8s-manager/utils"
	"testing"

	"sigs.k8s.io/yaml"
)

const (
	NamePrefix   = "4jdh7e9dk7"
	ProjectId    = "djsajfh74-23423-234123-32fdsf"
	MoSharedPath = "mogenius-external-secrets/data/backend-project"
)

func TestSecretListRender(t *testing.T) {
	utils.CONFIG.Kubernetes.OwnNamespace = "mogenius"

	yamlTemplate := utils.InitExternalSecretListYaml()
	secretListProps := externalSecretListExample()

	// rendering overall works
	yamlDataRendered := renderExternalSecretList(yamlTemplate, secretListProps)
	if yamlTemplate == yamlDataRendered {
		t.Errorf("Error updating yaml data: %s", yamlTemplate)
	} else {
		K8sLogger.Info("Yaml data updated (1/2) ✅")
	}

	// change values and compare
	expectedName := NamePrefix + "-" + utils.SecretListSuffix // lowercase only
	secretListProps.NamePrefix = NamePrefix
	secretListProps.SecretName = "projectMayhem"
	yamlDataRenderedChanged := renderExternalSecretList(yamlTemplate, secretListProps)
	if yamlDataRenderedChanged == yamlDataRendered {
		t.Errorf("Error updating yaml data: %s", yamlTemplate)
	} else {
		K8sLogger.Info("Yaml data updated (2/2) ✅")
	}

	// check if the values are replaced as expected
	var data YamlDataList
	err := yaml.Unmarshal([]byte(yamlDataRenderedChanged), &data)
	if err != nil {
		t.Fatalf("Error parsing YAML: %v", err)
	}

	if data.Spec.Target.Name != expectedName {
		t.Errorf("Error updating Name: expected: %s, got: %s", expectedName, data.Spec.Target.Name)
	} else {
		K8sLogger.Info("MoSharedPath updated ✅")
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
	utils.CONFIG.Kubernetes.OwnNamespace = "mogenius"

	testReq := externalSecretListExample()

	err := CreateExternalSecretList(testReq)
	if err != nil {
		t.Errorf("Error creating external secret list. Err: %s", err.Error())
	} else {
		K8sLogger.Info("Secret store created ✅")
	}
}

func TestCreateExternalSecret(t *testing.T) {
	t.Skip("Skipping TestListAvailSecrets temporarily, these only make sense with vault properly set up")

	utils.CONFIG.Kubernetes.OwnNamespace = "mogenius"

	props := CreateExternalSecretProps{
		ServiceName:  "test-service",
		Namespace:    "mogenius",
		PropertyName: "postgresURL",
		NamePrefix:   NamePrefix,
	}

	secretName, err := CreateExternalSecret(props)
	if err != nil {
		t.Errorf("Error creating external secret list. Err: %s", err.Error())
	} else {
		K8sLogger.Info("Secret store created ✅", "store", secretName)
	}
}
