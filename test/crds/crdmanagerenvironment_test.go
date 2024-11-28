package crds_test

import (
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/crds"
	"mogenius-k8s-manager/src/utils"
	"testing"
)

func TestEnvironment(t *testing.T) {
	name := "test"
	namespace := "default"
	newEnvironmentName := name + utils.NanoIdSmallLowerCase()

	// CREATE
	err := crds.CreateEnvironment(namespace, newEnvironmentName, crds.CrdEnvironment{})
	assert.AssertT(t, err == nil, err)
	t.Log("Environment created ✅")

	// GET
	environment, _, err := crds.GetEnvironment(namespace, newEnvironmentName)
	assert.AssertT(t, err == nil, err)
	t.Log("Environment retrieved ✅")

	environment.Id = "Updated " + name
	environment.DisplayName = "Updated Test environment"
	environment.CreatedBy = "Updated " + name
	// UPDATE
	err = crds.UpdateEnvironment(namespace, newEnvironmentName, environment)
	assert.AssertT(t, err == nil, err)
	t.Log("environment updated ✅")

	// DELETE
	err = crds.DeleteEnvironment(namespace, newEnvironmentName)
	assert.AssertT(t, err == nil, err)
	t.Log("environment deleted ✅")

	// LIST
	_, _, err = crds.ListEnvironments(namespace)
	assert.AssertT(t, err == nil, err)
	t.Log("environments listed ✅")
}
