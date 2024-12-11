package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ╭────────────────╮
// │ CRD: Workspace │
// ╰────────────────╯

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Workspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkspaceSpec   `json:"spec,omitempty"`
	Status WorkspaceStatus `json:"status,omitempty"`
}

type WorkspaceSpec struct {
	Name        string                        `json:"name,omitempty"`
	DisplayName string                        `json:"displayName,omitempty"`
	Resources   []WorkspaceResourceIdentifier `json:"resources,omitempty"`
}

type WorkspaceResourceIdentifier struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Group     string `json:"group,omitempty"`
	Version   string `json:"version,omitempty"`
}

type WorkspaceStatus struct{}
