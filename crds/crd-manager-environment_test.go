package crds

import (
	"fmt"
	"log"
	"testing"

	punqUtils "github.com/mogenius/punq/utils"
)

func TestEnvironment(t *testing.T) {
	name := "test"
	namespace := "default"
	newEnvironmentName := name + punqUtils.NanoIdSmallLowerCase()

	// CREATE
	err := CreateEnvironment(namespace, newEnvironmentName, CrdEnvironment{})
	if err != nil {
		log.Fatalf("Error creating Environment: %s", err.Error())
	} else {
		fmt.Println("Environment created ✅")
	}

	// GET
	environment, _, err := GetEnvironment(namespace, newEnvironmentName)
	if err != nil {
		log.Fatalf("Error getting Environment: %s", err.Error())
	} else {
		fmt.Println("Environment retrieved ✅")
	}
	environment.Id = "Updated " + name
	environment.DisplayName = "Updated Test environment"
	environment.CreatedBy = "Updated " + name
	// UPDATE
	err = UpdateEnvironment(namespace, newEnvironmentName, environment)
	if err != nil {
		log.Fatalf("Error updating environment: %s", err.Error())
	} else {
		fmt.Println("environment updated ✅")
	}

	// DELETE
	err = DeleteEnvironment(namespace, newEnvironmentName)
	if err != nil {
		log.Fatalf("Error deleting environment: %s", err.Error())
	} else {
		fmt.Println("environment deleted ✅")
	}

	// LIST
	_, _, err = ListEnvironments(namespace)
	if err != nil {
		log.Fatalf("Error listing environments: %s", err.Error())
	} else {
		fmt.Println("environments listed ✅")
	}
}
