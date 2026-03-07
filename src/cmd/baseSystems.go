package cmd

import (
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/k8sclient"
	mokubernetes "mogenius-operator/src/kubernetes"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"mogenius-operator/src/version"

	v1 "k8s.io/api/rbac/v1"
)

// baseSystems holds the minimal shared infrastructure used by all operator commands
// that require Kubernetes access. Each command-specific init function takes a baseSystems
// and layers its own services on top.
type baseSystems struct {
	clientProvider k8sclient.K8sClientProvider
	valkeyClient   valkeyclient.ValkeyClient
	versionModule  *version.Version
	logger         *slog.Logger
}

// initializeBaseSystems creates the shared foundation for all commands.
// Callers that need Valkey (cluster, nodemetrics) create their valkeyClient first and
// pass it here so kubernetes.Setup is called exactly once with the right client.
func initializeBaseSystems(
	logManagerModule logging.SlogManager,
	configModule *config.Config,
	cmdLogger *slog.Logger,
) baseSystems {
	assert.Assert(logManagerModule != nil)
	assert.Assert(configModule != nil)
	assert.Assert(cmdLogger != nil)

	clientProvider := k8sclient.NewK8sClientProvider(logManagerModule.CreateLogger("client-provider"), configModule)
	if !clientProvider.RunsInCluster() {
		impersonated, err := clientProvider.WithImpersonate(v1.Subject{
			Kind:      "ServiceAccount",
			Name:      "mogenius-operator-service-account-app",
			Namespace: configModule.Get("MO_OWN_NAMESPACE"),
		})
		assert.Assert(err == nil, err)
		clientProvider = impersonated
	}

	valkeyClient := valkeyclient.NewValkeyClient(logManagerModule.CreateLogger("valkey"), configModule)

	err := mokubernetes.Setup(logManagerModule, configModule, clientProvider, valkeyClient)
	assert.Assert(err == nil, err)
	utils.Setup(logManagerModule, configModule)

	return baseSystems{
		clientProvider: clientProvider,
		valkeyClient:   valkeyClient,
		versionModule:  version.NewVersion(),
		logger:         cmdLogger,
	}
}
