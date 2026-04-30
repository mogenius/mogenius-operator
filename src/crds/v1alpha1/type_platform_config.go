package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ╭─────────────────────╮
// │ CRD: PlatformConfig │
// ╰─────────────────────╯

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PlatformConfigList struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ListMeta `json:"metadata,omitempty"`

	Items []PlatformConfig `json:"items"`
}

// A mogenius `PlatformConfig` defines the configuration for the platform.
//
// From here on the operatr will derive needed platform components and their configuration.
// For example, a `PlatformConfig` could specifiy your cert-manager configuration,
// which the operator would then apply to the cluster and keep in sync with the `PlatformConfig` resource.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster
type PlatformConfig struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata"`

	Spec PlatformConfigSpec `json:"spec"`

	Status PlatformConfigStatus `json:"status"`
}

// Specification of platform components and their configuration.
type PlatformConfigSpec struct {
	PlatformVersion string             `json:"platformVersion"`
	PlatformSource  string             `json:"platformSource,omitempty"`
	GitOps          *GitOpsConfig      `json:"gitOps,omitempty"`
	CertManager     *CertManagerConfig `json:"certManager,omitempty"`
}
type GitOpsConfig struct {
	Engine string `json:"engine,omitempty"`
}

type CertManagerConfig struct {
	Enabled bool                          `json:"enabled,omitempty"`
	Issuers []CertManagerIssuerConfig     `json:"issuers,omitempty"`
	Patch   *PlatformConfigPatchReference `json:"patch,omitempty"`
	Chart   *HelmChartReference           `json:"chart,omitempty"`
}

type CertManagerIssuerConfig struct {
	// name of the ClusterIssuer resource to use for cert-manager
	ClusterIssuerName string `json:"clusterIssuerName,omitempty"`
}

type PlatformConfigPatchReference struct {
	// name of the PlatformPatch resource to apply
	Name string `json:"name,omitempty"`
}

type HelmChartReference struct {
	// name of the HelmRelease resource to apply
	Name       string `json:"name,omitempty"`
	Chart      string `json:"chart,omitempty"`
	Version    string `json:"version,omitempty"`
	Repository string `json:"repository,omitempty"`
}

type PlatformConfigStatus struct{}
