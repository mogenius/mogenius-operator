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
// +kubebuilder:printcolumn:name="ApiVersion",type=string,JSONPath=`.spec.reference.apiVersion`
// +kubebuilder:printcolumn:name="Kind",type=string,JSONPath=`.spec.reference.kind`
// +kubebuilder:printcolumn:name="Display Name",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Customized",type=boolean,JSONPath=`.spec.isCustomized`
// +kubebuilder:printcolumn:name="Name Valid",type=string,JSONPath=`.status.conditions[?(@.type=="NameValid")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type UIConfig struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata"`

	Spec UIConfigSpec `json:"spec"`

	Status UIConfigStatus `json:"status,omitempty"`
}

// UIConfigConditionNameValid reports whether metadata.name matches the
// deterministic "<kind>.<group>" scheme derived from spec.reference. A name
// outside the scheme is never found by the platform's direct lookups, so the
// config would silently not apply.
const UIConfigConditionNameValid = "NameValid"

type UIConfigStatus struct {
	// Conditions reports the results of the reconciler's integrity checks,
	// currently whether metadata.name matches the deterministic naming scheme.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type UIConfigSpec struct {
	// Reference points to the Kubernetes resource type this UI config customises.
	// The operator uses this to look up the matching UIConfig when a user opens a resource form.
	Reference CrdReference `json:"reference"`
	// DisplayName is the human-readable title shown in the UI. The resource's
	// metadata.name is deterministic ("<kind>.<group>", lowercase) so that exactly
	// one UIConfig can exist per resource type and lookups need no listing.
	DisplayName string `json:"displayName,omitempty"`
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
// It is a flat list of sections; the frontend renders them in order.
type UISchema struct {
	// Sections is the ordered list of form sections. The first section is conventionally
	// the metadata section (name, namespace); subsequent sections map to top-level spec fields.
	Sections []UISection `json:"sections"`
}

// UISection is a named group of fields rendered as a collapsible block in the form.
// Sections are purely visual grouping — structural nesting of the resource (objects,
// object arrays, variants) is expressed by the field types below, not by sections.
type UISection struct {
	// Label is the heading text displayed on the section block in the UI.
	Label string `json:"label"`
	// Description is shown below the section heading to explain its purpose to the user.
	Description string `json:"description,omitempty"`
	// Fields lists the inputs that belong to this section.
	Fields []UIField `json:"fields,omitempty"`
}

// UIFieldType selects the input widget the frontend renders for a field.
// +kubebuilder:validation:Enum=string;integer;number;boolean;array;map;object;objectArray;oneOf;yaml
type UIFieldType string

const (
	// UIFieldTypeString renders a free-text input (or a dropdown when Enum is set).
	UIFieldTypeString UIFieldType = "string"
	// UIFieldTypeInteger renders a whole-number input.
	UIFieldTypeInteger UIFieldType = "integer"
	// UIFieldTypeNumber renders a decimal-number input.
	UIFieldTypeNumber UIFieldType = "number"
	// UIFieldTypeBoolean renders a toggle/checkbox.
	UIFieldTypeBoolean UIFieldType = "boolean"
	// UIFieldTypeArray renders a repeatable list of scalar inputs (see ItemType).
	UIFieldTypeArray UIFieldType = "array"
	// UIFieldTypeMap renders editable key/value string pairs (labels, annotations, nodeSelector, ...).
	UIFieldTypeMap UIFieldType = "map"
	// UIFieldTypeObject renders a nested group of inputs for an object property.
	// The sub-fields are listed in Fields; their keys are relative to this field's Key.
	UIFieldTypeObject UIFieldType = "object"
	// UIFieldTypeObjectArray renders a repeatable group: the user can add/remove items,
	// and each item is an object described by Fields (keys relative to the item).
	// This is how lists like containers, ports, env, volumes, or ingress rules are modelled.
	UIFieldTypeObjectArray UIFieldType = "objectArray"
	// UIFieldTypeOneOf renders a variant selector: exactly one of Variants is active,
	// and only the selected variant's fields are shown and written to the manifest.
	// This models union-style specs such as volume sources or ingress backends.
	UIFieldTypeOneOf UIFieldType = "oneOf"
	// UIFieldTypeYaml renders a raw YAML editor for the subtree at Key. Escape hatch for
	// parts of a spec that cannot (or should not) be expressed as structured form fields.
	UIFieldTypeYaml UIFieldType = "yaml"
)

// UIField describes a single input in the form. Scalar types map to one property in the
// resource spec; the structural types (object, objectArray, oneOf, yaml) carry nested
// sub-schemas and may recurse to arbitrary depth.
//
// Key resolution: at the top level (fields directly inside a UISection), Key is the
// dot-notation path from the resource root, e.g. "spec.replicas". Inside a structural
// field (object, objectArray item, oneOf variant), Key is relative to the enclosing
// subtree, e.g. an objectArray with Key "spec.template.spec.containers" has item fields
// with keys like "name", "image" or "resources.limits.cpu".
type UIField struct {
	// Key is the dot-notation path this field reads from / writes to (see type comment
	// for relative vs. absolute resolution). For oneOf fields, Key may be empty when the
	// variant subtrees sit directly on the enclosing object (e.g. volume sources).
	Key string `json:"key"`
	// Label is the human-readable name shown next to the input control.
	Label string `json:"label"`
	// Type selects the input widget the frontend renders for this field.
	Type UIFieldType `json:"type"`
	// Description is shown as a tooltip or hint text below the input to help the user
	// understand what value is expected.
	Description string `json:"description,omitempty"`
	// Required causes the frontend to block form submission if this field is left empty.
	Required bool `json:"required,omitempty"`
	// Default is pre-filled into the input when the user opens a blank create form.
	Default *apiextensionsv1.JSON `json:"default,omitempty"`
	// Enum lists the allowed values for this field. When set, the frontend replaces the
	// free-text input with a dropdown restricted to these options. Only for scalar types.
	Enum []string `json:"enum,omitempty"`
	// Options restricts the field's value(s) to an admin-curated list and switches the
	// rendered control: scalar fields become a select, array and objectArray fields a
	// multi-select whose chosen entries are written verbatim into the manifest. Each
	// entry is either a raw value or an object {label, value}.
	Options []apiextensionsv1.JSON `json:"options,omitempty"`
	// ItemType is the scalar type of the entries of an "array" field (string when omitted).
	// +kubebuilder:validation:Enum=string;integer;number;boolean
	ItemType string `json:"itemType,omitempty"`
	// Fields holds the sub-schema of a structural field: the properties of an "object",
	// or the per-item properties of an "objectArray". Sub-field keys are relative to this
	// field's subtree. Ignored for scalar types.
	//
	// NOTE: this field is schemaless because CRD structural schemas cannot express
	// recursion. The apiserver stores it unvalidated; the operator validates it instead.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	Fields []UIField `json:"fields,omitempty"`
	// Variants lists the alternatives of a "oneOf" field. Exactly one variant is active
	// at a time. Ignored for all other types.
	//
	// NOTE: schemaless for the same recursion reason as Fields.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	Variants []UIVariant `json:"variants,omitempty"`
	// Discriminator is the relative path of a scalar property that identifies the active
	// variant of a "oneOf" field (e.g. "type" in HPA metric entries). When set, the frontend
	// writes the selected variant's DiscriminatorValue to this path. When empty, the active
	// variant is inferred from which variant subtree key is present.
	Discriminator string `json:"discriminator,omitempty"`
}

// UIVariant is one alternative of a "oneOf" field.
type UIVariant struct {
	// Key is the property name of this variant's subtree within the enclosing object,
	// e.g. "configMap", "secret" or "persistentVolumeClaim" for volume sources.
	Key string `json:"key"`
	// Label is the option text shown in the variant selector.
	Label string `json:"label"`
	// Description is shown to explain when to choose this variant.
	Description string `json:"description,omitempty"`
	// DiscriminatorValue is written to the parent field's Discriminator path when this
	// variant is selected (e.g. "Resource" for the "resource" variant of an HPA metric).
	DiscriminatorValue string `json:"discriminatorValue,omitempty"`
	// Fields describes the inputs of this variant; keys are relative to the variant subtree.
	Fields []UIField `json:"fields,omitempty"`
}