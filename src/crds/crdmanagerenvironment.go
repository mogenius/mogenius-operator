package crds

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func CreateEnvironmentCmd(client *dynamic.DynamicClient, job *structs.Job, projectName string, namespace string, newObj CrdEnvironment, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("create", "Create CRDs for ApplicationKit", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Creating CRDs for ApplicationKit")
		err := CreateEnvironment(client, namespace, namespace, newObj)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("CreateEnvironmentCmd ERROR: %s", err))
		}

		err = AddEnvironmentToProject(client, projectName, namespace)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("AddEnvironmentToProject ERROR: %s", err))
		} else {
			cmd.Success(job, "Created CRDs for ApplicationKit")
		}
	}(wg)
}

func UpdateEnvironmentCmd(client *dynamic.DynamicClient, job *structs.Job, namespace string, newObj CrdEnvironment, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("update", "Update CRDs for Environment", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Updating CRDs for Environment")
		err := UpdateEnvironment(client, namespace, namespace, &newObj)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("UpdateEnvironmentCmd ERROR: %s", err))
		} else {
			cmd.Success(job, "Updated CRDs for Environment")
		}
	}(wg)
}

func DeleteEnvironmentCmd(client *dynamic.DynamicClient, job *structs.Job, projectName string, namespace string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", "Delete CRDs for Environment", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting CRDs for Environment")
		err := DeleteEnvironment(client, namespace, namespace)
		if err != nil {
			cmd.Success(job, "Deleted CRDs for Environment")
		}
		err = RemoveEnvironmentFromProject(client, projectName, namespace)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("RemoveEnvironmentFromProject ERROR: %s", err))
		} else {
			cmd.Success(job, "Deleted CRDs for Environment")
		}
	}(wg)
}

func CreateEnvironment(client *dynamic.DynamicClient, namespace string, name string, newObj CrdEnvironment) error {
	environmentsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	raw := newObj.ToUnstructuredEnvironment(namespace, name)
	_, err := client.Resource(environmentsGVR).Namespace(namespace).Create(context.Background(), raw, metav1.CreateOptions{})
	if err != nil {
		crdLogger.Error("Error creating Environment", "error", err)
		return err
	}

	return nil
}

func UpdateEnvironment(client *dynamic.DynamicClient, namespace string, name string, updatedObj *CrdEnvironment) error {
	_, environmentUnstructured, err := GetEnvironment(client, namespace, name)
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
	_, err = client.Resource(environmentsGVR).Namespace(namespace).Update(context.Background(), environmentUnstructured, metav1.UpdateOptions{})
	if err != nil {
		crdLogger.Error("Error updating Environment", "error", err)
		return err
	}

	return nil
}

func DeleteEnvironment(client *dynamic.DynamicClient, namespace string, name string) error {
	environmentsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	err := client.Resource(environmentsGVR).Namespace(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		crdLogger.Error("Error deleting Environment", "error", err)
		return err
	}

	return nil
}

func GetEnvironment(client *dynamic.DynamicClient, namespace string, name string) (environment *CrdEnvironment, EnvironmentRaw *unstructured.Unstructured, err error) {
	result := CrdEnvironment{}

	environmentsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	environmentItem, err := client.Resource(environmentsGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
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

func ListEnvironments(client *dynamic.DynamicClient, namespace string) (Environment []CrdEnvironment, EnvironmentRaw *unstructured.UnstructuredList, err error) {
	result := []CrdEnvironment{}

	environmentsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceEnvironment}
	environments, err := client.Resource(environmentsGVR).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
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

func AddAppKitToEnvironment(client *dynamic.DynamicClient, namespace string, appkitName string) error {
	existingEnvironment, environmentUnstructured, err := GetEnvironment(client, namespace, namespace)
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
	_, err = client.Resource(environmentsGVR).Namespace(namespace).Update(context.Background(), environmentUnstructured, metav1.UpdateOptions{})
	if err != nil {
		crdLogger.Error("Error updating environment", "error", err)
		return err
	}

	return nil
}

func RemoveAppKitFromEnvironment(client *dynamic.DynamicClient, namespace string, appkitName string) error {
	existingEnironment, environmentUnstructured, err := GetEnvironment(client, namespace, namespace)
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
	_, err = client.Resource(environemntGVR).Namespace(namespace).Update(context.Background(), environmentUnstructured, metav1.UpdateOptions{})
	if err != nil {
		crdLogger.Error("Error updating environment", "error", err)
		return err
	}

	return nil
}
