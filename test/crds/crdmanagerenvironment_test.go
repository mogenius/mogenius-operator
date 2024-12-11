package crds_test

import (
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/crds"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/utils"
	"testing"
)

func TestEnvironment(t *testing.T) {
	logManager := logging.NewMockSlogManager(t)
	clientProvider := k8sclient.NewK8sClientProvider(logManager.CreateLogger("client-provider"))
	dynamicClient := clientProvider.DynamicClient()

	name := "test"
	namespace := "default"
	newEnvironmentName := name + utils.NanoIdSmallLowerCase()

	// CREATE
	err := crds.CreateEnvironment(dynamicClient, namespace, newEnvironmentName, crds.CrdEnvironment{})
	assert.AssertT(t, err == nil, err)
	t.Log("Environment created ✅")

	// GET
	environment, _, err := crds.GetEnvironment(dynamicClient, namespace, newEnvironmentName)
	assert.AssertT(t, err == nil, err)
	t.Log("Environment retrieved ✅")

	environment.Id = "Updated " + name
	environment.DisplayName = "Updated Test environment"
	environment.CreatedBy = "Updated " + name
	// UPDATE
	err = crds.UpdateEnvironment(dynamicClient, namespace, newEnvironmentName, environment)
	assert.AssertT(t, err == nil, err)
	t.Log("environment updated ✅")

	// DELETE
	err = crds.DeleteEnvironment(dynamicClient, namespace, newEnvironmentName)
	assert.AssertT(t, err == nil, err)
	t.Log("environment deleted ✅")

	// LIST
	_, _, err = crds.ListEnvironments(dynamicClient, namespace)
	assert.AssertT(t, err == nil, err)
	t.Log("environments listed ✅")
}
