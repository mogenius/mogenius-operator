package services

import (
	"mogenius-k8s-manager/utils"
	"testing"

	"github.com/mogenius/punq/logger"
	"gopkg.in/yaml.v2"
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
		logger.Log.Info("Yaml data updated (1/2) ✅")
	}

	// change values and compare
	expectedName := "team-yellow-projectmayhem-" + SecretListSuffix // lowercase only
	secretListProps.NamePrefix = "team-yellow"
	secretListProps.Project = "projectMayhem"
	yamlDataRenderedChanged := renderExternalSecretList(yamlTemplate, secretListProps)
	if yamlDataRenderedChanged == yamlDataRendered {
		t.Errorf("Error updating yaml data: %s", yamlTemplate)
	} else {
		logger.Log.Info("Yaml data updated (2/2) ✅")
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
		logger.Log.Info("MoSharedPath updated ✅")
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
		logger.Log.Info("Secret store created ✅")
	}
}

func TestCreateExternalSecret(t *testing.T) {
	utils.CONFIG.Kubernetes.OwnNamespace = "mogenius"
	// prereq
	TestSecretStoreCreate(t)

	// cleanup even when test fails
	defer TestSecretStoreDelete(t)

	testReq := CreateExternalSecretRequestExample()

	// assume composed name: customer-blue-backend-project-backend-service003-postgresURL
	testReq.SecretStoreNamePrefix = "customer-blue"
	testReq.ProjectName = "backend-project"
	testReq.ServiceName = "backend-service03"
	testReq.PropertyName = "postgresURL"
	// testReq.Namespace = "mogenius"
	testReq.Namespace = "vs-proj-blubb-2ghl0a"

	response := CreateExternalSecret(testReq)
	if response.Status != "SUCCESS" {
		t.Errorf("Error creating external secret: %s", response.ErrorMessage)
	} else {
		logger.Log.Info("External secret created ✅")
	}
}
