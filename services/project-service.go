package services

import (
	"fmt"
	"mogenius-k8s-manager/crds"
	"mogenius-k8s-manager/structs"
	"sync"
)

func CreateProject(r ProjectCreateRequest) *structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob(fmt.Sprintf("Create project %s", r.ProjectName), r.Project.Id, "", "")
	job.Start()
	crds.CreateProjectCmd(job, r.ProjectName, r.Project, &wg)
	wg.Wait()
	job.Finish()
	return job
}

func UpdateProject(r ProjectCreateRequest) *structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob(fmt.Sprintf("Update project %s", r.ProjectName), r.Project.Id, "", "")
	job.Start()
	crds.UpdateProjectCmd(job, r.ProjectName, r.Project, &wg)
	wg.Wait()
	job.Finish()
	return job
}

func DeleteProject(r ProjectDeleteRequest) *structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob(fmt.Sprintf("Delete project %s", r.ProjectName), r.ProjectId, "", "")
	job.Start()
	crds.DeleteProjectCmd(job, r.ProjectName, &wg)
	wg.Wait()
	job.Finish()
	return job
}
