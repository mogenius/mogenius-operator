package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ╭─────────────────╮
// │ CRD: Permission │
// ╰─────────────────╯

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PermissionList struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Permission `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Permission struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PermissionSpec `json:"spec,omitempty"`

	Status PermissionStatus `json:"status,omitempty"`
}

// Grant permissions to a workspace for a group
type PermissionSpec struct {
	// the group for which this permission is created
	GroupName string `json:"group,omitempty"`

	// the workspace to which this permission is applied
	WorkspaceName string `json:"workspace,omitempty"`

	// grant "read" permission
	//
	// this allows both viewing the workspace itself and reading all resources within this workspace
	Read bool `json:"read,omitempty"`

	// grant "write" permission
	//
	// this allows editing the workspace which means:
	// - users are allowed to create, change and delete resources within this workspace
	Write bool `json:"write,omitempty"`

	// grant "delete" permission
	//
	// this allows deleting the workspace itself
	Delete bool `json:"delete,omitempty"`
}

func NewPermissionSpec(groupName string, workspaceName string, read bool, write bool, delete bool) PermissionSpec {
	return PermissionSpec{
		GroupName:     groupName,
		WorkspaceName: workspaceName,
		Read:          read,
		Write:         write,
		Delete:        delete,
	}
}

type PermissionStatus struct{}
