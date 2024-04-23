package crds

import (
	"fmt"
	"log"
	"testing"

	punqUtils "github.com/mogenius/punq/utils"
)

func TestProject(t *testing.T) {
	name := "test"
	newProjectName := name + punqUtils.NanoIdSmallLowerCase()

	// CREATE
	err := CreateProject(newProjectName, CrdProject{Id: name, DisplayName: "Test Project", CreatedBy: name, ProductId: name, ClusterId: name, GitConnectionId: name, ApplicationKitRefs: []string{name}})
	if err != nil {
		log.Fatalf("Error creating project: %s", err.Error())
	} else {
		fmt.Println("Project created ✅")
	}

	// GET
	project, _, err := GetProject(newProjectName)
	if err != nil {
		log.Fatalf("Error getting project: %s", err.Error())
	} else {
		fmt.Println("Project retrieved ✅")
	}
	project.Id = "Updated " + name
	project.DisplayName = "Updated Test Project"
	project.CreatedBy = "Updated " + name
	project.ProductId = "Updated " + name
	project.ClusterId = "Updated " + name
	project.GitConnectionId = "Updated " + name
	project.ApplicationKitRefs = []string{"Updated " + name}

	// UPDATE
	err = UpdateProject(newProjectName, project)
	if err != nil {
		log.Fatalf("Error updating project: %s", err.Error())
	} else {
		fmt.Println("Project updated ✅")
	}

	// DELETE
	err = DeleteProject(newProjectName)
	if err != nil {
		log.Fatalf("Error deleting project: %s", err.Error())
	} else {
		fmt.Println("Project deleted ✅")
	}

	// LIST
	_, _, err = ListProjects()
	if err != nil {
		log.Fatalf("Error listing projects: %s", err.Error())
	} else {
		fmt.Println("Projects listed ✅")
	}
}
