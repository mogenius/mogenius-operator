package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/src/structs"

	json "github.com/json-iterator/go"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)

type NamespaceBackupResponse struct {
	NamespaceName string   `json:"namespaceName"`
	Data          string   `json:"data"`
	Messages      []string `json:"messages"`
}

func BackupNamespace(namespace string) (NamespaceBackupResponse, error) {
	result := NamespaceBackupResponse{
		NamespaceName: namespace,
	}
	skippedGroups := structs.NewUniqueStringArray()
	allResources := structs.NewUniqueStringArray()
	usedResources := structs.NewUniqueStringArray()

	// Get a list of all resource types in the cluster
	clientset := clientProvider.K8sClientSet()
	resourceList, err := clientset.Discovery().ServerPreferredResources()
	if err != nil {
		return result, err
	}

	output := ""
	if namespace != "" {
		output = namespaceString(namespace)
	}
	// Iterate over each resource type and backup all resources in the namespace
	for _, resource := range resourceList {
		gv, _ := schema.ParseGroupVersion(resource.GroupVersion)
		if len(resource.APIResources) <= 0 {
			continue
		}

		for _, aApiResource := range resource.APIResources {
			allResources.Add(aApiResource.Name)
			if !aApiResource.Namespaced && namespace != "" {
				skippedGroups.Add(aApiResource.Name)
				continue
			}

			resourceId := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: aApiResource.Name,
			}
			// Get the REST client for this resource type
			restClient := dynamic.New(clientset.RESTClient()).Resource(resourceId).Namespace(namespace)

			// Get a list of all resources of this type in the namespace
			list, err := restClient.List(context.Background(), v1.ListOptions{})
			if err != nil {
				result.Messages = append(result.Messages, fmt.Sprintf("(LIST) %s: %s", resourceId.Resource, err.Error()))
				continue
			}

			if len(list.Items) > 0 {
				usedResources.Add(aApiResource.Name)
			}

			// Iterate over each resource and write it to a file
			for _, obj := range list.Items {
				if obj.GetKind() == "Event" {
					continue
				}
				output = output + "---\n"
				result.Messages = append(result.Messages, fmt.Sprintf("(SUCCESS) %s: %s/%s", resourceId.Resource, obj.GetNamespace(), obj.GetName()))

				obj = cleanBackupResources(obj)

				json, err := json.Marshal(obj.Object)
				if err != nil {
					return result, err
				}
				data, err := yaml.JSONToYAML(json)
				if err != nil {
					return result, err
				}
				output = output + string(data)
			}
		}
	}
	result.Data = output

	//os.WriteFile("/Users/bene/Desktop/omg.yaml", []byte(output), 0777)

	k8sLogger.Info("ALL", "resources", allResources.Display())
	k8sLogger.Info("SKIPPED", "resources", skippedGroups.Display())
	k8sLogger.Info("USED", "resources", usedResources.Display())

	return result, nil
}

func namespaceString(ns string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
	name: %s
`, ns)
}

func cleanBackupResources(obj unstructured.Unstructured) unstructured.Unstructured {
	obj.SetManagedFields(nil)
	delete(obj.Object, "status")
	obj.SetUID("")
	obj.SetResourceVersion("")
	obj.SetCreationTimestamp(v1.Time{})

	if obj.GetKind() == "PersistentVolumeClaim" {
		if nested, ok := obj.Object["spec"].(map[string]interface{}); ok {
			delete(nested, "volumeName")
			obj.Object["spec"] = nested
		}

		cleanedAnnotations := obj.GetAnnotations()
		delete(cleanedAnnotations, "pv.kubernetes.io/bind-completed")
		obj.SetAnnotations(cleanedAnnotations)
	}

	return obj
}
