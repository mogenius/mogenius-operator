package crds

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"sync"

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
			cmd.Fail(job, fmt.Sprintf("CreateEnvironmentCmd ERROR: %s", err))
		}

		err = AddEnvironmentToProject(projectName, namespace)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("AddEnvironmentToProject ERROR: %s", err))
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
			cmd.Fail(job, fmt.Sprintf("UpdateEnvironmentCmd ERROR: %s", err))
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
			cmd.Fail(job, fmt.Sprintf("RemoveEnvironmentFromProject ERROR: %s", err))
		} else {
			cmd.Success(job, "Deleted CRDs for Environment")
		}
	}(wg)
}

func CreateEnvironment(namespace string, name string, newObj CrdEnvironment) error {
	provider, err := kubernetes.NewDynamicKubeProvider()
	if provider == nil || err != nil {
		crdLogger.Error("Error creating provider. Cannot continue because it is vital.", "error", err)
		return err
	}

	environmentsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	raw := newObj.ToUnstructuredEnvironment(namespace, name)
	_, err = provider.ClientSet.Resource(environmentsGVR).Namespace(namespace).Create(context.Background(), raw, metav1.CreateOptions{})
	if err != nil {
		crdLogger.Error("Error creating Environment", "error", err)
		return err
	}

	return nil
}

func UpdateEnvironment(namespace string, name string, updatedObj *CrdEnvironment) error {
	provider, err := kubernetes.NewDynamicKubeProvider()
	if provider == nil || err != nil {
		crdLogger.Error("Error creating provider. Cannot continue because it is vital.", "error", err)
		return err
	}

	_, environmentUnstructured, err := GetEnvironment(namespace, name)
	if err != nil {
		crdLogger.Error("Error updating Environment", "error", err)
		return err
	}

	unstrRaw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(updatedObj)
	if err != nil {
		crdLogger.Error("Error converting Environment to unstructured", "error", err)
		return err
	}
	environmentUnstructured.Object["spec"] = unstrRaw

	environmentsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	_, err = provider.ClientSet.Resource(environmentsGVR).Namespace(namespace).Update(context.Background(), environmentUnstructured, metav1.UpdateOptions{})
	if err != nil {
		crdLogger.Error("Error updating Environment", "error", err)
		return err
	}

	return nil
}

func DeleteEnvironment(namespace string, name string) error {
	provider, err := kubernetes.NewDynamicKubeProvider()
	if provider == nil || err != nil {
		crdLogger.Error("Error creating provider. Cannot continue because it is vital.", "error", err)
		return err
	}

	environmentsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	err = provider.ClientSet.Resource(environmentsGVR).Namespace(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		crdLogger.Error("Error deleting Environment", "error", err)
		return err
	}

	return nil
}

func GetEnvironment(namespace string, name string) (environment *CrdEnvironment, EnvironmentRaw *unstructured.Unstructured, err error) {
	result := CrdEnvironment{}

	provider, err := kubernetes.NewDynamicKubeProvider()
	if provider == nil || err != nil {
		crdLogger.Error("Error creating provider. Cannot continue because it is vital.", "error", err)
		return nil, nil, err
	}

	environmentsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	environmentItem, err := provider.ClientSet.Resource(environmentsGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		crdLogger.Error("Error getting Environment", "error", err)
		return nil, environmentItem, err
	}

	jsonData, err := json.Marshal(environmentItem.Object["spec"])
	if err != nil {
		crdLogger.Error("Error marshalling Environment spec", "error", err)
		return nil, environmentItem, err
	}
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		crdLogger.Error("Error unmarshalling Environment spec", "error", err)
		return nil, environmentItem, err
	}

	return &result, environmentItem, err
}

func ListEnvironments(namespace string) (Environment []CrdEnvironment, EnvironmentRaw *unstructured.UnstructuredList, err error) {
	result := []CrdEnvironment{}

	provider, err := kubernetes.NewDynamicKubeProvider()
	if provider == nil || err != nil {
		crdLogger.Error("Error creating provider. Cannot continue because it is vital.", "error", err)
		return result, nil, err
	}

	environmentsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	environments, err := provider.ClientSet.Resource(environmentsGVR).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		crdLogger.Error("Error getting Environment", "error", err)
		return result, environments, err
	}

	for _, Environment := range environments.Items {
		entry := CrdEnvironment{}
		jsonData, err := json.Marshal(Environment.Object["spec"])
		if err != nil {
			crdLogger.Error("Error marshalling Environment spec", "error", err)
			return result, environments, err
		}
		err = json.Unmarshal(jsonData, &entry)
		if err != nil {
			crdLogger.Error("Error unmarshalling Environment spec", "error", err)
			return result, environments, err
		}
		result = append(result, entry)
	}
	return result, environments, err
}

func AddAppKitToEnvironment(namespace string, appkitName string) error {
	provider, err := kubernetes.NewDynamicKubeProvider()
	if provider == nil || err != nil {
		crdLogger.Error("Error creating provider. Cannot continue because it is vital.", "error", err)
		return err
	}

	existingEnvironment, environmentUnstructured, err := GetEnvironment(namespace, namespace)
	if err != nil {
		crdLogger.Error("Error updating environment", "error", err)
		return err
	}

	// only add if not already present
	if utils.ContainsString(existingEnvironment.ApplicationKitRefs, appkitName) {
		return nil
	}
	existingEnvironment.ApplicationKitRefs = append(existingEnvironment.ApplicationKitRefs, appkitName)

	unstrRaw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(existingEnvironment)
	if err != nil {
		crdLogger.Error("Error converting environment to unstructured", "error", err)
		return err
	}
	environmentUnstructured.Object["spec"] = unstrRaw

	environmentsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	_, err = provider.ClientSet.Resource(environmentsGVR).Namespace(namespace).Update(context.Background(), environmentUnstructured, metav1.UpdateOptions{})
	if err != nil {
		crdLogger.Error("Error updating environment", "error", err)
		return err
	}

	return nil
}

func RemoveAppKitFromEnvironment(namespace string, appkitName string) error {
	provider, err := kubernetes.NewDynamicKubeProvider()
	if provider == nil || err != nil {
		crdLogger.Error("Error creating provider. Cannot continue because it is vital.", "error", err)
		return err
	}

	existingEnironment, environmentUnstructured, err := GetEnvironment(namespace, namespace)
	if err != nil {
		crdLogger.Error("Error updating environment", "error", err)
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
		crdLogger.Error("Error converting environment to unstructured", "error", err)
		return err
	}
	environmentUnstructured.Object["spec"] = unstrRaw

	environemntGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	_, err = provider.ClientSet.Resource(environemntGVR).Namespace(namespace).Update(context.Background(), environmentUnstructured, metav1.UpdateOptions{})
	if err != nil {
		crdLogger.Error("Error updating environment", "error", err)
		return err
	}

	return nil
}
