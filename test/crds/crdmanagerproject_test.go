package crds_test

import (
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/crds"
	"mogenius-k8s-manager/src/utils"
	"testing"
)

func TestProject(t *testing.T) {
	name := "test"
	newProjectName := name + utils.NanoIdSmallLowerCase()

	// CREATE
	err := crds.CreateProject(newProjectName, crds.CrdProject{Id: name, DisplayName: "Test Project", ProjectName: name, CreatedBy: name, ProductId: name, ClusterId: name,
		EnvironmentRefs: []string{name},
		Limits:          crds.ProjectLimits{LimitMemoryMB: 1024, LimitCpuCores: 1.0, EphemeralStorageMB: 1024, MaxVolumeSizeGb: 10}},
	)
	assert.AssertT(t, err == nil, err)
	t.Log("Project created ✅")

	// GET
	project, _, err := crds.GetProject(newProjectName)
	assert.AssertT(t, err == nil, err)
	t.Log("Project retrieved ✅")

	project.Id = "Updated " + name
	project.DisplayName = "Updated Test Project"
	project.CreatedBy = "Updated " + name
	project.ProductId = "Updated " + name
	project.ClusterId = "Updated " + name
	project.EnvironmentRefs = []string{"Updated " + name}

	// UPDATE
	err = crds.UpdateProject(newProjectName, project.Id, project.ProjectName, project.DisplayName, project.ProductId, project.Limits)
	assert.AssertT(t, err == nil, err)
	t.Log("Project updated ✅")

	// DELETE
	err = crds.DeleteProject(newProjectName)
	assert.AssertT(t, err == nil, err)
	t.Log("Project deleted ✅")

	// LIST
	_, _, err = crds.ListProjects()
	assert.AssertT(t, err == nil, err)
	t.Log("Projects listed ✅")
}
