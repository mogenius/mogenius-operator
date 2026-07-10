package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ╭────────────╮
// │ CRD: Agent │
// ╰────────────╯

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AgentList struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ListMeta `json:"metadata"`

	Items []Agent `json:"items"`
}

// A mogenius `Agent` resource defines an AI agent that observes a scoped set
// of namespaces (read-only) and proposes tasks which a user must approve or
// reject before anything is executed.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Agent struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata"`

	Spec AgentSpec `json:"spec"`

	Status AgentStatus `json:"status"`
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

	// Optional model override; empty uses the cluster-wide configured model.
	Model string `json:"model,omitempty"`
}

// AgentScope restricts an agent's visibility. At least one of WorkspaceRef or
// Namespaces must be set; when both are set the union applies.
type AgentScope struct {
	// Name of a Workspace CR whose namespace resources define the scope.
	WorkspaceRef string `json:"workspaceRef,omitempty"`

	// Explicit list of namespaces the agent may see. The single entry "*"
	// grants visibility into all namespaces (resolved at run time).
	Namespaces []string `json:"namespaces,omitempty"`
}

type AgentTriggers struct {
	// Event filters evaluated against watched cluster resources.
	Events []AgentEventFilter `json:"events,omitempty"`

	// Standard 5-field cron expression for periodic runs; empty disables.
	Cron string `json:"cron,omitempty"`

	// Allow triggering a run manually from the UI.
	Manual bool `json:"manual,omitempty"`
}

// AgentEventFilter matches watched resources by kind and JSONPath conditions
// (same semantics as the former AiFilter mechanic).
type AgentEventFilter struct {
	Id   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`

	Kind string `json:"kind"`

	// JSONPath → expected value; any match selects the object.
	Contains map[string]string `json:"contains,omitempty"`

	// JSONPath → value; any match excludes the object.
	Excludes map[string]string `json:"excludes,omitempty"`

	// Prompt for the analysis run triggered by this filter.
	Prompt string `json:"prompt,omitempty"`

	// Condition must hold for this duration before a task is created.
	For *metav1.Duration `json:"for,omitempty"`
}

type AgentStatus struct{}
