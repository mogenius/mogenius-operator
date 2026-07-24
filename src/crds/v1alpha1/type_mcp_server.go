package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ╭─────────────────╮
// │ CRD: McpServer  │
// ╰─────────────────╯

// McpTransportType is the protocol used to communicate with the MCP server.
type McpTransportType string

const (
	McpTransportStreamableHTTP McpTransportType = "streamableHttp"
	McpTransportSSE            McpTransportType = "sse"
)

// McpAuthType is the authentication scheme used when connecting to an MCP server.
type McpAuthType string

const (
	McpAuthNone   McpAuthType = "none"
	McpAuthBearer McpAuthType = "bearer"
	McpAuthAPIKey McpAuthType = "apiKey"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type McpServerList struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ListMeta `json:"metadata"`

	Items []McpServer `json:"items"`
}

// A mogenius McpServer resource declares an external MCP server that AI agents
// can use as a source of additional tools. Agents reference one or more McpServer
// CRs via spec.mcpServerRefs; the operator connects to the server, discovers its
// tools, and makes them available to the agent for every run.
// McpServers are only processed in the operator's own namespace (MO_OWN_NAMESPACE).
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,shortName=mcpserver,categories=mogenius
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Transport",type=string,JSONPath=`.spec.transport`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Tools",type=integer,JSONPath=`.status.toolCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type McpServer struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata"`

	Spec McpServerSpec `json:"spec"`

	Status McpServerStatus `json:"status,omitempty"`
}

type McpServerSpec struct {
	// Human-readable name shown in the UI.
	DisplayName string `json:"displayName,omitempty"`

	// Optional description of this MCP server's purpose.
	Description string `json:"description,omitempty"`

	// Transport protocol used to communicate with the MCP server.
	// +kubebuilder:validation:Enum=streamableHttp;sse
	// +kubebuilder:default=streamableHttp
	Transport McpTransportType `json:"transport,omitempty"`

	// Base URL of the MCP server endpoint.
	// +kubebuilder:validation:MinLength=1
	URL string `json:"url"`

	// Authentication for the MCP server. Omit or set type to "none" for
	// unauthenticated servers.
	Auth *McpServerAuth `json:"auth,omitempty"`

	// Additional HTTP headers sent with every request to the MCP server.
	// Use valueFrom to reference header values stored in a Secret.
	Headers []McpServerHeader `json:"headers,omitempty"`

	// AllowedTools is an optional allowlist of tool names exposed to agents.
	// An empty list means all tools discovered from the server are available.
	AllowedTools []string `json:"allowedTools,omitempty"`
}

// McpServerAuth configures how the operator authenticates with the MCP server.
type McpServerAuth struct {
	// Authentication scheme.
	// "bearer" adds an Authorization: Bearer <token> header.
	// "apiKey" adds a custom header named by the Header field.
	// "none" sends no credential.
	// +kubebuilder:validation:Enum=none;bearer;apiKey
	Type McpAuthType `json:"type"`

	// Secret in the same namespace whose value is used as the credential token.
	// Required for bearer and apiKey auth types; ignored for none.
	SecretRef *SecretKeyRef `json:"secretRef,omitempty"`

	// Header name for apiKey auth. Defaults to "X-Api-Key". Ignored for bearer
	// (which always uses the Authorization header) and none.
	// +kubebuilder:default="X-Api-Key"
	Header string `json:"header,omitempty"`
}

// McpServerHeader is a single HTTP header added to every MCP server request.
// Exactly one of Value or ValueFrom should be set.
type McpServerHeader struct {
	// HTTP header name.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Literal header value. Use ValueFrom to pull from a Secret instead.
	Value string `json:"value,omitempty"`

	// Secret whose data key provides the header value. Takes precedence over
	// Value when both are set.
	ValueFrom *SecretKeyRef `json:"valueFrom,omitempty"`
}

type McpServerStatus struct {
	// Conditions report the state of the McpServer; "Ready" indicates that the
	// spec is valid and the server was last successfully reached.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Generation of the spec the conditions were last evaluated against.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// AvailableTools lists the tool names discovered from the MCP server at the
	// last successful connection; empty when the server has not yet been reached
	// or returned no tools. When AllowedTools is set, only allowed tools appear.
	// +optional
	AvailableTools []string `json:"availableTools,omitempty"`

	// ToolCount is the number of currently available tools.
	// +optional
	ToolCount int `json:"toolCount,omitempty"`

	// LastConnectedAt records when the server was last probed successfully
	// (RFC3339 timestamp).
	// +optional
	LastConnectedAt string `json:"lastConnectedAt,omitempty"`
}
