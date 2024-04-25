package crds

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"sync"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func CreateProjectCmd(job *structs.Job, name string, newObj CrdProject, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand(fmt.Sprintf("Create CRDs for '%s'.", name), job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, fmt.Sprintf("Creating CRDs for '%s'.", name))
		err := CreateProject(name, newObj)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("CreateProjectCmd ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, fmt.Sprintf("Created CRDs for '%s'.", name))
		}
	}(wg)
}

func UpdateProjectCmd(job *structs.Job, name string, newObj CrdProject, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand(fmt.Sprintf("Update CRDs for '%s'.", name), job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, fmt.Sprintf("Updating CRDs for '%s'.", name))
		err := UpdateProject(name, &newObj)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("UpdateProjectCmd ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, fmt.Sprintf("Updated CRDs for '%s'.", name))
		}
	}(wg)
}

func DeleteProjectCmd(job *structs.Job, name string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand(fmt.Sprintf("Delete CRDs for '%s'.", name), job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, fmt.Sprintf("Deleting CRDs for '%s'.", name))
		err := DeleteProject(name)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("DeleteProjectKitCmd ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, fmt.Sprintf("Deleted CRDs for '%s'.", name))
		}
	}(wg)
}

func CreateProject(name string, newObj CrdProject) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return err
	}

	projectsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceProject}
	raw := newObj.ToUnstructuredProject(name)
	_, err = provider.ClientSet.Resource(projectsGVR).Create(context.Background(), raw, metav1.CreateOptions{})
	if err != nil {
		log.Errorf("Error creating project: %s", err.Error())
		return err
	}

	return nil
}

func UpdateProject(name string, updatedObj *CrdProject) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return err
	}

	_, projectUnstructured, err := GetProject(name)
	if err != nil {
		log.Errorf("Error updating project: %s", err.Error())
		return err
	}

	unstrRaw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(updatedObj)
	if err != nil {
		log.Errorf("Error converting project to unstructured: %s", err.Error())
		return err
	}
	projectUnstructured.Object["spec"] = unstrRaw

	projectsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceProject}
	_, err = provider.ClientSet.Resource(projectsGVR).Update(context.Background(), projectUnstructured, metav1.UpdateOptions{})
	if err != nil {
		log.Errorf("Error updating project: %s", err.Error())
		return err
	}

	return nil
}

func DeleteProject(name string) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return err
	}

	projectsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceProject}
	err = provider.ClientSet.Resource(projectsGVR).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		log.Errorf("Error deleting project: %s", err.Error())
		return err
	}

	return nil
}

func GetProject(name string) (project *CrdProject, projectRaw *unstructured.Unstructured, err error) {
	result := CrdProject{}

	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return nil, nil, err
	}

	projectsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceProject}
	projectItem, err := provider.ClientSet.Resource(projectsGVR).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Error getting project: %s", err.Error())
		return nil, projectItem, err
	}

	jsonData, err := json.Marshal(projectItem.Object["spec"])
	if err != nil {
		log.Errorf("Error marshalling project spec: %s", err.Error())
		return nil, projectItem, err
	}
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		log.Errorf("Error unmarshalling project spec: %s", err.Error())
		return nil, projectItem, err
	}

	return &result, projectItem, err
}

func ListProjects() (project []CrdProject, projectRaw *unstructured.UnstructuredList, err error) {
	result := []CrdProject{}

	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return result, nil, err
	}

	projectsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceProject}
	projects, err := provider.ClientSet.Resource(projectsGVR).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Errorf("Error getting project: %s", err.Error())
		return result, projects, err
	}

	for _, project := range projects.Items {
		entry := CrdProject{}
		jsonData, err := json.Marshal(project.Object["spec"])
		if err != nil {
			log.Errorf("Error marshalling project spec: %s", err.Error())
			return result, projects, err
		}
		err = json.Unmarshal(jsonData, &entry)
		if err != nil {
			log.Errorf("Error unmarshalling project spec: %s", err.Error())
			return result, projects, err
		}
		result = append(result, entry)
	}
	return result, projects, err
}

func AddAppIdToProject(name string, appId string) error {
	project, _, err := GetProject(name)
	if err != nil {
		log.Errorf("Error getting project: %s", err.Error())
		return err
	}
	if project == nil {
		log.Errorf("Project not found: %s", name)
		return err
	}

	project.ApplicationKitRefs = append(project.ApplicationKitRefs, appId)
	err = UpdateProject(name, project)
	if err != nil {
		log.Errorf("Error updating project: %s", err.Error())
		return err
	}

	return nil
}

func RemoveAppIdFromProject(name string, appId string) error {
	project, _, err := GetProject(name)
	if err != nil {
		log.Errorf("Error getting project: %s", err.Error())
		return err
	}
	if project == nil {
		log.Errorf("Project not found: %s", name)
		return err
	}

	for i, id := range project.ApplicationKitRefs {
		if id == appId {
			project.ApplicationKitRefs = append(project.ApplicationKitRefs[:i], project.ApplicationKitRefs[i+1:]...)
			break
		}
	}
	err = UpdateProject(name, project)
	if err != nil {
		log.Errorf("Error updating project: %s", err.Error())
		return err
	}

	return nil
}
