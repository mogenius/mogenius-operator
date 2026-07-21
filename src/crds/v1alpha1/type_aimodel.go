package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ╭──────────────╮
// │ CRD: AiModel │
// ╰──────────────╯

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AiModelList struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ListMeta `json:"metadata"`

	Items []AiModel `json:"items"`
}

// A mogenius `AiModel` resource declares one usable AI model configuration:
// which provider SDK to speak, which model to request, where the API lives and
// which Secret holds the API key. Agents reference an AiModel by name via
// `spec.modelRef`; chat and unattended runs without an explicit reference use
// the AiModel marked as default. AiModels are only processed in the operator's
// own namespace (MO_OWN_NAMESPACE).
//
// The API key deliberately lives in a referenced Secret instead of this spec:
// Secrets carry their own RBAC and etcd encryption, several AiModels may share
// one key, and key rotation never touches the CR.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,shortName=aimodel,categories=mogenius
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SDK",type=string,JSONPath=`.spec.sdk`
// +kubebuilder:printcolumn:name="Model",type=string,JSONPath=`.spec.model`
// +kubebuilder:printcolumn:name="Default",type=boolean,JSONPath=`.spec.default`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type AiModel struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata"`

	Spec AiModelSpec `json:"spec"`

	Status AiModelStatus `json:"status,omitempty"`
}

type AiModelSpec struct {
	// Human-readable name shown in the UI.
	DisplayName string `json:"displayName,omitempty"`

	// Provider SDK used to talk to the model endpoint.
	// +kubebuilder:validation:Enum=openai;anthropic;ollama
	Sdk string `json:"sdk"`

	// Model identifier requested from the provider (e.g. "claude-sonnet-5",
	// "gpt-5", "llama3.1:8b").
	// +kubebuilder:validation:MinLength=1
	Model string `json:"model"`

	// Base URL of the provider API. Required for ollama and self-hosted
	// OpenAI-compatible endpoints; empty selects the provider's public API for
	// openai and anthropic.
	ApiUrl string `json:"apiUrl,omitempty"`

	// Reference to the Secret (same namespace) holding the API key. Optional
	// because local providers like Ollama don't authenticate.
	ApiKeySecretRef *SecretKeyRef `json:"apiKeySecretRef,omitempty"`

	// Marks this model as the cluster-wide default used whenever no explicit
	// model reference is given. Exactly one AiModel should be default; when
	// several are, the oldest wins.
	Default bool `json:"default,omitempty"`

	// Maximum number of tool calls per run; unset uses the global default.
	// +kubebuilder:validation:Minimum=1
	MaxToolCalls *int `json:"maxToolCalls,omitempty"`

	// Token budget per run; 0 means unlimited, unset uses the global default.
	// +kubebuilder:validation:Minimum=0
	MaxTokensPerRun *int64 `json:"maxTokensPerRun,omitempty"`
}

// SecretKeyRef points at one key inside a Secret in the same namespace as the
// referencing resource.
type SecretKeyRef struct {
	// Name of the Secret.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Key within the Secret's data; defaults to "API_KEY" when empty.
	Key string `json:"key,omitempty"`
}

type AiModelStatus struct {
	// Conditions describe the validation state of the model config; the
	// "Ready" condition reports whether the operator can use it (spec valid,
	// referenced Secret and key resolvable).
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Generation of the spec the conditions were last evaluated against.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}
