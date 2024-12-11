package servicesExternal_test

import (
	"mogenius-k8s-manager/src/assert"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	servicesExternal "mogenius-k8s-manager/src/servicesexternal"
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
	logManager := logging.NewMockSlogManager(t)
	config := cfg.NewConfig()
	servicesExternal.Setup(logManager, config)
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
	})
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_PATH",
		DefaultValue: utils.Pointer(filepath.Join(t.TempDir(), "mogenius.db")),
	})
	clientProvider := k8sclient.NewK8sClientProvider(logManager.CreateLogger("client-provider"))
	watcherModule := kubernetes.NewWatcher(clientProvider)
	err := kubernetes.Setup(logManager, config, watcherModule, clientProvider)
	assert.AssertT(t, err == nil, err)

	props := externalSecretStorePropsExample()

	// assume composed name: 4jdh7e9dk7-vault-secret-store
	props.DisplayName = DisplayName
	props.NamePrefix = NamePrefix
	props.SecretPath = SecretPath
	props.ProjectId = ProjectId
	props.Role = Role

	err = servicesExternal.CreateExternalSecretsStore(props)
	assert.AssertT(t, err == nil, err)
}

// don't move this test as it is dependent on the previous test to create the secret store!
func TestSecretStoreList(t *testing.T) {
	t.Skip("test relies on another test which is illegal")
	stores, err := kubernetes.ListExternalSecretsStores(ProjectId)
	assert.AssertT(t, err == nil, err)

	assert.AssertT(t, len(stores) > 0, "Error listing secret stores: No secret stores found")
	found := false
	for _, store := range stores {
		if store.Prefix == NamePrefix && store.ProjectId == ProjectId {
			found = true
			break
		}
	}
	assert.AssertT(t, found, "Error: Expected to find secret store %s but none was found", utils.GetSecretStoreName(NamePrefix))
}

func TestListAvailSecrets(t *testing.T) {
	t.Skip("Skipping TestListAvailSecrets temporarily, these only make sense with vault properly set up")
	availSecrets := servicesExternal.ListAvailableExternalSecrets(NamePrefix)

	assert.AssertT(t, len(availSecrets) > 0, "Error listing available secrets: No secrets found")
}

func TestSecretStoreDelete(t *testing.T) {
	name := utils.GetSecretStoreName(NamePrefix)

	err := servicesExternal.DeleteExternalSecretsStore(name)
	assert.AssertT(t, err == nil, err)
}
