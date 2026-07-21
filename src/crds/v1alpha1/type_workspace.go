package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ╭────────────────╮
// │ CRD: Workspace │
// ╰────────────────╯

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkspaceList struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ListMeta `json:"metadata"`

	Items []Workspace `json:"items"`
}

// A mogenius `Workspace` resource contains references to all resources included
// in a workspace. In addition it contains a human-readable name for the workspace.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
type Workspace struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata"`

	Spec WorkspaceSpec `json:"spec"`

	Status WorkspaceStatus `json:"status,omitempty"`
}

type WorkspaceSpec struct {
	Name string `json:"name,omitempty"`

	Resources []WorkspaceResourceIdentifier `json:"resources,omitempty"`

	// DashboardRef optionally references a WorkspaceDashboard (by name, in the
	// same namespace) that configures which resource types the workspace
	// dashboard shows. When empty, the dashboard falls back to the built-in
	// default (Deployments, StatefulSets, DaemonSets).
	DashboardRef string `json:"dashboardRef,omitempty"`
}

func NewWorkspaceSpec(displayName string, resources []WorkspaceResourceIdentifier, dashboardRef string) WorkspaceSpec {
	return WorkspaceSpec{
		Name:         displayName,
		Resources:    resources,
		DashboardRef: dashboardRef,
	}
}

type WorkspaceResourceIdentifier struct {
	// target entity identifier (name)
	Id string `json:"id,omitempty"`

	// allowed values: "namespace", "helm", "argocd"
	Type string `json:"type,omitempty"`

	// Type=="namespace": unused
	// Type=="helm": namespace in which the chart was installed
	// Type=="argocd": namespace in which the application was installed
	Namespace string `json:"namespace,omitempty"`
}

// WorkspaceConditionResourcesValid reports whether all resources referenced by
// the workspace (namespaces, helm releases) exist in the cluster.
const WorkspaceConditionResourcesValid = "ResourcesValid"

// WorkspaceConditionDashboardRefValid reports whether the WorkspaceDashboard
// referenced by spec.dashboardRef exists. The workspace reconciler owns this
// condition; the K8s API server cannot enforce cross-object references itself.
const WorkspaceConditionDashboardRefValid = "DashboardRefValid"

type WorkspaceStatus struct {
	// Conditions reports the results of the reconciler's integrity checks.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
