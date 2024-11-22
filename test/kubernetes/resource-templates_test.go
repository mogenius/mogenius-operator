package kubernetes_test

import (
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/watcher"
	"path/filepath"
	"testing"
)

// test the functionality of the custom resource with a basic pod
func TestResourceTemplates(t *testing.T) {
	logManager := interfaces.NewMockSlogManager(t)
	config := config.NewConfig()
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

	// CREATE
	err = kubernetes.CreateOrUpdateResourceTemplateConfigmap()
	assert.Assert(err == nil, err)

	// unknown resource
	yaml := kubernetes.GetResourceTemplateYaml("", "v1", "mypod", "Pod", "default", "mypod")
	assert.Assert(yaml != "", "Error getting resource template")

	// known resource Deployment
	knownResourceYaml := kubernetes.GetResourceTemplateYaml("v1", "Deployment", "testtemplate", "Pod", "default", "mypod")
	assert.Assert(knownResourceYaml != "", "Error getting resource template")

	// known resource Certificate
	knownResourceYamlCert := kubernetes.GetResourceTemplateYaml("cert-manager.io/v1", "v1", "certificates", "Certificate", "default", "mypod")
	assert.Assert(knownResourceYamlCert != "", "Error getting resource template")
}
