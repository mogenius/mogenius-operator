package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ╭─────────────────╮
// │ CRD: GroupGrant │
// ╰─────────────────╯

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type GroupGrantList struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ListMeta `json:"metadata,omitempty"`

	Items []GroupGrant `json:"items"`
}

// A mogenius `GroupGrant` is a RULE that maps an identity-provider group
// (by its group name, delivered in the login token's groups claim) to a
// role on a mogenius `Workspace` — or on every workspace of the cluster.
//
// Unlike `Grant` (which materializes access for a concrete `User` resource),
// a GroupGrant is declarative input: the mogenius platform reconciles it at
// login time and creates/updates the matching `Grant` resources for each
// user that carries the group in their claim.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Claim Value",type=string,JSONPath=`.spec.claimValue`
// +kubebuilder:printcolumn:name="Role",type=string,JSONPath=`.spec.role`
// +kubebuilder:printcolumn:name="Target Type",type=string,JSONPath=`.spec.targetType`
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.targetName`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type GroupGrant struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata"`

	Spec GroupGrantSpec `json:"spec"`

	// optional so plain `kubectl apply` manifests (GitOps) validate without a
	// status stanza — GroupGrants are customer-authored rules
	// +optional
	Status GroupGrantStatus `json:"status,omitempty"`
}

// Maps an IdP group to a workspace (or the whole cluster)
type GroupGrantSpec struct {
	// the IdP group name as it appears in the login token's groups claim
	// (free-form, chosen by the customer at their identity provider)
	ClaimValue string `json:"claimValue,omitempty"`

	// type of grant:
	//
	// - "workspace"
	// - "cluster" (applies to every workspace of the cluster)
	TargetType string `json:"targetType,omitempty"`

	// to which specific resource is the grant applied:
	//
	// - workspace.meta.name (empty for targetType "cluster")
	TargetName string `json:"targetName,omitempty"`

	// FALLBACK role, only used when the login claim carries no role for the
	// group. IdPs that pair groups with roles (e.g. Entra enterprise app
	// role assignments, delivered as "<group>:<role>") override this — the
	// claim role always wins.
	//
	// - "viewer"
	// - "editor"
	// - "admin"
	Role string `json:"role,omitempty"`
}

func NewGroupGrantSpec(claimValue string, targetType string, targetName string, role string) GroupGrantSpec {
	return GroupGrantSpec{
		ClaimValue: claimValue,
		TargetType: targetType,
		TargetName: targetName,
		Role:       role,
	}
}

type GroupGrantStatus struct{}
