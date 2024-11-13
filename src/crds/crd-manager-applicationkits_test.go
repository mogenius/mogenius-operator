package crds_test

import (
	"fmt"
	"mogenius-k8s-manager/src/crds"
	"testing"

	punqUtils "github.com/mogenius/punq/utils"
)

func TestApplicationKit(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	name := "test"
	namespace := "default"
	newAppkitName := name + punqUtils.NanoIdSmallLowerCase()

	// CREATE
	err := crds.CreateApplicationKit(namespace, newAppkitName, crds.CrdApplicationKit{Id: name, DisplayName: "Test Project", CreatedBy: name, Controller: "tesst", AppId: "gtesdf"})
	if err != nil {
		t.Fatalf("Error creating appkit: %s", err.Error())
	} else {
		fmt.Println("Applicationkit created ✅")
	}

	// GET
	appkit, _, err := crds.GetApplicationKit(namespace, newAppkitName)
	if err != nil {
		t.Fatalf("Error getting appkit: %s", err.Error())
	} else {
		fmt.Println("Applicationkit retrieved ✅")
	}
	appkit.Id = "Updated " + name
	appkit.DisplayName = "Updated Test Project"
	appkit.CreatedBy = "Updated " + name
	appkit.Controller = "Updated " + name
	appkit.AppId = "Updated " + name

	// UPDATE
	err = crds.UpdateApplicationKit(namespace, newAppkitName, &appkit)
	if err != nil {
		t.Fatalf("Error updating appkit: %s", err.Error())
	} else {
		fmt.Println("Applicationkit updated ✅")
	}

	// DELETE
	err = crds.DeleteApplicationKit(namespace, newAppkitName)
	if err != nil {
		t.Fatalf("Error deleting appkit: %s", err.Error())
	} else {
		fmt.Println("ApplicationKit deleted ✅")
	}

	// LIST
	_, _, err = crds.ListProjects()
	if err != nil {
		t.Fatalf("Error listing appkits: %s", err.Error())
	} else {
		fmt.Println("Applicationkits listed ✅")
	}
}
