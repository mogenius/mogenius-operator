package kubernetes

import (
	"context"
	"strings"
	"time"

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

		var res *unstructured.Unstructured
		var err error

		// check if fetched resource is ready 3x, but finally update it either way
		for i := 0; i < 3; i++ {
			res, err = GetResource(gvr.Group, gvr.Version, gvr.Resource, obj.GetName(), namespace, isClusterWideResource)
			if err != nil {
				return err
			}

			k8sLogger.Info("Resource retrieved", "resource", gvr.Resource, "name", res.GetName())

			if isReady(res) {
				break // resource is ready and probably won't change anymore before the next update
			}
			k8sLogger.Info("Resource not ready.  Retrying in 2 seconds...", "name", res.GetName())
			time.Sleep(2 * time.Second)
		}
		// Try update if already exists
		obj, err = client.Update(context.TODO(), res, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		k8sLogger.Info("Resource updated successfully ✅", "name", obj.GetName())

	} else {
		k8sLogger.Info("Resource created successfully ✅", "name", obj.GetName())
	}
	return nil
}

type ResourceStatus struct {
	// Object struct {
	Status struct {
		Conditions []struct {
			LastTransitionTime string `yaml:"lastTransitionTime"`
			Message            string `yaml:"message"`
			Reason             string `yaml:"reason"`
			Status             string `yaml:"status"`
			Type               string `yaml:"type"`
		} `yaml:"conditions"`
	} `yaml:"status"`
	// } `yaml:"Object"`
}

func isReady(res *unstructured.Unstructured) bool {
	// Convert res to []byte
	resBytes, err := res.MarshalJSON()
	if err != nil {
		k8sLogger.Error("Error converting res to []byte", "error", err)
		return false
	}
	var resourceStatus ResourceStatus
	// Unmarshal the YAML into the struct
	if err := yaml.Unmarshal(resBytes, &resourceStatus); err != nil {
		k8sLogger.Error("Error unmarshalling YAML", "error", err)
		return false
	}

	// Iterate through conditions to check if the resource is "Ready"
	for _, condition := range resourceStatus.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return true
		}
	}
	return false
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
	var client dynamic.NamespaceableResourceInterface = clientProvider.DynamicClient().Resource(gvr)

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
