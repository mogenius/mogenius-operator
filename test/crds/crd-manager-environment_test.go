package crds_test

import (
	"fmt"
	"mogenius-k8s-manager/src/crds"
	"testing"

	punqUtils "github.com/mogenius/punq/utils"
)

func TestEnvironment(t *testing.T) {
	name := "test"
	namespace := "default"
	newEnvironmentName := name + punqUtils.NanoIdSmallLowerCase()

	// CREATE
	err := crds.CreateEnvironment(namespace, newEnvironmentName, crds.CrdEnvironment{})
	if err != nil {
		t.Fatalf("Error creating Environment: %s", err.Error())
	} else {
		fmt.Println("Environment created ✅")
	}

	// GET
	environment, _, err := crds.GetEnvironment(namespace, newEnvironmentName)
	if err != nil {
		t.Fatalf("Error getting Environment: %s", err.Error())
	} else {
		fmt.Println("Environment retrieved ✅")
	}
	environment.Id = "Updated " + name
	environment.DisplayName = "Updated Test environment"
	environment.CreatedBy = "Updated " + name
	// UPDATE
	err = crds.UpdateEnvironment(namespace, newEnvironmentName, environment)
	if err != nil {
		t.Fatalf("Error updating environment: %s", err.Error())
	} else {
		fmt.Println("environment updated ✅")
	}

	// DELETE
	err = crds.DeleteEnvironment(namespace, newEnvironmentName)
	if err != nil {
		t.Fatalf("Error deleting environment: %s", err.Error())
	} else {
		fmt.Println("environment deleted ✅")
	}

	// LIST
	_, _, err = crds.ListEnvironments(namespace)
	if err != nil {
		t.Fatalf("Error listing environments: %s", err.Error())
	} else {
		fmt.Println("environments listed ✅")
	}
}
