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

func CreateOrUpdateApplicationKitCmd(job *structs.Job, namespace string, name string, newObj CrdApplicationKit, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("create", "Update CRDs ApplicationKit", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Creating/Updating CRDs for ApplicationKit")
		err := CreateOrUpdateApplicationKit(namespace, name, newObj)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("CreateOrUpdateApplicationKitCmd ERROR: %s", err.Error()))
		}

		err = AddAppKitToEnvironment(namespace, newObj.Controller)
		if err != nil {
			// TODO: wieder aufnehmen
			// cmd.Fail(job, fmt.Sprintf("AddAppKitToEnvironment ERROR: %s", err.Error()))
			cmd.Success(job, "Updated CRDs ApplicationKit")
		} else {
			cmd.Success(job, "Updated CRDs ApplicationKit")
		}
	}(wg)
}

func DeleteApplicationKitCmd(job *structs.Job, namespace string, name string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", "Delete CRDs for ApplicationKit", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting CRDs ApplicationKit")
		err := DeleteApplicationKit(namespace, name)
		if err != nil {
			cmd.Success(job, "Deleted CRDs for ApplicationKit")
			// cmd.Fail(job, fmt.Sprintf("DeleteApplicationKitCmd ERROR: %s", err.Error())) // ignore this until we migrate to the new system
		}
		err = RemoveAppKitFromEnvironment(namespace, job.ControllerName)
		if err != nil {
			// TODO: wieder aufnehmen
			// cmd.Fail(job, fmt.Sprintf("RemoveAppKitFromEnvironment ERROR: %s", err.Error()))
			cmd.Success(job, "Deleted CRDs ApplicationKit")
		} else {
			cmd.Success(job, "Deleted CRDs ApplicationKit")
		}
	}(wg)
}

func CreateOrUpdateApplicationKit(namespace string, name string, newObj CrdApplicationKit) error {
	_, _, err := GetApplicationKit(namespace, name)
	if err != nil {
		err = CreateApplicationKit(namespace, name, newObj)
		if err != nil {
			CrdLogger.Errorf("Error creating applicationkit: %s", err.Error())
			return err
		}
	} else {
		err = UpdateApplicationKit(namespace, name, &newObj)
		if err != nil {
			CrdLogger.Errorf("Error updating applicationkit: %s", err.Error())
			return err
		}
	}
	return nil
}

func CreateApplicationKit(namespace string, name string, newObj CrdApplicationKit) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		CrdLogger.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return err
	}

	appKitsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceApplicationKit}
	raw := newObj.ToUnstructuredApplicationKit(namespace, name)
	_, err = provider.ClientSet.Resource(appKitsGVR).Namespace(namespace).Create(context.Background(), raw, metav1.CreateOptions{})
	if err != nil {
		CrdLogger.Errorf("Error creating applicationkit: %s", err.Error())
		return err
	}
	return err
}

func UpdateApplicationKit(namespace string, name string, updatedObj *CrdApplicationKit) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		CrdLogger.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return err
	}

	_, appkitUnstructured, err := GetApplicationKit(namespace, name)
	if err != nil {
		CrdLogger.Errorf("Error updating applicationkit: %s", err.Error())
		return err
	}

	unstrRaw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(updatedObj)
	if err != nil {
		CrdLogger.Errorf("Error converting applicationkit to unstructured: %s", err.Error())
		return err
	}
	appkitUnstructured.Object["spec"] = unstrRaw

	appKitsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceApplicationKit}
	_, err = provider.ClientSet.Resource(appKitsGVR).Namespace(namespace).Update(context.Background(), appkitUnstructured, metav1.UpdateOptions{})
	if err != nil {
		CrdLogger.Errorf("Error updating applicationkit: %s", err.Error())
		return err
	}

	return nil
}

func DeleteApplicationKit(namespace string, name string) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		CrdLogger.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return err
	}

	appKitsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceApplicationKit}
	err = provider.ClientSet.Resource(appKitsGVR).Namespace(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		CrdLogger.Errorf("Error deleting applicationkit: %s", err.Error())
		return err
	}
	return err
}

func GetApplicationKit(namespace string, name string) (appkit CrdApplicationKit, appkitRaw *unstructured.Unstructured, err error) {
	result := CrdApplicationKit{}

	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		CrdLogger.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return result, nil, err
	}

	appKitsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceApplicationKit}
	appkitItem, err := provider.ClientSet.Resource(appKitsGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		CrdLogger.Errorf("Error getting applicationkit: %s", err.Error())
		return result, appkitItem, err
	}

	jsonData, err := json.Marshal(appkitItem.Object["spec"])
	if err != nil {
		CrdLogger.Errorf("Error marshalling applicationkit spec: %s", err.Error())
		return result, appkitItem, err
	}
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		CrdLogger.Errorf("Error unmarshalling applicationkit spec: %s", err.Error())
		return result, appkitItem, err
	}

	return result, appkitItem, err
}

func ListApplicationKits(namespace string) (appkit []CrdApplicationKit, appkitRaw *unstructured.UnstructuredList, err error) {
	result := []CrdApplicationKit{}

	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		CrdLogger.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return result, nil, err
	}

	appKitsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceApplicationKit}
	appkits, err := provider.ClientSet.Resource(appKitsGVR).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		CrdLogger.Errorf("Error getting applicationkit: %s", err.Error())
		return result, appkits, err
	}

	for _, appkit := range appkits.Items {
		entry := CrdApplicationKit{}
		jsonData, err := json.Marshal(appkit.Object["spec"])
		if err != nil {
			CrdLogger.Errorf("Error marshalling applicationkit spec: %s", err.Error())
			return result, appkits, err
		}
		err = json.Unmarshal(jsonData, &entry)
		if err != nil {
			CrdLogger.Errorf("Error unmarshalling applicationkit spec: %s", err.Error())
			return result, appkits, err
		}
		result = append(result, entry)
	}
	return result, appkits, err
}
