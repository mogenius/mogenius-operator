package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ╭────────────╮
// │ CRD: Agent │
// ╰────────────╯

// AgentRunRequestedAtAnnotation triggers a one-off manual run when its value
// changes (Flux-style). Set it to a fresh timestamp to request a run:
//
//	kubectl annotate agent.mogenius.com <name> -n mogenius \
//	  "mogenius.com/run-requested-at=$(date -u +%FT%T.%NZ)" --overwrite
const AgentRunRequestedAtAnnotation = "mogenius.com/run-requested-at"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AgentList struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ListMeta `json:"metadata"`

	Items []Agent `json:"items"`
}

// A mogenius `Agent` resource defines an AI agent that observes a scoped set
// of namespaces (read-only) and proposes tasks which a user must approve or
// reject before anything is executed. Agents are only processed in the
// operator's own namespace (MO_OWN_NAMESPACE).
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,shortName=aiagent,categories=mogenius
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Enabled",type=boolean,JSONPath=`.spec.enabled`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Cron",type=string,JSONPath=`.spec.triggers.cron`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type Agent struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata"`

	Spec AgentSpec `json:"spec"`

	Status AgentStatus `json:"status,omitempty"`
}

type AgentSpec struct {
	// Human-readable name shown in the UI.
	DisplayName string `json:"displayName,omitempty"`

	// Optional description of what this agent looks after.
	Description string `json:"description,omitempty"`

	// Optional icon identifier (e.g. a Font Awesome name like "fa-broom")
	// shown next to the agent in the UI.
	Icon string `json:"icon,omitempty"`

	// Agent-specific instruction appended to the base system prompt for
	// every run of this agent.
	Instruction string `json:"instruction,omitempty"`

	// Disabled agents neither trigger nor process tasks.
	Enabled bool `json:"enabled"`

	// What the agent is allowed to see. Agents are always read-only; the
	// scope additionally restricts reads to the resolved namespaces.
	Scope AgentScope `json:"scope"`

	Triggers AgentTriggers `json:"triggers,omitempty"`

	// Name of the AiModel CR (same namespace) this agent runs with; empty uses
	// the cluster-wide default AiModel.
	ModelRef string `json:"modelRef,omitempty"`

	// Deprecated: overrides only the model name within the globally configured
	// provider. Use ModelRef instead, which selects a full AiModel (provider,
	// URL and credentials). Ignored when ModelRef is set.
	Model string `json:"model,omitempty"`
}

// AgentScope restricts an agent's visibility. At least one of WorkspaceRef or
// Namespaces must be set; when both are set the union applies.
//
// +kubebuilder:validation:XValidation:rule="(has(self.workspaceRef) && self.workspaceRef != \"\") || (has(self.namespaces) && self.namespaces.size() > 0)",message="scope must reference a workspace or list at least one namespace"
type AgentScope struct {
	// Name of a Workspace CR whose namespace resources define the scope.
	WorkspaceRef string `json:"workspaceRef,omitempty"`

	// Explicit list of namespaces the agent may see. The single entry "*"
	// grants visibility into all namespaces (resolved at run time).
	// +kubebuilder:validation:items:Pattern=`^(\*|[a-z0-9]([-a-z0-9]*[a-z0-9])?)$`
	// +kubebuilder:validation:items:MaxLength=63
	Namespaces []string `json:"namespaces,omitempty"`
}

// AgentTriggers declares when an agent runs. A manual run is always available
// for an enabled agent (via the UI or the "mogenius.com/run-requested-at"
// annotation) and needs no field here.
type AgentTriggers struct {
	// Standard 5-field cron expression for periodic runs; empty disables.
	Cron string `json:"cron,omitempty"`

	// When set, the agent runs a whole-scope analysis whenever a matching
	// cluster resource changes (rate limited by MinInterval).
	OnChange *AgentChangeTrigger `json:"onChange,omitempty"`
}

// AgentChangeTrigger runs the agent when resources in its scope change. There
// are no JSONPath conditions — the agent's instruction decides relevance; the
// trigger only decides which change signals wake it up.
type AgentChangeTrigger struct {
	// Resource kinds whose changes trigger a run (e.g. "Deployment", "Job").
	// Empty means every watched kind.
	Kinds []string `json:"kinds,omitempty"`

	// Which change types trigger a run: any of "created", "updated",
	// "deleted". Empty means all of them.
	// +kubebuilder:validation:items:Enum=created;updated;deleted
	On []string `json:"on,omitempty"`

	// Minimum time between change-triggered runs of this agent — the cooldown
	// that prevents a burst of changes from starting many runs. Defaults to 6h
	// when unset.
	// +optional
	MinInterval metav1.Duration `json:"minInterval,omitempty"`
}

type AgentStatus struct {
	// Conditions describe the validation state of the agent; the "Ready"
	// condition reports whether the operator accepts and processes it.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Generation of the spec the conditions were last evaluated against.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastHandledTriggerAt echoes the value of the
	// "mogenius.com/run-requested-at" annotation the operator has already
	// acted on, so a manual trigger fires exactly once per distinct value.
	// +optional
	LastHandledTriggerAt string `json:"lastHandledTriggerAt,omitempty"`
}
