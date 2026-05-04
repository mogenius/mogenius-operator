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
// +kubebuilder:subresource:status
type PlatformConfig struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata"`

	Spec PlatformConfigSpec `json:"spec"`

	Status PlatformConfigStatus `json:"status,omitempty"`
}

// Specification of platform components and their configuration.
type PlatformConfigSpec struct {
	PlatformVersion string             `json:"platformVersion"`
	PlatformSource  string             `json:"platformSource,omitempty"`
	GitOps          *GitOpsConfig      `json:"gitOps"`
	CertManager     *CertManagerConfig `json:"certManager,omitempty"`
	Traefik         *TraefikConfig     `json:"traefik,omitempty"`
}
type GitOpsConfig struct {
	Engine string `json:"engine"`
}

type CertManagerConfig struct {
	Enabled bool                          `json:"enabled,omitempty"`
	Issuers []CertManagerIssuerConfig     `json:"issuers,omitempty"`
	Patch   *PlatformConfigPatchReference `json:"patch,omitempty"`
	Chart   *HelmChartReference           `json:"chart,omitempty"`
}

type TraefikConfig struct {
	Enabled bool                          `json:"enabled,omitempty"`
	Patch   *PlatformConfigPatchReference `json:"patch,omitempty"`
	Chart   *HelmChartReference           `json:"chart,omitempty"`
}

type CertManagerIssuerConfig struct {
	// Name is the name of the ClusterIssuer resource.
	Name string `json:"name"`
	// Email is the contact address for the ACME account.
	Email string `json:"email"`
	// Server is the ACME directory URL.
	// Defaults to the Let's Encrypt production endpoint when empty.
	Server string `json:"server,omitempty"`
	// HTTP01 configures the HTTP-01 challenge solver.
	// Mutually exclusive with future solver types (e.g. dns01).
	HTTP01 *CertManagerHTTP01Config `json:"http01,omitempty"`
}

// CertManagerHTTP01Config configures an ACME HTTP-01 challenge solver.
type CertManagerHTTP01Config struct {
	// IngressClass is the ingress class to use when creating the challenge ingress.
	IngressClass string `json:"ingressClass,omitempty"`
	// IngressAnnotations are extra annotations added to the challenge ingress resource.
	IngressAnnotations map[string]string `json:"ingressAnnotations,omitempty"`
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

type PlatformConfigStatus struct {
	Components []PlatformComponentStatus `json:"components,omitempty"`
}

type PlatformComponentStatus struct {
	Name     string      `json:"name"`
	Ready    bool        `json:"ready"`
	LastSync metav1.Time `json:"lastSync,omitempty"`
	Message  string      `json:"message,omitempty"`
}
