package reconciler

import (
	"context"
	"log/slog"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/utils"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ReconcilerFactory(logger *slog.Logger, clientProvider k8sclient.K8sClientProvider, interval time.Duration) Reconciler {
	configs := []ResourceConfig{
		{
			Resource:  utils.ResourceDescriptor{Plural: "namespaces", Kind: "Namespace", ApiVersion: "v1", Namespaced: false},
			Reconcile: reconcileNamespaces,
		},
		{
			Resource:  utils.ResourceDescriptor{Plural: "clusterroles", Kind: "ClusterRole", ApiVersion: "rbac.authorization.k8s.io/v1", Namespaced: false},
			Reconcile: reconcileClusterRoles,
		},
		{
			Resource:  utils.ResourceDescriptor{Plural: "clusterrolebindings", Kind: "ClusterRoleBinding", ApiVersion: "rbac.authorization.k8s.io/v1", Namespaced: false},
			Reconcile: reconcileClusterRoleBindings,
		},
		{
			Resource:  utils.ResourceDescriptor{Plural: "rolebindings", Kind: "RoleBinding", ApiVersion: "rbac.authorization.k8s.io/v1", Namespaced: false},
			Reconcile: reconcileRoleBindings,
		},
		{
			Resource:  utils.ResourceDescriptor{Plural: "configmaps", Kind: "ConfigMap", ApiVersion: "v1", Namespaced: true},
			Reconcile: reconcileConfigMaps,
		},
		{
			Resource:  utils.ResourceDescriptor{Plural: "workspaces", Kind: "Workspace", ApiVersion: "mogenius.com/v1alpha1", Namespaced: false},
			Reconcile: reconcileWorkspaces,
		},
		{
			Resource:  utils.ResourceDescriptor{Plural: "users", Kind: "User", ApiVersion: "mogenius.com/v1alpha1", Namespaced: false},
			Reconcile: reconcileUsers,
		},
		{
			Resource:  utils.ResourceDescriptor{Plural: "grants", Kind: "Grant", ApiVersion: "mogenius.com/v1alpha1", Namespaced: false},
			Reconcile: reconcileGrants,
		},
	}
	return newReconciler(logger, clientProvider, interval, configs)
}

func reconcileNamespaces(ctx context.Context, obj *unstructured.Unstructured, operation operation) []ReconcileResult {
	return []ReconcileResult{}
}

func reconcileClusterRoles(ctx context.Context, obj *unstructured.Unstructured, operation operation) []ReconcileResult {
	return []ReconcileResult{}
}

func reconcileClusterRoleBindings(ctx context.Context, obj *unstructured.Unstructured, operation operation) []ReconcileResult {
	return []ReconcileResult{}
}

func reconcileRoleBindings(ctx context.Context, obj *unstructured.Unstructured, operation operation) []ReconcileResult {
	return []ReconcileResult{}
}

func reconcileConfigMaps(ctx context.Context, obj *unstructured.Unstructured, operation operation) []ReconcileResult {
	return []ReconcileResult{}
}

func reconcileWorkspaces(ctx context.Context, obj *unstructured.Unstructured, operation operation) []ReconcileResult {
	return []ReconcileResult{}
}

func reconcileUsers(ctx context.Context, obj *unstructured.Unstructured, operation operation) []ReconcileResult {
	return []ReconcileResult{}
}

func reconcileGrants(ctx context.Context, obj *unstructured.Unstructured, operation operation) []ReconcileResult {
	return []ReconcileResult{}
}
