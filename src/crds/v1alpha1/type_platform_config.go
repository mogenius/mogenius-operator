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
//
// Each component field follows the same two-state rule (see reconcileComponent
// and reconcileComponent's "Not declared" check in each platform_*.go file):
// a nil block means "not mentioned" and leaves whatever is installed alone,
// while a present block with Enabled:false actively uninstalls the component.
// This matters because the resource is synced from git with prune enabled, so
// a renamed or broken file path makes the block disappear — and treating that
// as enabled:false would tear down cert-manager, Traefik, Prometheus and Loki
// over a rename.
type PlatformConfigSpec struct {
	// PlatformVersion is the version of the base platform config that this is derived from
	PlatformVersion string `json:"platformVersion,omitempty"`
	// PlatformSource records where this spec came from
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
	AgentSandboxes          *AgentSandboxesConfig          `json:"agentSandboxes,omitempty"`
}

// GitOpsConfig picks the GitOps engine (ArgoCD xor FluxCD — whichever block
// is present decides which engine the operator detects/manages) and the
// repositories that engine syncs from. Each engine block still carries its
// own Enabled, following the normal component semantics (see
// PlatformConfigSpec): present-but-disabled uninstalls it.
type GitOpsConfig struct {
	ArgoCD       *ArgoCDInstallConfig     `json:"argocd,omitempty"`
	FluxCD       *FluxCDInstallConfig     `json:"fluxcd,omitempty"`
	Repositories []GitOpsRepositoryConfig `json:"repositories,omitempty"`
}

// The two kinds of repository a PlatformConfig can declare.
const (
	// RepositoryTypePlatform holds the platform config itself; the operator
	// turns the entry into the engine's sync objects.
	RepositoryTypePlatform = "platform"
	// RepositoryTypeApplication holds application workloads. Declared for the
	// mogenius platform to read; this operator creates no sync objects for it.
	RepositoryTypeApplication = "application"
)

// +kubebuilder:validation:XValidation:rule="!(has(self.externalSecret) && has(self.githubApp))",message="externalSecret and githubApp are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!(has(self.githubApp) && has(self.write) && has(self.write.secretRef))",message="write.secretRef cannot be combined with githubApp, the write token is minted from the app"
type GitOpsRepositoryConfig struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	// Type says what the repository holds: "platform" (the default when
	// absent — every config written before the field existed means exactly
	// that) or "application".
	// +kubebuilder:validation:Enum=platform;application
	// +optional
	Type           string          `json:"type,omitempty"`
	Path           string          `json:"path,omitempty"`
	Revision       string          `json:"revision,omitempty"`
	ExternalSecret *ExternalSecret `json:"externalSecret,omitempty"`
	// GitHubApp authenticates as a GitHub App installation. The engine
	// authenticates with the app directly; the platform commits with
	// short-lived installation tokens the operator mints on request.
	// +optional
	GitHubApp *GitHubAppConfig `json:"githubApp,omitempty"`
	// Write configures how edits made in the platform reach this repository.
	// Read by the mogenius API, not by this operator: declaring it changes
	// nothing in the cluster on its own.
	// +optional
	Write *GitOpsWriteConfig `json:"write,omitempty"`
}

// IsPlatformRepository answers whether the operator owns this entry's sync
// objects. Absent type reads as platform: every config written before the
// field existed declared exactly that.
func (r GitOpsRepositoryConfig) IsPlatformRepository() bool {
	return r.Type == "" || r.Type == RepositoryTypePlatform
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

// GitHubAppConfig identifies a GitHub App installation with access to a
// repository.
type GitHubAppConfig struct {
	// +kubebuilder:validation:Minimum=1
	AppID int64 `json:"appId"`
	// +kubebuilder:validation:Minimum=1
	InstallationID int64 `json:"installationId"`
	// EnterpriseBaseURL is the REST API root of a GitHub Enterprise Server,
	// e.g. https://github.example.com/api/v3. Empty means github.com.
	// +optional
	EnterpriseBaseURL string `json:"enterpriseBaseUrl,omitempty"`
	// PrivateKey is the app's PEM-encoded private key. secret.key and
	// externalSecret.key default to "privateKey".
	PrivateKey GenericSecretReference `json:"privateKey"`
}

// SecretReference points at a Secret already present in the
// mogenius-operator's namespace.
type SecretReference struct {
	// Name of the Secret resource to reference.
	Name string `json:"name,omitempty"`
	// Key of the Secret data to reference.
	Key string `json:"key,omitempty"`
}

// GenericSecretReference picks exactly one source for a credential: a plain
// in-cluster Secret, or an ExternalSecret the operator materializes into one.
// +kubebuilder:validation:OneOf=secret;externalSecret
type GenericSecretReference struct {
	Secret         *SecretReference `json:"secret,omitempty"`
	ExternalSecret *ExternalSecret  `json:"externalSecret,omitempty"`
}

// ArgoCDInstallConfig installs Argo CD as the GitOps engine.
type ArgoCDInstallConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
	// Project is the Argo CD AppProject name components are deployed into.
	// Falls back to the operator's default project when empty.
	Project string `json:"project,omitempty"`
}

// FluxCDInstallConfig installs Flux as the GitOps engine.
type FluxCDInstallConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
}

// CertManagerConfig installs cert-manager and the Issuers/ClusterIssuers it
// should manage alongside it.
type CertManagerConfig struct {
	Enabled        bool                           `json:"enabled,omitempty"`
	Issuers        []IssuerConfig                 `json:"issuers,omitempty"`
	ClusterIssuers []ClusterIssuerConfig          `json:"clusterIssuers,omitempty"`
	Patches        []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart          *HelmChartReference            `json:"chart,omitempty"`
}

// TraefikConfig installs Traefik as the cluster's ingress controller.
type TraefikConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
	Service *runtime.RawExtension          `json:"service,omitempty"`
}

// ExternalDNSConfig installs external-dns, which keeps DNS records in sync
// with Services/Ingresses for the given provider.
type ExternalDNSConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
	// Provider is the DNS provider name external-dns talks to (e.g. "aws",
	// "cloudflare", "google"). Also used to name the generated provider secret.
	Provider string `json:"provider"`
	// DomainFilters restricts external-dns to managing only these domains.
	DomainFilters []string `json:"domainFilters,omitempty"`
	// ExternalSecret references the provider API credentials. When its Vault
	// is empty, the operator falls back to the first vault declared in
	// spec.externalSecretsOperator.
	ExternalSecret ExternalSecret `json:"externalSecret"`
}

// KubePrometheusStackConfig installs the kube-prometheus-stack chart
// (Prometheus, Alertmanager and Grafana).
type KubePrometheusStackConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
}

// LokiConfig installs Loki for log aggregation.
type LokiConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
}

// AlloyConfig installs Grafana Alloy, the agent that ships logs/metrics to
// Loki and kube-prometheus-stack.
type AlloyConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
}

// AgentSandboxesConfig installs the mogenius agent-sandbox stack: an isolated
// namespace where untrusted agent code runs, the mogenius-provided default
// SandboxTemplate/WarmPool objects, and the network isolation around them.
//
// It reuses the existing mogenius-agent-sandbox Helm chart as the install unit;
// Namespace and NetworkPolicy template into that chart's values (see
// reconcileAgentSandboxes). Runtime-class / gVisor installation is intentionally
// not modelled here yet — it is a cluster/node-specific follow-up.
type AgentSandboxesConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
	// Namespace the sandbox workloads run in. Falls back to the chart default
	// ("agent-sandbox") when empty.
	Namespace string `json:"namespace,omitempty"`
	// NetworkPolicy configures the egress isolation around sandbox pods.
	NetworkPolicy *AgentSandboxNetworkPolicyConfig `json:"networkPolicy,omitempty"`
}

// AgentSandboxNetworkPolicyConfig configures the egress isolation the sandbox
// controller applies to each sandbox pod. It maps onto the chart's
// sandboxes.networkPolicy values.
type AgentSandboxNetworkPolicyConfig struct {
	// Managed toggles whether the sandbox controller creates a per-sandbox
	// NetworkPolicy. A pointer so leaving it unset keeps the chart default (true).
	// The default rule set already denies RFC1918 and link-local egress — the
	// latter covering the 169.254.169.254 metadata endpoint, the classic
	// SSRF/exfil vector for untrusted agent code.
	// +optional
	Managed *bool `json:"managed,omitempty"`
	// AdditionalBlockedCidrs are extra egress-blocked CIDRs on top of the
	// RFC1918 and link-local defaults — e.g. the kube-apiserver service CIDR,
	// which on some clusters (GKE) sits outside RFC1918 and must be listed
	// explicitly.
	// +optional
	// +kubebuilder:validation:items:Format=cidr
	AdditionalBlockedCidrs []string `json:"additionalBlockedCidrs,omitempty"`
}

// RenovateOperatorConfig installs renovate-operator, which runs a RenovateJob
// per declared repository to keep dependency manifests up to date.
type RenovateOperatorConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
	// MaxParallelJobs caps how many RenovateJobs run concurrently.
	MaxParallelJobs int                 `json:"maxParallelJobs,omitempty"`
	Repositories    []RenovateJobConfig `json:"repositories,omitempty"`
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

// RenovateProvider names the SCM provider a RenovateJob discovers repositories
// on (e.g. "github", "gitlab", "gitea") and, for self-hosted instances, the
// API endpoint to use instead of the provider's public default.
type RenovateProvider struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint,omitempty"`
}

// ExternalSecretsOperatorConfig installs the External Secrets Operator and
// declares the vaults other components' ExternalSecret references resolve
// against.
type ExternalSecretsOperatorConfig struct {
	Enabled bool                           `json:"enabled,omitempty"`
	Patches []PlatformConfigPatchReference `json:"patches,omitempty"`
	Chart   *HelmChartReference            `json:"chart,omitempty"`
	Vaults  []ExternalSecretVault          `json:"vaults"`
}

// ExternalSecretVault names a backend (e.g. Vault, AWS Secrets Manager) the
// External Secrets Operator's SecretStore is created for. Provider holds the
// backend-specific connection config as raw JSON, since its shape differs
// per vault type.
type ExternalSecretVault struct {
	Name     string               `json:"name"`
	Type     string               `json:"type"`
	Provider runtime.RawExtension `json:"provider"`
}

// ExternalSecret points at one secret inside a vault declared in
// spec.externalSecretsOperator.vaults. Vault may be left empty by the caller
// and defaulted to the first declared vault (see e.g. ExternalDNSConfig).
type ExternalSecret struct {
	Vault string `json:"vault,omitempty"`
	Path  string `json:"path"`
	Key   string `json:"key,omitempty"`
}

// IssuerConfig declares a cert-manager Issuer (namespace-scoped) for cert-manager to create.
type IssuerConfig struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Namespace string `json:"namespace"`
	Server    string `json:"server,omitempty"`
	// Solvers are raw cert-manager ACME challenge solver objects (HTTP-01,
	// DNS-01, …), passed through untyped since their shape varies per solver.
	Solvers []runtime.RawExtension `json:"solvers,omitempty"`
}

// ClusterIssuerConfig declares a cert-manager ClusterIssuer (cluster-scoped,
// usable by Certificates in any namespace) for cert-manager to create.
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

// PlatformConfigPatchReference names a PlatformPatch resource (see
// type_platform_patch.go) applied on top of a component's generated chart
// values/objects, for overrides this CRD has no dedicated field for.
type PlatformConfigPatchReference struct {
	// name of the PlatformPatch resource to apply
	Name string `json:"name,omitempty"`
}

// HelmChartReference overrides where a component's Helm chart is pulled from
// and what it's installed as. Every field is optional: an absent field falls
// back to the component's hardcoded default (see e.g. defaultChart/defaultRepo
// in componentSpec), so this only needs to be set to pin a version or point at
// a mirror.
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
	// ConfigSource reports who owns this resource's spec.
	// +optional
	ConfigSource *PlatformConfigSource `json:"configSource,omitempty"`
	// Repositories mirrors spec.gitOps.repositories with the credentials the
	// operator resolved for them, so the platform never derives Secret names
	// on its own.
	// +optional
	Repositories []GitOpsRepositoryStatus `json:"repositories,omitempty"`
}

// Kinds of credential a GitOpsCredentialReference points at.
const (
	// GitOpsCredentialKindToken is a Secret holding a token under Key.
	GitOpsCredentialKindToken = "token"
	// GitOpsCredentialKindGitHubApp is a Secret holding GitHub App
	// credentials. Tokens are minted from it through the operator's
	// platform/github-app-token command.
	GitOpsCredentialKindGitHubApp = "githubApp"
)

// GitOpsRepositoryStatus reports one entry of spec.gitOps.repositories.
type GitOpsRepositoryStatus struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	// Type is the entry's type with the default applied.
	Type string `json:"type"`
	// WriteCredential is the credential the platform commits with. Absent when
	// the spec declares none; the platform then uses the token it stored
	// itself.
	// +optional
	WriteCredential *GitOpsCredentialReference `json:"writeCredential,omitempty"`
}

// GitOpsCredentialReference locates a credential Secret.
type GitOpsCredentialReference struct {
	// +kubebuilder:validation:Enum=token;githubApp
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	// Key holds the token for kind token.
	// +optional
	Key string `json:"key,omitempty"`
}

// PlatformConfigSource says where the spec in the cluster came from.
//
// The mogenius API switches on this: while it reads "cluster" the API edits the
// resource directly, and once it reads "git" it commits platformconfig.yaml
// instead, because a direct write would be reverted by the engine's next sync.
//
// It only turns to "git" on evidence that an engine actually applied this
// object — never on a repository merely being configured. A repository that is
// set up but not syncing (wrong path, bad credential, controllers still coming
// up) therefore leaves the platform editable rather than locking the user out
// of their own configuration.
//
// As with GitOpsStatus, no field carries `omitempty`: the status is written as
// a JSON merge patch, where an omitted key keeps its previous value, and a
// stale revision would outlive the sync that produced it.
type PlatformConfigSource struct {
	// Source is "cluster" or "git".
	// +kubebuilder:validation:Enum=cluster;git
	// +optional
	Source string `json:"source"`
	// Revision the engine last applied from the repository, when it reports
	// one — Flux's lastAppliedRevision, Argo CD's synced revision. Empty while
	// Source is "cluster".
	// +optional
	Revision string `json:"revision"`
	// SyncedBy names the object that applied this resource, for pointing a user
	// at the right place when a sync goes wrong: the Flux Kustomization or the
	// Argo CD Application, as "namespace/name".
	// +optional
	SyncedBy string `json:"syncedBy"`
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
