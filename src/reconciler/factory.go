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
}

type ReconcilerFactory interface {
	Build() Reconciler
}

func NewReconcilerFactory(logger *slog.Logger, clientProvider k8sclient.K8sClientProvider, configModule config.ConfigModule, valkeyClient valkeyclient.ValkeyClient) ReconcilerFactory {
	return &reconcilerFactory{
		module: &reconcilerModule{
			logger:         logger,
			clientProvider: clientProvider,
			config:         configModule,
			valkeyClient:   valkeyClient,
		},
		interval: 5 * time.Minute, // Example interval for background reconciliation
	}
}

// ReconcilerFactory creates a Reconciler that reacts to changes in the RBAC and CRD resources
// relevant to mogenius grant management. interval controls optional background re-reconciliation;
// 0 disables it (event-driven only).
func (f *reconcilerFactory) Build() Reconciler {

	configs := []ResourceConfig{
		{
			Resource:  utils.ResourceDescriptor{Plural: "namespaces", Kind: "Namespace", ApiVersion: "v1", Namespaced: false},
			Reconcile: f.module.reconcileNamespaces,
		},
		{
			Resource:  utils.ResourceDescriptor{Plural: "clusterroles", Kind: "ClusterRole", ApiVersion: "rbac.authorization.k8s.io/v1", Namespaced: false},
			Reconcile: f.module.reconcileClusterRoles,
		},
		{
			Resource:  utils.ResourceDescriptor{Plural: "workspaces", Kind: "Workspace", ApiVersion: "mogenius.com/v1alpha1", Namespaced: false},
			Reconcile: f.module.reconcileWorkspaces,
		},
		{
			Resource:  utils.ResourceDescriptor{Plural: "users", Kind: "User", ApiVersion: "mogenius.com/v1alpha1", Namespaced: false},
			Reconcile: f.module.reconcileUsers,
		},
		{
			Resource:  utils.ResourceDescriptor{Plural: "grants", Kind: "Grant", ApiVersion: "mogenius.com/v1alpha1", Namespaced: false},
			Reconcile: f.module.reconcileGrants,
		},
	}
	return newReconciler(f.module.logger, f.module.clientProvider, f.interval, configs)
}
