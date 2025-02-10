package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ╭───────────╮
// │ CRD: Team │
// ╰───────────╯

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TeamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Team `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Team struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TeamSpec   `json:"spec,omitempty"`
	Status TeamStatus `json:"status,omitempty"`
}

type TeamSpec struct {
	// name for this group
	//
	// the name has to be unique across users and groups
	DisplayName string `json:"displayName,omitempty"`

	// a list of usernames within this group
	Users []string `json:"users,omitempty"`
}

func NewTeamSpec(displayName string, users []string) TeamSpec {
	return TeamSpec{
		DisplayName: displayName,
		Users:       users,
	}
}

type TeamStatus struct{}
