package crds_test

import (
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/crds"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/utils"
	"testing"
)

func TestApplicationKit(t *testing.T) {
	logManager := logging.NewMockSlogManager(t)
	clientProvider := k8sclient.NewK8sClientProvider(logManager.CreateLogger("client-provider"))
	dynamicClient := clientProvider.DynamicClient()

	name := "test"
	namespace := "default"
	newAppkitName := name + utils.NanoIdSmallLowerCase()

	// CREATE
	err := crds.CreateApplicationKit(dynamicClient, namespace, newAppkitName, crds.CrdApplicationKit{Id: name, DisplayName: "Test Project", CreatedBy: name, Controller: "tesst", AppId: "gtesdf"})
	assert.AssertT(t, err == nil, err)
	t.Log("Applicationkit created ✅")

	// GET
	appkit, _, err := crds.GetApplicationKit(dynamicClient, namespace, newAppkitName)
	assert.AssertT(t, err == nil, err)
	t.Log("Applicationkit retrieved ✅")
	appkit.Id = "Updated " + name
	appkit.DisplayName = "Updated Test Project"
	appkit.CreatedBy = "Updated " + name
	appkit.Controller = "Updated " + name
	appkit.AppId = "Updated " + name

	// UPDATE
	err = crds.UpdateApplicationKit(dynamicClient, namespace, newAppkitName, &appkit)
	assert.AssertT(t, err == nil, err)
	t.Log("Applicationkit updated ✅")

	// DELETE
	err = crds.DeleteApplicationKit(dynamicClient, namespace, newAppkitName)
	assert.AssertT(t, err == nil, err)
	t.Log("ApplicationKit deleted ✅")

	// LIST
	_, _, err = crds.ListProjects(dynamicClient)
	assert.AssertT(t, err == nil, err)
	t.Log("Applicationkits listed ✅")
}
