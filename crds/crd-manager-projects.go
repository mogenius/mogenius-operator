package crds

import (
	"context"
	"encoding/json"
	"mogenius-k8s-manager/kubernetes"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

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

func GetProject(name string) (project CrdProject, projectRaw *unstructured.Unstructured, err error) {
	result := CrdProject{}

	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return result, nil, err
	}

	projectsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceProject}
	projectItem, err := provider.ClientSet.Resource(projectsGVR).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Error getting project: %s", err.Error())
		return result, projectItem, err
	}

	jsonData, err := json.Marshal(projectItem.Object["spec"])
	if err != nil {
		log.Errorf("Error marshalling project spec: %s", err.Error())
		return result, projectItem, err
	}
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		log.Errorf("Error unmarshalling project spec: %s", err.Error())
		return result, projectItem, err
	}

	return result, projectItem, err
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
