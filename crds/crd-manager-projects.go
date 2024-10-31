package crds

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func CreateProjectCmd(job *structs.Job, name string, newObj CrdProject, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("create", "Create CRDs for Project", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Creating CRDs for Project")
		err := CreateProject(name, newObj)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("CreateProjectCmd ERROR: %s", err))
		} else {
			cmd.Success(job, "Created CRDs for Project")
		}
	}(wg)
}

func UpdateProjectCmd(job *structs.Job, id string, projectName string, displayName string, productId string, limits ProjectLimits, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("update", "Update CRDs for Project", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Updating CRDs for Project")
		err := UpdateProject(projectName, id, projectName, displayName, productId, limits)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("UpdateProjectCmd ERROR: %s", err))
		} else {
			cmd.Success(job, "Updated CRDs for Project")
		}
	}(wg)
}

func DeleteProjectCmd(job *structs.Job, name string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", "Delete CRDs for Project", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting CRDs for Project")
		err := DeleteProject(name)
		if err != nil {
			// cmd.Fail(job, fmt.Sprintf("DeleteProjectKitCmd ERROR: %s", err)) // ignore this until we migrate to the new system
			cmd.Success(job, "Deleted CRDs for Project")
		} else {
			cmd.Success(job, "Deleted CRDs for Project")
		}
	}(wg)
}

func CreateProject(name string, newObj CrdProject) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		crdLogger.Error("Error creating provider. Cannot continue because it is vital.", "error", err)
		return err
	}

	projectsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceProject}
	raw := newObj.ToUnstructuredProject(name)
	_, err = provider.ClientSet.Resource(projectsGVR).Create(context.Background(), raw, metav1.CreateOptions{})
	if err != nil {
		crdLogger.Error("Error creating project", "error", err)
		return err
	}

	return nil
}

func UpdateProject(name string, id string, projectName string, displayName string, productId string, limits ProjectLimits) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		crdLogger.Error("Error creating provider. Cannot continue because it is vital.", "error", err)
		return err
	}

	existingProject, projectUnstructured, err := GetProject(name)
	if err != nil {
		crdLogger.Error("Error updating project", "error", err)
		return err
	}
	existingProject.Id = id
	existingProject.DisplayName = displayName
	existingProject.ProjectName = projectName
	existingProject.ProductId = productId
	existingProject.Limits = limits

	unstrRaw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(existingProject)
	if err != nil {
		crdLogger.Error("Error converting project to unstructured", "error", err)
		return err
	}
	projectUnstructured.Object["spec"] = unstrRaw

	projectsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceProject}
	_, err = provider.ClientSet.Resource(projectsGVR).Update(context.Background(), projectUnstructured, metav1.UpdateOptions{})
	if err != nil {
		crdLogger.Error("Error updating project", "error", err)
		return err
	}

	return nil
}

func DeleteProject(name string) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		crdLogger.Error("Error creating provider. Cannot continue because it is vital.", "error", err)
		return err
	}

	projectsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceProject}
	err = provider.ClientSet.Resource(projectsGVR).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		crdLogger.Error("Error deleting project", "error", err)
		return err
	}

	return nil
}

func GetProject(name string) (project *CrdProject, projectRaw *unstructured.Unstructured, err error) {
	result := CrdProject{}

	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		crdLogger.Error("Error creating provider. Cannot continue because it is vital.", "error", err)
		return nil, nil, err
	}

	projectsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceProject}
	projectItem, err := provider.ClientSet.Resource(projectsGVR).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		crdLogger.Error("Error getting project", "error", err)
		return nil, projectItem, err
	}

	jsonData, err := json.Marshal(projectItem.Object["spec"])
	if err != nil {
		crdLogger.Error("Error marshalling project spec", "error", err)
		return nil, projectItem, err
	}
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		crdLogger.Error("Error unmarshalling project spec", "error", err)
		return nil, projectItem, err
	}

	return &result, projectItem, err
}

func ListProjects() (project []CrdProject, projectRaw *unstructured.UnstructuredList, err error) {
	result := []CrdProject{}

	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		crdLogger.Error("Error creating provider. Cannot continue because it is vital.", "error", err)
		return result, nil, err
	}

	projectsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceProject}
	projects, err := provider.ClientSet.Resource(projectsGVR).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		crdLogger.Error("Error getting project", "error", err)
		return result, projects, err
	}

	for _, project := range projects.Items {
		entry := CrdProject{}
		jsonData, err := json.Marshal(project.Object["spec"])
		if err != nil {
			crdLogger.Error("Error marshalling project spec", "error", err)
			return result, projects, err
		}
		err = json.Unmarshal(jsonData, &entry)
		if err != nil {
			crdLogger.Error("Error unmarshalling project spec", "error", err)
			return result, projects, err
		}
		result = append(result, entry)
	}
	return result, projects, err
}

func CountProject() (count int, err error) {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		crdLogger.Error("Error creating provider. Cannot continue because it is vital.", "error", err)
		return 0, err
	}

	projectsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceProject}
	projects, err := provider.ClientSet.Resource(projectsGVR).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		crdLogger.Error("Error getting project", "error", err)
		return 0, err
	}

	return len(projects.Items), err
}

func AddEnvironmentToProject(name string, environmentName string) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		crdLogger.Error("Error creating provider. Cannot continue because it is vital.", "error", err)
		return err
	}

	existingProject, projectUnstructured, err := GetProject(name)
	if err != nil {
		crdLogger.Error("Error updating project", "error", err)
		return err
	}
	existingProject.EnvironmentRefs = append(existingProject.EnvironmentRefs, environmentName)

	unstrRaw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(existingProject)
	if err != nil {
		crdLogger.Error("Error converting project to unstructured", "error", err)
		return err
	}
	projectUnstructured.Object["spec"] = unstrRaw

	projectsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceProject}
	_, err = provider.ClientSet.Resource(projectsGVR).Update(context.Background(), projectUnstructured, metav1.UpdateOptions{})
	if err != nil {
		crdLogger.Error("Error updating project", "error", err)
		return err
	}

	return nil
}

func RemoveEnvironmentFromProject(name string, environmentName string) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		crdLogger.Error("Error creating provider. Cannot continue because it is vital.", "error", err)
		return err
	}

	existingProject, projectUnstructured, err := GetProject(name)
	if err != nil {
		crdLogger.Error("Error updating project", "error", err)
		return err
	}
	for i, id := range existingProject.EnvironmentRefs {
		if id == environmentName {
			existingProject.EnvironmentRefs = append(existingProject.EnvironmentRefs[:i], existingProject.EnvironmentRefs[i+1:]...)
			break
		}
	}

	unstrRaw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(existingProject)
	if err != nil {
		crdLogger.Error("Error converting project to unstructured", "error", err)
		return err
	}
	projectUnstructured.Object["spec"] = unstrRaw

	projectsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceProject}
	_, err = provider.ClientSet.Resource(projectsGVR).Update(context.Background(), projectUnstructured, metav1.UpdateOptions{})
	if err != nil {
		crdLogger.Error("Error updating project", "error", err)
		return err
	}

	return nil
}
