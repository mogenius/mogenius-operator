package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ╭────────────╮
// │ CRD: Grant │
// ╰────────────╯

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Grant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GrantSpec   `json:"spec,omitempty"`
	Status GrantStatus `json:"status,omitempty"`
}

type GrantSpec struct {
	WorkspaceName  string `json:"workspace,omitempty"`
	PermissionName string `json:"permission,omitempty"`
	GroupName      string `json:"group,omitempty"`
}

type GrantStatus struct{}
