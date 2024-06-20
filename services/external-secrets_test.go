package services

import (
	"mogenius-k8s-manager/utils"
	"testing"

	"github.com/mogenius/punq/logger"
	"gopkg.in/yaml.v2"
)

func TestSecretListRender(t *testing.T) {
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
	expectedName := "team-yellow-projectMayhem-" + SecretListSuffix
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
	utils.InitConfigYaml(false, "", "dev")
	testReq := externalSecretListExample()
	testReq.NamePrefix = "mo-ex-secr-test-003"
	response := CreateExternalSecretList(testReq)

	if response.Status != "SUCCESS" {
		t.Errorf("Error creating secret store: %s", response.Status)
	} else {
		logger.Log.Info("Secret store created ✅")
	}
}
