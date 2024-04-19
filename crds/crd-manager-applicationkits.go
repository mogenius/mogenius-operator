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

func CreateOrUpdateApplicationKit(namespace string, name string, newObj CrdApplicationKit) error {
	_, _, err := GetApplicationKit(namespace, name)
	if err != nil {
		err = CreateApplicationKit(namespace, name, newObj)
		if err != nil {
			log.Errorf("Error creating applicationkit: %s", err.Error())
			return err
		}
	} else {
		err = UpdateApplicationKit(namespace, name, &newObj)
		if err != nil {
			log.Errorf("Error updating applicationkit: %s", err.Error())
			return err
		}
	}
	return nil
}

func CreateApplicationKit(namespace string, name string, newObj CrdApplicationKit) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return err
	}

	appKitsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceApplicationKit}
	raw := newObj.ToUnstructuredApplicationKit(namespace, name)
	_, err = provider.ClientSet.Resource(appKitsGVR).Namespace(namespace).Create(context.Background(), raw, metav1.CreateOptions{})
	if err != nil {
		log.Errorf("Error creating applicationkit: %s", err.Error())
		return err
	}

	err = AddAppIdToProject(namespace, newObj.AppId)

	return err
}

func UpdateApplicationKit(namespace string, name string, updatedObj *CrdApplicationKit) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return err
	}

	_, appkitUnstructured, err := GetApplicationKit(namespace, name)
	if err != nil {
		log.Errorf("Error updating applicationkit: %s", err.Error())
		return err
	}

	unstrRaw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(updatedObj)
	if err != nil {
		log.Errorf("Error converting applicationkit to unstructured: %s", err.Error())
		return err
	}
	appkitUnstructured.Object["spec"] = unstrRaw

	appKitsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceApplicationKit}
	_, err = provider.ClientSet.Resource(appKitsGVR).Namespace(namespace).Update(context.Background(), appkitUnstructured, metav1.UpdateOptions{})
	if err != nil {
		log.Errorf("Error updating applicationkit: %s", err.Error())
		return err
	}

	return nil
}

func DeleteApplicationKit(namespace string, name string) error {
	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return err
	}

	appKitsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceApplicationKit}
	err = provider.ClientSet.Resource(appKitsGVR).Namespace(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		log.Errorf("Error deleting applicationkit: %s", err.Error())
		return err
	}

	err = RemoveAppIdFromProject(namespace, name)

	return err
}

func GetApplicationKit(namespace string, name string) (appkit CrdApplicationKit, appkitRaw *unstructured.Unstructured, err error) {
	result := CrdApplicationKit{}

	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return result, nil, err
	}

	appKitsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceApplicationKit}
	appkitItem, err := provider.ClientSet.Resource(appKitsGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Error getting applicationkit: %s", err.Error())
		return result, appkitItem, err
	}

	jsonData, err := json.Marshal(appkitItem.Object["spec"])
	if err != nil {
		log.Errorf("Error marshalling applicationkit spec: %s", err.Error())
		return result, appkitItem, err
	}
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		log.Errorf("Error unmarshalling applicationkit spec: %s", err.Error())
		return result, appkitItem, err
	}

	return result, appkitItem, err
}

func ListApplicationKits(namespace string) (appkit []CrdApplicationKit, appkitRaw *unstructured.UnstructuredList, err error) {
	result := []CrdApplicationKit{}

	provider, err := kubernetes.NewDynamicKubeProvider(nil)
	if provider == nil || err != nil {
		log.Errorf("Error creating provider. Cannot continue because it is vital: %s", err.Error())
		return result, nil, err
	}

	appKitsGVR := schema.GroupVersionResource{Group: MogeniusGroup, Version: MogeniusVersion, Resource: MogeniusResourceApplicationKit}
	appkits, err := provider.ClientSet.Resource(appKitsGVR).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Errorf("Error getting applicationkit: %s", err.Error())
		return result, appkits, err
	}

	for _, appkit := range appkits.Items {
		entry := CrdApplicationKit{}
		jsonData, err := json.Marshal(appkit.Object["spec"])
		if err != nil {
			log.Errorf("Error marshalling applicationkit spec: %s", err.Error())
			return result, appkits, err
		}
		err = json.Unmarshal(jsonData, &entry)
		if err != nil {
			log.Errorf("Error unmarshalling applicationkit spec: %s", err.Error())
			return result, appkits, err
		}
		result = append(result, entry)
	}
	return result, appkits, err
}
