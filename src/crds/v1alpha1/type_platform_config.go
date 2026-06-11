package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	PlatformVersion         string                         `json:"platformVersion"`
	PlatformSource          string                         `json:"platformSource,omitempty"`
	GitOps                  *GitOpsConfig                  `json:"gitOps"`
	CertManager             *CertManagerConfig             `json:"certManager,omitempty"`
	Traefik                 *TraefikConfig                 `json:"traefik,omitempty"`
	ExternalDNS             *ExternalDNSConfig             `json:"externalDns,omitempty"`
	KubePrometheusStack     *KubePrometheusStackConfig     `json:"kubePrometheusStack,omitempty"`
	Loki                    *LokiConfig                    `json:"loki,omitempty"`
	Alloy                   *AlloyConfig                   `json:"alloy,omitempty"`
	RenovateOperator        *RenovateOperatorConfig        `json:"renovateOperator,omitempty"`
	ExternalSecretsOperator *ExternalSecretsOperatorConfig `json:"externalSecretsOperator,omitempty"`
}
type GitOpsConfig struct {
	ArgoCD *ArgoCDInstallConfig `json:"argocd,omitempty"`
	FluxCD *FluxCDInstallConfig `json:"fluxcd,omitempty"`
}

type ArgoCDInstallConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
}

type FluxCDInstallConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
}

type CertManagerConfig struct {
	Enabled        bool                           `json:"enabled,omitempty"`
	Issuers        []IssuerConfig                 `json:"issuers,omitempty"`
	ClusterIssuers []ClusterIssuerConfig          `json:"clusterIssuers,omitempty"`
	Patches        []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart          *HelmChartReference            `json:"chart,omitempty"`
}

type TraefikConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
	Service *runtime.RawExtension          `json:"service,omitempty"`
}

type ExternalDNSConfig struct {
	Enabled        bool                           `json:"enabled,omitempty"`
	Patches        []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart          *HelmChartReference            `json:"chart,omitempty"`
	Provider       string                         `json:"provider"`
	DomainFilters  []string                       `json:"domainFilters,omitempty"`
	ExternalSecret ExternalSecret                 `json:"externalSecret"`
}

type KubePrometheusStackConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
}

type LokiConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
}

type AlloyConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
}

type RenovateOperatorConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
}

type ExternalSecretsOperatorConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
	Vaults  []ExternalSecretVault          `json:"vaults"`
}

type ExternalSecretVault struct {
	Name                    string            `json:"name"`
	Type                    string            `json:"type"`
	ServiceAccountSecretRef ServiceAccountRef `json:"serviceAccountSecretRef"`
}

type ServiceAccountRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type ExternalSecret struct {
	Vault string `json:"vault"`
	Path  string `json:"path"`
	Key   string `json:"key"`
}

type IssuerConfig struct {
	Name      string                 `json:"name"`
	Email     string                 `json:"email"`
	Namespace string                 `json:"namespace"`
	Solvers   []runtime.RawExtension `json:"solvers,omitempty"`
}

type ClusterIssuerConfig struct {
	Name    string                 `json:"name"`
	Email   string                 `json:"email"`
	Solvers []runtime.RawExtension `json:"solvers,omitempty"`
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
	Namespace  string `json:"namespace,omitempty"`
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
