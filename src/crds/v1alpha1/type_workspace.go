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
type Workspace struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata"`

	Spec WorkspaceSpec `json:"spec"`

	Status WorkspaceStatus `json:"status"`
}

type WorkspaceSpec struct {
	Name string `json:"name,omitempty"`

	Resources []WorkspaceResourceIdentifier `json:"resources,omitempty"`
}

func NewWorkspaceSpec(displayName string, resources []WorkspaceResourceIdentifier) WorkspaceSpec {
	return WorkspaceSpec{
		Name:      displayName,
		Resources: resources,
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

type WorkspaceStatus struct{}
