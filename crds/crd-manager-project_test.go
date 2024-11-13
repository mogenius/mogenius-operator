package crds_test

import (
	"fmt"
	"mogenius-k8s-manager/crds"
	"testing"

	punqUtils "github.com/mogenius/punq/utils"
)

func TestProject(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	name := "test"
	newProjectName := name + punqUtils.NanoIdSmallLowerCase()

	// CREATE
	err := crds.CreateProject(newProjectName, crds.CrdProject{Id: name, DisplayName: "Test Project", ProjectName: name, CreatedBy: name, ProductId: name, ClusterId: name,
		EnvironmentRefs: []string{name},
		Limits:          crds.ProjectLimits{LimitMemoryMB: 1024, LimitCpuCores: 1.0, EphemeralStorageMB: 1024, MaxVolumeSizeGb: 10}},
	)
	if err != nil {
		t.Fatalf("Error creating project: %s", err.Error())
	} else {
		fmt.Println("Project created ✅")
	}

	// GET
	project, _, err := crds.GetProject(newProjectName)
	if err != nil {
		t.Fatalf("Error getting project: %s", err.Error())
	} else {
		fmt.Println("Project retrieved ✅")
	}
	project.Id = "Updated " + name
	project.DisplayName = "Updated Test Project"
	project.CreatedBy = "Updated " + name
	project.ProductId = "Updated " + name
	project.ClusterId = "Updated " + name
	project.EnvironmentRefs = []string{"Updated " + name}

	// UPDATE
	err = crds.UpdateProject(newProjectName, project.Id, project.ProjectName, project.DisplayName, project.ProductId, project.Limits)
	if err != nil {
		t.Fatalf("Error updating project: %s", err.Error())
	} else {
		fmt.Println("Project updated ✅")
	}

	// DELETE
	err = crds.DeleteProject(newProjectName)
	if err != nil {
		t.Fatalf("Error deleting project: %s", err.Error())
	} else {
		fmt.Println("Project deleted ✅")
	}

	// LIST
	_, _, err = crds.ListProjects()
	if err != nil {
		t.Fatalf("Error listing projects: %s", err.Error())
	} else {
		fmt.Println("Projects listed ✅")
	}
}
