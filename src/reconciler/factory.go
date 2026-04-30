package reconciler

import (
	"log/slog"
	"mogenius-operator/src/config"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"time"
)

// reconcilerModule holds shared dependencies for all reconcile handler methods.
type reconcilerModule struct {
	logger         *slog.Logger
	clientProvider k8sclient.K8sClientProvider
	config         config.ConfigModule
	valkeyClient   valkeyclient.ValkeyClient
}

type reconcilerFactory struct {
	module   *reconcilerModule
	interval time.Duration
	configs  []ResourceConfig
}

type ReconcilerFactory interface {
	Build() Reconciler
}

func NewReconcilerFactory(logger *slog.Logger, clientProvider k8sclient.K8sClientProvider, configModule config.ConfigModule, valkeyClient valkeyclient.ValkeyClient) ReconcilerFactory {
	factory := &reconcilerFactory{
		module: &reconcilerModule{
			logger:         logger,
			clientProvider: clientProvider,
			config:         configModule,
			valkeyClient:   valkeyClient,
		},
		interval: 1 * time.Minute,
		configs:  []ResourceConfig{},
	}

	factory.WithReconciler(utils.WorkspaceResource, factory.module.reconcileWorkspaces).
		WithReconciler(utils.PlatformConfigResource, factory.module.reconcilePlatformConfig)

	return factory
}

func (f *reconcilerFactory) WithReconciler(resource utils.ResourceDescriptor, reconcileFunc ReconcileFunc) *reconcilerFactory {
	f.configs = append(f.configs, ResourceConfig{
		Resource:  resource,
		Reconcile: reconcileFunc,
	})
	return f
}

func (f *reconcilerFactory) Build() Reconciler {
	return newReconciler(f.module.logger, f.module.clientProvider, f.interval, f.configs)
}
