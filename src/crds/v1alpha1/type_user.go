package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ╭───────────╮
// │ CRD: User │
// ╰───────────╯

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UserList struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ListMeta `json:"metadata,omitempty"`

	Items []User `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type User struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec UserSpec `json:"spec,omitempty"`

	Status UserStatus `json:"status,omitempty"`
}

type UserSpec struct {
	// mogenius identifier
	MogeniusId string `json:"mogeniusId,omitempty" validate:"required"`

	// TODO: to manage access through kubectl we will have to store references to kubernetes (service-)accounts
	// KubernetesId string `json:"kubernetesId,omitempty"`
}

func NewUserSpec(mogeniusId string) UserSpec {
	return UserSpec{
		MogeniusId: mogeniusId,
	}
}

type UserStatus struct{}
