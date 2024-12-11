package crds

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/src/structs"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func CreateOrUpdateApplicationKitCmd(client *dynamic.DynamicClient, job *structs.Job, namespace string, name string, newObj CrdApplicationKit, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("create", "Update CRDs ApplicationKit", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Creating/Updating CRDs for ApplicationKit")
		err := CreateOrUpdateApplicationKit(client, namespace, name, newObj)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("CreateOrUpdateApplicationKitCmd ERROR: %s", err))
		}

		err = AddAppKitToEnvironment(client, namespace, newObj.Controller)
		if err != nil {
			// TODO: wieder aufnehmen
			// cmd.Fail(job, fmt.Sprintf("AddAppKitToEnvironment ERROR: %s", err))
			cmd.Success(job, "Updated CRDs ApplicationKit")
		} else {
			cmd.Success(job, "Updated CRDs ApplicationKit")
		}
	}(wg)
}

func DeleteApplicationKitCmd(client *dynamic.DynamicClient, job *structs.Job, namespace string, name string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", "Delete CRDs for ApplicationKit", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting CRDs ApplicationKit")
		err := DeleteApplicationKit(client, namespace, name)
		if err != nil {
			cmd.Success(job, "Deleted CRDs for ApplicationKit")
			// cmd.Fail(job, fmt.Sprintf("DeleteApplicationKitCmd ERROR: %s", err)) // ignore this until we migrate to the new system
		}
		err = RemoveAppKitFromEnvironment(client, namespace, job.ControllerName)
		if err != nil {
			// TODO: wieder aufnehmen
			// cmd.Fail(job, fmt.Sprintf("RemoveAppKitFromEnvironment ERROR: %s", err))
			cmd.Success(job, "Deleted CRDs ApplicationKit")
		} else {
			cmd.Success(job, "Deleted CRDs ApplicationKit")
		}
	}(wg)
}

func CreateOrUpdateApplicationKit(client *dynamic.DynamicClient, namespace string, name string, newObj CrdApplicationKit) error {
	_, _, err := GetApplicationKit(client, namespace, name)
	if err != nil {
		err = CreateApplicationKit(client, namespace, name, newObj)
		if err != nil {
			crdLogger.Error("Error creating applicationkit", "error", err)
			return err
		}
	} else {
		err = UpdateApplicationKit(client, namespace, name, &newObj)
		if err != nil {
			crdLogger.Error("Error updating applicationkit", "error", err)
			return err
		}
	}
	return nil
}

func CreateApplicationKit(client *dynamic.DynamicClient, namespace string, name string, newObj CrdApplicationKit) error {
	appKitsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceApplicationKit}
	raw := newObj.ToUnstructuredApplicationKit(namespace, name)
	_, err := client.Resource(appKitsGVR).Namespace(namespace).Create(context.Background(), raw, metav1.CreateOptions{})
	if err != nil {
		crdLogger.Error("Error creating applicationkit", "error", err)
		return err
	}
	return err
}

func UpdateApplicationKit(client *dynamic.DynamicClient, namespace string, name string, updatedObj *CrdApplicationKit) error {
	_, appkitUnstructured, err := GetApplicationKit(client, namespace, name)
	if err != nil {
		crdLogger.Error("Error updating applicationkit", "error", err)
		return err
	}

	unstrRaw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(updatedObj)
	if err != nil {
		crdLogger.Error("Error converting applicationkit to unstructured", "error", err)
		return err
	}
	appkitUnstructured.Object["spec"] = unstrRaw

	appKitsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceApplicationKit}
	_, err = client.Resource(appKitsGVR).Namespace(namespace).Update(context.Background(), appkitUnstructured, metav1.UpdateOptions{})
	if err != nil {
		crdLogger.Error("Error updating applicationkit", "error", err)
		return err
	}

	return nil
}

func DeleteApplicationKit(client *dynamic.DynamicClient, namespace string, name string) error {
	appKitsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceApplicationKit}
	err := client.Resource(appKitsGVR).Namespace(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		crdLogger.Error("Error deleting applicationkit", "error", err)
		return err
	}
	return err
}

func GetApplicationKit(client *dynamic.DynamicClient, namespace string, name string) (appkit CrdApplicationKit, appkitRaw *unstructured.Unstructured, err error) {
	result := CrdApplicationKit{}

	appKitsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceApplicationKit}
	appkitItem, err := client.Resource(appKitsGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		crdLogger.Error("Error getting applicationkit", "error", err)
		return result, appkitItem, err
	}

	jsonData, err := json.Marshal(appkitItem.Object["spec"])
	if err != nil {
		crdLogger.Error("Error marshalling applicationkit spec", "error", err)
		return result, appkitItem, err
	}
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		crdLogger.Error("Error unmarshalling applicationkit spec", "error", err)
		return result, appkitItem, err
	}

	return result, appkitItem, err
}

func ListApplicationKits(client *dynamic.DynamicClient, namespace string) (appkit []CrdApplicationKit, appkitRaw *unstructured.UnstructuredList, err error) {
	result := []CrdApplicationKit{}

	appKitsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceApplicationKit}
	appkits, err := client.Resource(appKitsGVR).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		crdLogger.Error("Error getting applicationkit", "error", err)
		return result, appkits, err
	}

	for _, appkit := range appkits.Items {
		entry := CrdApplicationKit{}
		jsonData, err := json.Marshal(appkit.Object["spec"])
		if err != nil {
			crdLogger.Error("Error marshalling applicationkit spec", "error", err)
			return result, appkits, err
		}
		err = json.Unmarshal(jsonData, &entry)
		if err != nil {
			crdLogger.Error("Error unmarshalling applicationkit spec", "error", err)
			return result, appkits, err
		}
		result = append(result, entry)
	}
	return result, appkits, err
}
