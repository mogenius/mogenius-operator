package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ╭─────────────────────────╮
// │ CRD: WorkspaceDashboard │
// ╰─────────────────────────╯

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkspaceDashboardList struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ListMeta `json:"metadata"`

	Items []WorkspaceDashboard `json:"items"`
}

// A mogenius `WorkspaceDashboard` resource configures the workspace dashboard:
// the built-in standard components can be toggled on or off, and any number of
// resource tables can be added below them. Workspaces opt in by referencing a
// WorkspaceDashboard via `spec.dashboardRef`; several workspaces may share the
// same dashboard. Workspaces without a reference fall back to the built-in
// default layout.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
type WorkspaceDashboard struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata"`

	Spec WorkspaceDashboardSpec `json:"spec"`

	// omitempty keeps status optional in the generated CRD schema, so clients
	// creating dashboards (e.g. the frontend via the generic resource API)
	// don't have to send an empty status object.
	Status WorkspaceDashboardStatus `json:"status,omitempty"`
}

type WorkspaceDashboardSpec struct {
	// Default marks this dashboard as the cluster-wide default: it applies to
	// every workspace that does not reference a dashboard explicitly via
	// spec.dashboardRef.
	Default bool `json:"default,omitempty"`

	// DashboardHeader toggles the workspace header component.
	DashboardHeader DashboardToggle `json:"dashboardHeader,omitempty"`

	// CapacitiesTile toggles the workspace capacities tile.
	CapacitiesTile DashboardToggle `json:"capacitiesTile,omitempty"`

	// AiInsightsBanner toggles the AI insights banner.
	AiInsightsBanner DashboardToggle `json:"aiInsightsBanner,omitempty"`

	// MembersTile toggles the workspace members tile.
	MembersTile DashboardToggle `json:"membersTile,omitempty"`

	// ResourceOverview toggles and parameterises the aggregated resource overview.
	ResourceOverview DashboardResourceOverview `json:"resourceOverview,omitempty"`

	// ResourceTables lists resource tables rendered below the standard
	// components, in display order.
	ResourceTables []DashboardResourceTable `json:"resourceTables,omitempty"`
}

// DashboardToggle switches one of the built-in dashboard components on or off.
// Omitted toggles count as disabled.
type DashboardToggle struct {
	Enabled bool `json:"enabled,omitempty"`
}

// DashboardResourceOverview configures the aggregated resource overview component.
type DashboardResourceOverview struct {
	Enabled bool `json:"enabled,omitempty"`

	// Header is an optional heading rendered above the component.
	Header string `json:"header,omitempty"`

	// Resources identifies the resource types aggregated by the overview.
	// Optional: when omitted, consumers render their built-in default kinds.
	Resources []CrdReference `json:"resources,omitempty"`
}

// DashboardResourceTable configures one resource table shown on the dashboard.
type DashboardResourceTable struct {
	// Header is an optional heading rendered above the table.
	Header string `json:"header,omitempty"`

	// Resources identifies the resource types listed in this table.
	// +kubebuilder:validation:MinItems=1
	Resources []CrdReference `json:"resources"`
}

// WorkspaceDashboardConditionResourcesValid reports whether all resource types
// referenced by the dashboard's components exist in the cluster. The dashboard
// reconciler owns this condition.
const WorkspaceDashboardConditionResourcesValid = "ResourcesValid"

type WorkspaceDashboardStatus struct {
	// Conditions reports the results of the reconciler's integrity checks,
	// e.g. whether all referenced resource types exist in the cluster.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}