package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ╭─────────────────────╮
// │ CRD: UIConfig       │
// ╰─────────────────────╯

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UIConfigList struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ListMeta `json:"metadata,omitempty"`

	Items []UIConfig `json:"items"`
}

// UIConfig defines a specialised UI for a specific custom resource
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
type UIConfig struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata"`

	Spec UIConfigSpec `json:"spec"`
}

type UIConfigSpec struct {
	// Reference points to the Kubernetes resource type this UI config customises.
	// The operator uses this to look up the matching UIConfig when a user opens a resource form.
	Reference CrdReference `json:"reference"`
	// IsCustomized is false when the schema was auto-generated from the resource's OpenAPI spec
	// and true once a user has saved their own edits. The UI uses this to show a "customised" badge
	// and to decide whether to offer a "reset to default" action.
	IsCustomized bool `json:"isCustomized"`
	// Schema describes the complete form layout the frontend renders when creating or editing
	// a resource of the type identified by Reference.
	Schema UISchema `json:"schema"`
}

// CrdReference identifies a Kubernetes resource type by its API group/version and kind.
type CrdReference struct {
	// ApiVersion is the group and version of the target resource, e.g. "apps/v1" for Deployments
	// or "v1" for core resources like Services. Used together with Kind to uniquely identify the type.
	ApiVersion string `json:"apiVersion"`
	// Kind is the resource kind as it appears in the Kubernetes API, e.g. "Deployment" or "Service".
	Kind string `json:"kind"`
}

// UISchema is the complete form layout for one resource type.
// It is a flat list of sections; the frontend renders them in Order sequence.
type UISchema struct {
	// Sections is the ordered list of form sections. The first section is conventionally
	// the metadata section (name, namespace); subsequent sections map to top-level spec fields.
	Sections []UISection `json:"sections"`
}

// UISection is a named group of related fields rendered as a collapsible block in the form.
// Nested objects in the resource spec are expressed as child sections rather than
// as fields that own other fields, keeping fields as pure leaf inputs.
type UISection struct {
	// Label is the heading text displayed on the section block in the UI.
	Label string `json:"label"`
	// Description is shown below the section heading to explain its purpose to the user.
	Description string `json:"description,omitempty"`
	// Fields lists the leaf inputs that belong directly to this section.
	// Each field maps to a scalar, enum, array-of-scalars, or map property in the resource spec.
	Fields []UIField `json:"fields,omitempty"`
	// Sections holds child sections for nested object properties within this section.
	// For example, a top-level "Spec" section may contain a "Resources" sub-section
	// for spec.resources.limits / spec.resources.requests.
	Sections []UISubSection `json:"sections,omitempty"`
}

type UISubSection struct {
	// Label is the heading text displayed on the section block in the UI.
	Label string `json:"label"`
	// Description is shown below the section heading to explain its purpose to the user.
	Description string `json:"description,omitempty"`
	// Fields lists the leaf inputs that belong directly to this section.
	// Each field maps to a scalar, enum, array-of-scalars, or map property in the resource spec.
	Fields []UIField `json:"fields,omitempty"`
}

// UIField describes a single leaf input in the form — a control that maps to one scalar,
// enum, array-of-scalars, or map property in the resource spec.
// Object-type properties are represented as child UISection values, not as UIFields.
type UIField struct {
	// Key is the dot-notation path to this field in the resource YAML, e.g. "spec.replicas"
	// or "spec.template.spec.containers[].image". The frontend uses this to read and write
	// the correct location when building the resource manifest.
	Key string `json:"key"`
	// Label is the human-readable name shown next to the input control.
	Label string `json:"label"`
	// Type selects the input widget the frontend renders for this field.
	// +kubebuilder:validation:Enum=string;integer;number;boolean;array;map
	Type string `json:"type"`
	// Description is shown as a tooltip or hint text below the input to help the user
	// understand what value is expected.
	Description string `json:"description,omitempty"`
	// Required causes the frontend to block form submission if this field is left empty.
	Required bool `json:"required,omitempty"`
	// Default is pre-filled into the input when the user opens a blank create form.
	Default *apiextensionsv1.JSON `json:"default,omitempty"`
	// Enum lists the allowed values for this field. When set, the frontend replaces the
	// free-text input with a dropdown restricted to these options.
	Enum []string `json:"enum,omitempty"`
}
