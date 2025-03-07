package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ╭────────────╮
// │ CRD: Grant │
// ╰────────────╯

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type GrantList struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Grant `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Grant struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec GrantSpec `json:"spec,omitempty"`

	Status GrantStatus `json:"status,omitempty"`
}

// Grant permissions to a workspace for a group
type GrantSpec struct {
	// who is granted permission
	//
	// - user.meta.name
	// - team.meta.name
	Grantee string `json:"grantee,omitempty"`

	// type of grant:
	//
	// - "workspace"
	TargetType string `json:"targetType,omitempty"`

	// to which specific resource is the grant applied:
	//
	// - workspace.meta.name
	TargetName string `json:"targetName,omitempty"`

	// which permissions are granted:
	//
	// - "viewer"
	// - "editor"
	// - "admin"
	Role string `json:"role,omitempty"`
}

func NewGrantSpec(grantee string, targetType string, targetName string, role string) GrantSpec {
	return GrantSpec{
		Grantee:    grantee,
		TargetType: targetType,
		TargetName: targetName,
		Role:       role,
	}
}

type GrantStatus struct{}
