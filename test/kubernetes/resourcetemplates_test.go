package kubernetes_test

import (
	"log/slog"
	"mogenius-operator/src/assert"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"os"
	"testing"
)

// test the functionality of the custom resource with a basic pod
func TestResourceTemplates(t *testing.T) {
	logManager := logging.NewSlogManager(slog.LevelDebug, []slog.Handler{slog.NewJSONHandler(os.Stderr, nil)})
	config := cfg.NewConfig()
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_OWN_NAMESPACE",
		DefaultValue: utils.Pointer("mogenius"),
	})
	config.Declare(cfg.ConfigDeclaration{
		Key:          "KUBERNETES_DEBUG",
		DefaultValue: utils.Pointer("false"),
	})
	clientProvider := k8sclient.NewK8sClientProvider(logManager.CreateLogger("client-provider"), config)
	valkeyClient := valkeyclient.NewValkeyClient(logManager.CreateLogger("valkey"), config)
	err := kubernetes.Setup(logManager, config, clientProvider, valkeyClient)
	assert.AssertT(t, err == nil, err)

	// unknown resource
	yaml := kubernetes.GetResourceTemplateYaml("v1", "Pod")
	assert.AssertT(t, yaml != "", "Error getting resource template")

	// known resource Deployment
	knownResourceYaml := kubernetes.GetResourceTemplateYaml("v1", "Pod")
	assert.AssertT(t, knownResourceYaml != "", "Error getting resource template")

	// known resource Certificate
	knownResourceYamlCert := kubernetes.GetResourceTemplateYaml("cert-manager.io/v1", "Certificate")
	assert.AssertT(t, knownResourceYamlCert != "", "Error getting resource template")
}
