package core

import (
	"encoding/json"
	"log/slog"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/utils"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const DEFAULT_WORKSPACE_DASHBOARD_NAME = "default"

// defaultWorkspaceDashboard mirrors the frontend's built-in default layout:
// every standard component enabled plus one "Controller" resource table.
func defaultWorkspaceDashboard(namespace string) v1alpha1.WorkspaceDashboard {
	enabled := v1alpha1.DashboardToggle{Enabled: true}
	return v1alpha1.WorkspaceDashboard{
		TypeMeta: metav1.TypeMeta{
			Kind:       utils.WorkspaceDashboardResource.Kind,
			APIVersion: utils.WorkspaceDashboardResource.ApiVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      DEFAULT_WORKSPACE_DASHBOARD_NAME,
			Namespace: namespace,
		},
		Spec: v1alpha1.WorkspaceDashboardSpec{
			Default:          true,
			DashboardHeader:  enabled,
			CapacitiesTile:   enabled,
			AiInsightsBanner: enabled,
			MembersTile:      enabled,
			ResourceOverview: enabled,
			ResourceTables: []v1alpha1.DashboardResourceTable{
				{
					Header: "Controller",
					Resources: []v1alpha1.CrdReference{
						{ApiVersion: "apps/v1", Kind: "Deployment"},
						{ApiVersion: "apps/v1", Kind: "StatefulSet"},
						{ApiVersion: "apps/v1", Kind: "DaemonSet"},
						{ApiVersion: "batch/v1", Kind: "CronJob"},
					},
				},
			},
		},
	}
}

// EnsureDefaultWorkspaceDashboard creates the default WorkspaceDashboard if no
// dashboard with spec.default=true exists yet. Meant to run on the leader —
// concurrent replicas would only race into AlreadyExists errors, which are
// tolerated anyway.
func EnsureDefaultWorkspaceDashboard(logger *slog.Logger, config cfg.ConfigModule) {
	namespace := config.Get("MO_OWN_NAMESPACE")

	dashboards := kubernetes.GetUnstructuredResourceListFromStore(
		utils.WorkspaceDashboardResource.ApiVersion,
		utils.WorkspaceDashboardResource.Kind,
		&namespace,
		nil,
	)
	for _, dashboard := range dashboards.Items {
		if isDefault, _, _ := unstructured.NestedBool(dashboard.Object, "spec", "default"); isDefault {
			return
		}
	}

	dashboard := defaultWorkspaceDashboard(namespace)
	// JSON is a subset of YAML, so the marshalled dashboard can be fed
	// straight into the yaml-based generic create.
	raw, err := json.Marshal(dashboard)
	if err != nil {
		logger.Error("ensure default workspace dashboard: failed to marshal dashboard", "error", err)
		return
	}
	if _, err := kubernetes.CreateUnstructuredResource(
		utils.WorkspaceDashboardResource.ApiVersion,
		utils.WorkspaceDashboardResource.Plural,
		utils.WorkspaceDashboardResource.Namespaced,
		string(raw),
	); err != nil {
		if errors.IsAlreadyExists(err) {
			return
		}
		logger.Error("ensure default workspace dashboard: failed to create dashboard", "error", err)
		return
	}
	logger.Info("created default workspace dashboard", "name", DEFAULT_WORKSPACE_DASHBOARD_NAME, "namespace", namespace)
}