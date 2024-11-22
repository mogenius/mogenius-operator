package servicesExternal_test

import (
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/kubernetes"
	servicesExternal "mogenius-k8s-manager/src/services-external"
	"mogenius-k8s-manager/src/watcher"
	"path/filepath"

	"mogenius-k8s-manager/src/utils"
	"testing"
)

const (
	DisplayName = "Vault Secret Store 1"
	NamePrefix  = "4jdh7e9dk7"
	ProjectId   = "djsajfh74-23423-234123-32fdsf"
	SecretPath  = "mogenius-external-secrets/data/backend-project"
	Role        = "db-access-role"
)

func externalSecretStorePropsExample() servicesExternal.ExternalSecretStoreProps {
	return servicesExternal.ExternalSecretStoreProps{
		DisplayName:    "Vault Secret Store 1",
		ProjectId:      "jkhdfjk66-lkj4fdklfj-lkdsjfkl-4rt645-dalksf",
		Role:           "mogenius-external-secrets",
		VaultServerUrl: "http://vault.default.svc.cluster.local:8200",
		SecretPath:     "mogenius-external-secrets/data/phoenix",
	}
}

func TestSecretStoreCreate(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := config.NewConfig()
	servicesExternal.Setup(logManager, config)
	config.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
	})
	config.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_PATH",
		DefaultValue: utils.Pointer(filepath.Join(t.TempDir(), "mogenius.db")),
	})
	watcherModule := watcher.NewWatcher()
	err := kubernetes.Setup(logManager, config, watcherModule)
	assert.Assert(err == nil, err)

	props := externalSecretStorePropsExample()

	// assume composed name: 4jdh7e9dk7-vault-secret-store
	props.DisplayName = DisplayName
	props.NamePrefix = NamePrefix
	props.SecretPath = SecretPath
	props.ProjectId = ProjectId
	props.Role = Role

	err = servicesExternal.CreateExternalSecretsStore(props)
	assert.Assert(err == nil, err)
}

// don't move this test as it is dependent on the previous test to create the secret store!
func TestSecretStoreList(t *testing.T) {
	t.Skip("test relies on another test which is illegal")
	stores, err := kubernetes.ListExternalSecretsStores(ProjectId)
	assert.Assert(err == nil, err)

	assert.Assert(len(stores) > 0, "Error listing secret stores: No secret stores found")
	found := false
	for _, store := range stores {
		if store.Prefix == NamePrefix && store.ProjectId == ProjectId {
			found = true
			break
		}
	}
	assert.Assert(found, "Error: Expected to find secret store %s but none was found", utils.GetSecretStoreName(NamePrefix))
}

func TestListAvailSecrets(t *testing.T) {
	t.Skip("Skipping TestListAvailSecrets temporarily, these only make sense with vault properly set up")
	availSecrets := servicesExternal.ListAvailableExternalSecrets(NamePrefix)

	assert.Assert(len(availSecrets) > 0, "Error listing available secrets: No secrets found")
}

func TestSecretStoreDelete(t *testing.T) {
	name := utils.GetSecretStoreName(NamePrefix)

	err := servicesExternal.DeleteExternalSecretsStore(name)
	assert.Assert(err == nil, err)
}
