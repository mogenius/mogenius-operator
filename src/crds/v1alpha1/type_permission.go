package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ╭─────────────────╮
// │ CRD: Permission │
// ╰─────────────────╯

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Permission struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PermissionSpec   `json:"spec,omitempty"`
	Status PermissionStatus `json:"status,omitempty"`
}

type PermissionSpec struct {
	Name  string   `json:"name,omitempty"`
	Verbs []string `json:"verbs,omitempty"`
}

type PermissionStatus struct{}
