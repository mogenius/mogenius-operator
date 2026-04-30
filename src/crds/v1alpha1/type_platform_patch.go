package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ╭────────────────────╮
// │ CRD: PlatformPatch │
// ╰────────────────────╯

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PlatformPatchList struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ListMeta `json:"metadata,omitempty"`

	Items []PlatformPatch `json:"items"`
}

// A mogenius `PlatformPatch` defines patches that can be applied to the configuration of a platform component.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster
type PlatformPatch struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata"`

	Spec PlatformPatchSpec `json:"spec"`

	Status PlatformPatchStatus `json:"status,omitempty"`
}

// Patches to be applied to the configuration of a platform component.
type PlatformPatchSpec struct {
	// helm values to be merged into the configuration of a platform component.
	ValuesObject *apiextensionsv1.JSON `json:"valuesObject,omitempty"`
	// extra Kubernetes objects to be applied to the cluster.
	ExtraObjects []runtime.RawExtension `json:"extraObjects,omitempty"`
}

type PlatformPatchStatus struct{}
