package client

import (
	"reflect"
	"strings"
	"testing"

	mov1alpha1 "mogenius-operator/src/crds/v1alpha1"

	"github.com/stretchr/testify/assert"
)

// UpdateWorkspace builds its merge patch by hand (the spec fields carry
// omitempty, so cleared values like an empty dashboardRef would otherwise be
// dropped and the update would silently keep the stored value). The map there
// must list every WorkspaceSpec field explicitly — a field added to the struct
// but forgotten in the patch would silently never be written on updates.
//
// This test pins the set of json fields the hand-built patch covers. When it
// fails, extend the patch map in UpdateWorkspace and then add the new field
// here.
func TestWorkspaceSpecFieldsCoveredByUpdateWorkspacePatch(t *testing.T) {
	coveredByUpdatePatch := map[string]bool{
		"name":         true,
		"resources":    true,
		"dashboardRef": true,
	}

	specType := reflect.TypeOf(mov1alpha1.WorkspaceSpec{})
	specFields := map[string]bool{}
	for i := 0; i < specType.NumField(); i++ {
		field := specType.Field(i)
		jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonName == "" || jsonName == "-" {
			continue
		}
		specFields[jsonName] = true
		assert.True(t, coveredByUpdatePatch[jsonName],
			"WorkspaceSpec field %s (json %q) is not part of the hand-built merge patch in UpdateWorkspace — add it there, then list it in this test", field.Name, jsonName)
	}

	for jsonName := range coveredByUpdatePatch {
		assert.True(t, specFields[jsonName],
			"the merge patch in UpdateWorkspace covers %q, but WorkspaceSpec has no such json field — remove it from the patch and from this test", jsonName)
	}
}
