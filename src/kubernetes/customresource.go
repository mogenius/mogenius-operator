package kubernetes

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

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
	err = client.Delete(context.Background(), name, backgroundDeleteOptions())
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
