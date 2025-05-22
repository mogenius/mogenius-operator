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

// A mogenius `Grant` assigns permissions for mogenius `User` or `Team`
// resources to mogenius `Workspace` resources.
//
// In the first iteration we chose to keep existing hardcoded `Role` definitions as
// found ob the mogenius web platform ("viewer", "editor" and "admin"). These roles
// are only designed with the website in mind and are not yet speaking the same language
// as RBAC `Role` and `ClusterRole` resources.
//
// However this system could be easily extended by another resource (e.g. a mogenius `Role` resource)
// which could then allow in-depth configuration for what users should be allowed to do.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Grant struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata"`

	Spec GrantSpec `json:"spec"`

	Status GrantStatus `json:"status"`
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
