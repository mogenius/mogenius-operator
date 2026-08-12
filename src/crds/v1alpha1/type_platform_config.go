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
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.platformVersion`
// +kubebuilder:printcolumn:name="GitOps",type=string,JSONPath=`.status.gitOpsStatus.engine`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type PlatformConfig struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata"`

	Spec PlatformConfigSpec `json:"spec"`

	Status PlatformConfigStatus `json:"status,omitempty"`
}

// Specification of platform components and their configuration.
//
// Every field is optional so that the default PlatformConfig the operator
// creates at startup — an empty spec that only carries the GitOps status —
// passes schema validation.
type PlatformConfigSpec struct {
	PlatformVersion         string                         `json:"platformVersion,omitempty"`
	PlatformSource          string                         `json:"platformSource,omitempty"`
	GitOps                  *GitOpsConfig                  `json:"gitOps,omitempty"`
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
	ArgoCD       *ArgoCDInstallConfig     `json:"argocd,omitempty"`
	FluxCD       *FluxCDInstallConfig     `json:"fluxcd,omitempty"`
	Repositories []GitOpsRepositoryConfig `json:"repositories,omitempty"`
}

type GitOpsRepositoryConfig struct {
	Name           string          `json:"name"`
	URL            string          `json:"url"`
	Path           string          `json:"path,omitempty"`
	Revision       string          `json:"revision,omitempty"`
	ExternalSecret *ExternalSecret `json:"externalSecret,omitempty"`
	// Write configures how edits made in the platform reach this repository.
	// Read by the mogenius API, not by this operator: declaring it changes
	// nothing in the cluster on its own.
	// +optional
	Write *GitOpsWriteConfig `json:"write,omitempty"`
}

// GitOpsWriteConfig overrides how the platform commits to a GitOps repository.
//
// Every field is optional. The credential Secret is what actually enables
// editing, so an absent write block still allows it — this only overrides the
// defaults the API would otherwise derive from the cluster.
type GitOpsWriteConfig struct {
	// Enabled is false to refuse edits for this repository even when a
	// credential exists. Absent means enabled.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// Mode is "DIRECT_COMMIT" or "PULL_REQUEST". Defaults to PULL_REQUEST.
	// +kubebuilder:validation:Enum=DIRECT_COMMIT;PULL_REQUEST
	// +optional
	Mode string `json:"mode,omitempty"`
	// SecretRef names the Secret in the mogenius-operator`s namespace holding the write credentials.
	// secretRef.secret - reference a secret in the mogenius-operator`s namespace
	// secretRef.externalSecret - reference a secret in an external vault,
	// the operator will automatically create the externalsecret for you in the mogenius-operator`s namespace.
	// +optional
	SecretRef *GenericSecretReference `json:"secretRef,omitempty"`
	// Provider is "GIT_HUB", "GIT_LAB" or "GITEA". Derived from url when empty.
	// +optional
	Provider string `json:"provider,omitempty"`
}

type SecretReference struct {
	// Name of the Secret resource to reference.
	Name string `json:"name,omitempty"`
	// Key of the Secret data to reference.
	Key string `json:"key,omitempty"`
}

// +kubebuilder:validation:OneOf=secret;externalSecret
type GenericSecretReference struct {
	Secret         *SecretReference `json:"secret,omitempty"`
	ExternalSecret *ExternalSecret  `json:"externalSecret,omitempty"`
}

type ArgoCDInstallConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
	Project string                         `json:"project,omitempty"`
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
	Enabled         bool                           `json:"enabled,omitempty"`
	Patches         []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart           *HelmChartReference            `json:"chart,omitempty"`
	MaxParallelJobs int                            `json:"maxParallelJobs,omitempty"`
	Repositories    []RenovateJobConfig            `json:"repositories,omitempty"`
}

type RenovateJobConfig struct {
	// Name of the RenovateJob resource. Defaults to the gitOpsRepository name when set.
	Name string `json:"name,omitempty"`
	// GitOpsRepository references a repository from spec.gitOps.repositories by name.
	// Its name is used as the discoverTopics filter.
	GitOpsRepository string `json:"gitOpsRepository,omitempty"`
	// Filter is a discovery topic used when not referencing a gitops repository.
	Filter         string           `json:"filter,omitempty"`
	Provider       RenovateProvider `json:"provider"`
	Schedule       string           `json:"schedule,omitempty"`
	ExternalSecret *ExternalSecret  `json:"externalSecret,omitempty"`
}

type RenovateProvider struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint,omitempty"`
}

type ExternalSecretsOperatorConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
	Vaults  []ExternalSecretVault          `json:"vaults"`
}

type ExternalSecretVault struct {
	Name     string               `json:"name"`
	Type     string               `json:"type"`
	Provider runtime.RawExtension `json:"provider"`
}

type ExternalSecret struct {
	Vault string `json:"vault,omitempty"`
	Path  string `json:"path"`
	Key   string `json:"key,omitempty"`
}

type IssuerConfig struct {
	Name      string                 `json:"name"`
	Email     string                 `json:"email"`
	Namespace string                 `json:"namespace"`
	Server    string                 `json:"server,omitempty"`
	Solvers   []runtime.RawExtension `json:"solvers,omitempty"`
}

type ClusterIssuerConfig struct {
	Name    string                 `json:"name"`
	Email   string                 `json:"email"`
	Server  string                 `json:"server,omitempty"`
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
	Conditions   []metav1.Condition `json:"conditions,omitempty"`
	GitOpsStatus *GitOpsStatus      `json:"gitOpsStatus,omitempty"`
}

// GitOpsStatus reports whether, where and which GitOps engine runs on this
// cluster. No field carries `omitempty`: the status is written with a JSON
// merge patch, where an omitted key keeps its previous value — stale engine
// data would survive an engine being removed or replaced.
type GitOpsStatus struct {
	// Engine identity, "argo-cd" or "flux".
	// +optional
	Engine string `json:"engine"`
	// Installed is true when an engine was actually found on the cluster.
	// +optional
	Installed bool `json:"installed"`
	// +optional
	Namespace string `json:"namespace"`
	// +optional
	ReleaseName string `json:"releaseName"`
	// Version of the engine, e.g. "v2.7.5".
	// +optional
	Version string `json:"version"`
	// IsUserManaged is true when the engine is not installed and owned by mogenius.
	// +optional
	IsUserManaged bool `json:"isUserManaged"`
	// Source is "spec" when the information comes from spec.gitOps and
	// "detected" when it comes from probing the cluster.
	// +optional
	Source string `json:"source"`
	// DefaultProjectName is the ArgoCD AppProject mogenius deploys into.
	// +optional
	DefaultProjectName string `json:"defaultProjectName"`
	// Controllers are the engine's controller deployments.
	// +optional
	Controllers []GitOpsControllerStatus `json:"controllers"`
}

type GitOpsControllerStatus struct {
	// +optional
	Name string `json:"name"`
	// +optional
	Version string `json:"version"`
	// +optional
	Ready bool `json:"ready"`
}
