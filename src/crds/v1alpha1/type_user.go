package v1alpha1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ╭───────────╮
// │ CRD: User │
// ╰───────────╯

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UserList struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ListMeta `json:"metadata"`

	Items []User `json:"items"`
}

// A mogenius `User` resource contains information about a user on the mogenius
// platform.
//
// In addition it maps the mogenius user to `subjects` using the same information
// as RBAC uses. This could be `User` or `Group` references or `ServiceAccount`
// references.
//
// ## User
//
// Refers to a human or system account authenticated by Kubernetes.
//
//	```
//	subjects:
//	  - kind: User
//	    name: jane
//	    apiGroup: rbac.authorization.k8s.io
//	```
//
// ## Group
//
// Refers to a collection of users. Group memberships are typically established by the authentication provider.
//
//	```
//	subjects:
//	  - kind: Group
//	    name: operations
//	    apiGroup: rbac.authorization.k8s.io
//	```
//
// ## ServiceAccount
//
// A Kubernetes resource that acts as an identity for processes running in a Pod.
//
//	```
//	subjects:
//	  - kind: ServiceAccount
//	    name: default
//	    namespace: my-namespace
//	```
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type User struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata"`

	Spec UserSpec `json:"spec"`

	Status UserStatus `json:"status"`
}

type UserSpec struct {
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Email     string `json:"email,omitempty"`

	Subject *rbacv1.Subject `json:"subject"`
}

func NewUserSpec(firstName string, lastName string, email string) UserSpec {
	return UserSpec{
		FirstName: firstName,
		LastName:  lastName,
		Email:     email,
	}
}

type UserStatus struct{}
