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
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workspace `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Workspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkspaceSpec   `json:"spec,omitempty"`
	Status WorkspaceStatus `json:"status,omitempty"`
}

type WorkspaceSpec struct {
	Name      string                        `json:"name,omitempty"`
	Resources []WorkspaceResourceIdentifier `json:"resources,omitempty"`
}

type WorkspaceResourceIdentifier struct {
	Id   string `json:"id,omitempty"`
	Type string `json:"type,omitempty"`
}

type WorkspaceStatus struct{}
