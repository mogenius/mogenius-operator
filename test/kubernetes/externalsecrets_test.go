package kubernetes_test

import (
	"fmt"
	"mogenius-k8s-manager/src/assert"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/utils"
	"path/filepath"
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
	logManager := logging.NewMockSlogManager(t)
	config := cfg.NewConfig()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
	})
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_PATH",
		DefaultValue: utils.Pointer(filepath.Join(t.TempDir(), "mogenius.db")),
	})
	watcherModule := kubernetes.NewWatcher()
	err := kubernetes.Setup(logManager, config, watcherModule)
	assert.AssertT(t, err == nil, err)

	yamlTemplate := utils.InitExternalSecretListYaml()
	secretListProps := externalSecretListExample()

	// rendering overall works
	yamlDataRendered := kubernetes.RenderExternalSecretList(yamlTemplate, secretListProps)
	assert.AssertT(t, yamlTemplate != yamlDataRendered, fmt.Sprintf("Error updating yaml data: %s", yamlTemplate))

	// change values and compare
	expectedName := NamePrefix + "-" + utils.SecretListSuffix // lowercase only
	secretListProps.NamePrefix = NamePrefix
	secretListProps.SecretName = "projectMayhem"
	yamlDataRenderedChanged := kubernetes.RenderExternalSecretList(yamlTemplate, secretListProps)
	assert.AssertT(t, yamlDataRenderedChanged != yamlDataRendered, fmt.Sprintf("Error updating yaml data: %s", yamlTemplate))

	// check if the values are replaced as expected
	var data YamlDataList
	err = yaml.Unmarshal([]byte(yamlDataRenderedChanged), &data)
	assert.AssertT(t, err == nil, err)

	assert.AssertT(t, data.Spec.Target.Name == expectedName, fmt.Sprintf(
		"Error updating Name: expected: %s, got: %s",
		expectedName,
		data.Spec.Target.Name,
	))
}

type YamlDataList struct {
	Spec struct {
		Target struct {
			Name string `yaml:"name"`
		} `yaml:"target"`
	} `yaml:"spec"`
}

func TestCreateExternalSecretList(t *testing.T) {
	logManager := logging.NewMockSlogManager(t)
	config := cfg.NewConfig()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
	})
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_PATH",
		DefaultValue: utils.Pointer(filepath.Join(t.TempDir(), "mogenius.db")),
	})
	watcherModule := kubernetes.NewWatcher()
	err := kubernetes.Setup(logManager, config, watcherModule)
	assert.AssertT(t, err == nil, err)

	testReq := externalSecretListExample()

	err = kubernetes.CreateExternalSecretList(testReq)
	assert.AssertT(t, err == nil, err)
}
