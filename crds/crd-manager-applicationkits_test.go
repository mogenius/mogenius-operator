package crds

import (
	"fmt"
	"testing"

	punqUtils "github.com/mogenius/punq/utils"
)

func TestApplicationKit(t *testing.T) {
	name := "test"
	namespace := "default"
	newAppkitName := name + punqUtils.NanoIdSmallLowerCase()

	// CREATE
	err := CreateApplicationKit(namespace, newAppkitName, CrdApplicationKit{Id: name, DisplayName: "Test Project", CreatedBy: name, Controller: "tesst", AppId: "gtesdf"})
	if err != nil {
		CrdLogger.Fatalf("Error creating appkit: %s", err.Error())
	} else {
		fmt.Println("Applicationkit created ✅")
	}

	// GET
	appkit, _, err := GetApplicationKit(namespace, newAppkitName)
	if err != nil {
		CrdLogger.Fatalf("Error getting appkit: %s", err.Error())
	} else {
		fmt.Println("Applicationkit retrieved ✅")
	}
	appkit.Id = "Updated " + name
	appkit.DisplayName = "Updated Test Project"
	appkit.CreatedBy = "Updated " + name
	appkit.Controller = "Updated " + name
	appkit.AppId = "Updated " + name

	// UPDATE
	err = UpdateApplicationKit(namespace, newAppkitName, &appkit)
	if err != nil {
		CrdLogger.Fatalf("Error updating appkit: %s", err.Error())
	} else {
		fmt.Println("Applicationkit updated ✅")
	}

	// DELETE
	err = DeleteApplicationKit(namespace, newAppkitName)
	if err != nil {
		CrdLogger.Fatalf("Error deleting appkit: %s", err.Error())
	} else {
		fmt.Println("ApplicationKit deleted ✅")
	}

	// LIST
	_, _, err = ListProjects()
	if err != nil {
		CrdLogger.Fatalf("Error listing appkits: %s", err.Error())
	} else {
		fmt.Println("Applicationkits listed ✅")
	}
}
