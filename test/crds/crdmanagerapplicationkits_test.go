package crds_test

import (
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/crds"
	"mogenius-k8s-manager/src/utils"
	"testing"
)

func TestApplicationKit(t *testing.T) {
	name := "test"
	namespace := "default"
	newAppkitName := name + utils.NanoIdSmallLowerCase()

	// CREATE
	err := crds.CreateApplicationKit(namespace, newAppkitName, crds.CrdApplicationKit{Id: name, DisplayName: "Test Project", CreatedBy: name, Controller: "tesst", AppId: "gtesdf"})
	assert.AssertT(t, err == nil, err)
	t.Log("Applicationkit created ✅")

	// GET
	appkit, _, err := crds.GetApplicationKit(namespace, newAppkitName)
	assert.AssertT(t, err == nil, err)
	t.Log("Applicationkit retrieved ✅")
	appkit.Id = "Updated " + name
	appkit.DisplayName = "Updated Test Project"
	appkit.CreatedBy = "Updated " + name
	appkit.Controller = "Updated " + name
	appkit.AppId = "Updated " + name

	// UPDATE
	err = crds.UpdateApplicationKit(namespace, newAppkitName, &appkit)
	assert.AssertT(t, err == nil, err)
	t.Log("Applicationkit updated ✅")

	// DELETE
	err = crds.DeleteApplicationKit(namespace, newAppkitName)
	assert.AssertT(t, err == nil, err)
	t.Log("ApplicationKit deleted ✅")

	// LIST
	_, _, err = crds.ListProjects()
	assert.AssertT(t, err == nil, err)
	t.Log("Applicationkits listed ✅")
}
