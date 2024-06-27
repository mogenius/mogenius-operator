package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"github.com/mogenius/punq/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	scheme "k8s.io/client-go/kubernetes/scheme"
)

func ApplyResource(yamlData string, isClusterWideResource bool) error {
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

	client, err := getClient(gvr, namespace, isClusterWideResource)
	if err != nil {
		return err
	}
	_, err = client.Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil {
		// get fresh metadata about existing resource
		gvr := getGVR(obj)
		namespace := obj.GetNamespace()
		res, err := GetResource(gvr.Group, gvr.Version, gvr.Resource, obj.GetName(), namespace, isClusterWideResource)
		if err != nil {
			return err
		} else {
			logger.Log.Info(fmt.Sprintf("Resource retrieved %s:%s", gvr.Resource, res.GetName()))
		}
		// Try update if already exists
		obj, err = client.Update(context.TODO(), res, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		logger.Log.Info("Resource updated successfully ✅: " + obj.GetName())

	} else {
		logger.Log.Info("Resource created successfully ✅: " + obj.GetName())
	}
	return nil
}

func GetResource(group string, version string, resource string, name string, namespace string, isClusterWideResource bool) (*unstructured.Unstructured, error) {
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: strings.ToLower(resource),
	}
	client, err := getClient(gvr, namespace, isClusterWideResource)
	if err != nil {
		return nil, err
	}
	resourceResult, err := client.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return resourceResult, nil
}

func ListResources(group string, version string, resource string, namespace string, isClusterWideResource bool) (*unstructured.UnstructuredList, error) {
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: strings.ToLower(resource),
	}

	client, err := getClient(gvr, namespace, isClusterWideResource)
	if err != nil {
		return nil, err
	}
	resourceResult, err := client.List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		return nil, err
	}
	return resourceResult, nil

}

func DeleteResource(group string, version string, resource string, name string, namespace string, isClusterWideResource bool) error {

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: strings.ToLower(resource),
	}

	client, err := getClient(gvr, namespace, isClusterWideResource)
	if err != nil {
		return err
	}
	err = client.Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func getClient(gvr schema.GroupVersionResource, namespace string, isClusterWideResource bool) (dynamic.ResourceInterface, error) {
	provider, err := NewDynamicKubeProvider(nil)
	if err != nil {
		return nil, err
	}

	var client dynamic.NamespaceableResourceInterface = provider.ClientSet.Resource(gvr)

	if !isClusterWideResource {
		if namespace == "" {
			namespace = "default"
		}
		return client.Namespace(namespace), nil
	} else {
		return client, nil
	}
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
