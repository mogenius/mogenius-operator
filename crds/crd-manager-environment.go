package crds

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func CreateEnvironmentCmd(job *structs.Job, projectName string, namespace string, newObj CrdEnvironment, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("create", "Create CRDs for ApplicationKit", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Creating CRDs for ApplicationKit")
		err := CreateEnvironment(namespace, namespace, newObj)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("CreateEnvironmentCmd ERROR: %s", err.Error()))
		}

		err = AddEnvironmentToProject(projectName, namespace)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("AddEnvironmentToProject ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Created CRDs for ApplicationKit")
		}
	}(wg)
}

func UpdateEnvironmentCmd(job *structs.Job, namespace string, newObj CrdEnvironment, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("update", "Update CRDs for Environment", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Updating CRDs for Environment")
		err := UpdateEnvironment(namespace, namespace, &newObj)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("UpdateEnvironmentCmd ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Updated CRDs for Environment")
		}
	}(wg)
}

func DeleteEnvironmentCmd(job *structs.Job, projectName string, namespace string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", "Delete CRDs for Environment", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting CRDs for Environment")
		err := DeleteEnvironment(namespace, namespace)
		if err != nil {
			cmd.Success(job, "Deleted CRDs for Environment")
		}
		err = RemoveEnvironmentFromProject(projectName, namespace)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("RemoveEnvironmentFromProject ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Deleted CRDs for Environment")
		}
	}(wg)
}

func CreateEnvironment(namespace string, name string, newObj CrdEnvironment) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return err
	}

	environmentsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	raw := newObj.ToUnstructuredEnvironment(namespace, name)
	_, err = provider.ClientSet.Resource(environmentsGVR).Namespace(namespace).Create(context.Background(), raw, metav1.CreateOptions{})
	if err != nil {
		log.Errorf("Error creating Environment: %s", err.Error())
		return err
	}

	return nil
}

func UpdateEnvironment(namespace string, name string, updatedObj *CrdEnvironment) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return err
	}

	_, environmentUnstructured, err := GetEnvironment(namespace, name)
	if err != nil {
		log.Errorf("Error updating Environment: %s", err.Error())
		return err
	}

	unstrRaw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(updatedObj)
	if err != nil {
		log.Errorf("Error converting Environment to unstructured: %s", err.Error())
		return err
	}
	environmentUnstructured.Object["spec"] = unstrRaw

	environmentsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	_, err = provider.ClientSet.Resource(environmentsGVR).Namespace(namespace).Update(context.Background(), environmentUnstructured, metav1.UpdateOptions{})
	if err != nil {
		log.Errorf("Error updating Environment: %s", err.Error())
		return err
	}

	return nil
}

func DeleteEnvironment(namespace string, name string) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return err
	}

	environmentsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	err = provider.ClientSet.Resource(environmentsGVR).Namespace(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		log.Errorf("Error deleting Environment: %s", err.Error())
		return err
	}

	return nil
}

func GetEnvironment(namespace string, name string) (environment *CrdEnvironment, EnvironmentRaw *unstructured.Unstructured, err error) {
	result := CrdEnvironment{}

	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return nil, nil, err
	}

	environmentsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	environmentItem, err := provider.ClientSet.Resource(environmentsGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Error getting Environment: %s", err.Error())
		return nil, environmentItem, err
	}

	jsonData, err := json.Marshal(environmentItem.Object["spec"])
	if err != nil {
		log.Errorf("Error marshalling Environment spec: %s", err.Error())
		return nil, environmentItem, err
	}
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		log.Errorf("Error unmarshalling Environment spec: %s", err.Error())
		return nil, environmentItem, err
	}

	return &result, environmentItem, err
}

func ListEnvironments(namespace string) (Environment []CrdEnvironment, EnvironmentRaw *unstructured.UnstructuredList, err error) {
	result := []CrdEnvironment{}

	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return result, nil, err
	}

	environmentsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	environments, err := provider.ClientSet.Resource(environmentsGVR).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Errorf("Error getting Environment: %s", err.Error())
		return result, environments, err
	}

	for _, Environment := range environments.Items {
		entry := CrdEnvironment{}
		jsonData, err := json.Marshal(Environment.Object["spec"])
		if err != nil {
			log.Errorf("Error marshalling Environment spec: %s", err.Error())
			return result, environments, err
		}
		err = json.Unmarshal(jsonData, &entry)
		if err != nil {
			log.Errorf("Error unmarshalling Environment spec: %s", err.Error())
			return result, environments, err
		}
		result = append(result, entry)
	}
	return result, environments, err
}

func AddAppKitToEnvironment(namespace string, appkitName string) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return err
	}

	existingEnvironment, environmentUnstructured, err := GetEnvironment(namespace, namespace)
	if err != nil {
		log.Errorf("Error updating environment: %s", err.Error())
		return err
	}

	// only add if not already present
	if utils.ContainsString(existingEnvironment.ApplicationKitRefs, appkitName) {
		return nil
	}
	existingEnvironment.ApplicationKitRefs = append(existingEnvironment.ApplicationKitRefs, appkitName)

	unstrRaw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(existingEnvironment)
	if err != nil {
		log.Errorf("Error converting environment to unstructured: %s", err.Error())
		return err
	}
	environmentUnstructured.Object["spec"] = unstrRaw

	environmentsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	_, err = provider.ClientSet.Resource(environmentsGVR).Namespace(namespace).Update(context.Background(), environmentUnstructured, metav1.UpdateOptions{})
	if err != nil {
		log.Errorf("Error updating environment: %s", err.Error())
		return err
	}

	return nil
}

func RemoveAppKitFromEnvironment(namespace string, appkitName string) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return err
	}

	existingEnironment, environmentUnstructured, err := GetEnvironment(namespace, namespace)
	if err != nil {
		log.Errorf("Error updating environment: %s", err.Error())
		return err
	}
	for i, id := range existingEnironment.ApplicationKitRefs {
		if id == appkitName {
			existingEnironment.ApplicationKitRefs = append(existingEnironment.ApplicationKitRefs[:i], existingEnironment.ApplicationKitRefs[i+1:]...)
			break
		}
	}

	unstrRaw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(existingEnironment)
	if err != nil {
		log.Errorf("Error converting environment to unstructured: %s", err.Error())
		return err
	}
	environmentUnstructured.Object["spec"] = unstrRaw

	environemntGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	_, err = provider.ClientSet.Resource(environemntGVR).Namespace(namespace).Update(context.Background(), environmentUnstructured, metav1.UpdateOptions{})
	if err != nil {
		log.Errorf("Error updating environment: %s", err.Error())
		return err
	}

	return nil
}
