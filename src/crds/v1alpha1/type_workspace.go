package v1alpha1

import (
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/src/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ╭────────────────╮
// │ CRD: Workspace │
// ╰────────────────╯

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workspace `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Workspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkspaceSpec   `json:"spec,omitempty"`
	Status WorkspaceStatus `json:"status,omitempty"`
}

type WorkspaceSpec struct {
	Name      string                        `json:"name,omitempty"`
	Resources []WorkspaceResourceIdentifier `json:"resources,omitempty"`
}

type WorkspaceResourceIdentifier struct {
	Id   string        `json:"id,omitempty"`
	Type WorkspaceType `json:"type,omitempty"`
}

type WorkspaceStatus struct{}

type WorkspaceType string

const (
	WorkspaceTypeNamespace WorkspaceType = "namespace"
	WorkspaceTypeHelm      WorkspaceType = "helm"
)

var WorkspaceTypeToString = map[WorkspaceType]string{
	WorkspaceTypeNamespace: "namespace",
	WorkspaceTypeHelm:      "helm",
}

var WorkspaceTypeFromString = map[string]WorkspaceType{
	"namespace": WorkspaceTypeNamespace,
	"helm":      WorkspaceTypeHelm,
}

func (self WorkspaceType) MarshalJSON() ([]byte, error) {
	val, ok := WorkspaceTypeToString[self]
	assert.Assert(ok, "unhandled enum variant", self)
	return []byte(`"` + val + `"`), nil
}

func (self *WorkspaceType) UnmarshalJSON(data []byte) error {
	var dataString *string
	err := json.Unmarshal(data, &dataString)
	if err != nil {
		return err
	}
	userSource, ok := WorkspaceTypeFromString[*dataString]
	if !ok {
		return fmt.Errorf("unknown workspace source: %s", *dataString)
	}
	*self = userSource
	return nil
}
