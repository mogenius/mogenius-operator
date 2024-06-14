package kubernetes

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	scheme "k8s.io/client-go/kubernetes/scheme"
)

func ApplyResource(yamlData string) error {
	provider, err := NewDynamicKubeProvider(nil)
	if err != nil {
		return err
	}

	jsonData, err := yaml.ToJSON([]byte(yamlData))
	if err != nil {
		return err
	}

	obj := &unstructured.Unstructured{}
	dec := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	_, _, err = dec.Decode(jsonData, nil, obj)
	if err != nil {
		return err
	}

	gvr := getGVR(obj)

	namespace := obj.GetNamespace()
	if namespace == "" {
		namespace = "default"
	}

	_, err = provider.ClientSet.Resource(gvr).Namespace(namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil {
		// Try update if already exists
		_, err = provider.ClientSet.Resource(gvr).Namespace(namespace).Update(context.TODO(), obj, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		fmt.Println("Resource updated successfully")
	} else {
		fmt.Println("Resource created successfully")
	}
	return nil
}

func GetResource(group string, version string, resource string, name string, namespace string) (*unstructured.Unstructured, error) {
	provider, err := NewDynamicKubeProvider(nil)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: strings.ToLower(resource),
	}

	if namespace == "" {
		resourceResult, err := provider.ClientSet.Resource(gvr).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return resourceResult, nil
	} else {
		resourceResult, err := provider.ClientSet.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return resourceResult, nil
	}
}

func ListResources(group string, version string, resource string, namespace string) (*unstructured.UnstructuredList, error) {
	provider, err := NewDynamicKubeProvider(nil)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: strings.ToLower(resource),
	}

	if namespace == "" {
		resourceResult, err := provider.ClientSet.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		return resourceResult, nil
	} else {
		resourceResult, err := provider.ClientSet.Resource(gvr).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		return resourceResult, nil
	}
}

func DeleteResource(group string, version string, resource string, name string, namespace string) error {
	provider, err := NewDynamicKubeProvider(nil)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: strings.ToLower(resource),
	}

	if namespace == "" {
		err = provider.ClientSet.Resource(gvr).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	} else {
		err = provider.ClientSet.Resource(gvr).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func getGVR(obj *unstructured.Unstructured) schema.GroupVersionResource {
	group := obj.GroupVersionKind().Group
	version := obj.GroupVersionKind().Version
	kind := obj.GroupVersionKind().Kind

	// Handle core resources
	if group == "" {
		group = ""
	}

	// Pluralize the kind
	resource := strings.ToLower(kind) + "s"

	// Special case handling (if needed) can be added here

	return schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}
}
