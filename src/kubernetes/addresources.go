package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"mogenius-k8s-manager/src/crds"
	"mogenius-k8s-manager/src/shutdown"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

func GetValkeyPwd() (*string, error) {
	clientset := clientProvider.K8sClientSet()
	secretClient := clientset.CoreV1().Secrets(config.Get("MO_OWN_NAMESPACE"))

	existingSecret, getErr := secretClient.Get(context.Background(), "mogenius-k8s-manager-valkey", metav1.GetOptions{})
	if getErr != nil {
		return nil, getErr
	}

	foundPwd := string(existingSecret.Data["valkey-password"])
	if foundPwd == "" {
		return nil, fmt.Errorf("valkey password not found")
	}

	return &foundPwd, nil
}

func InitOrUpdateCrds() {
	crds := crds.GetCRDs()
	for _, crd := range crds {
		err := CreateOrUpdateYamlString(crd.Content)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			k8sLogger.Error("error updating/creating mogenius CRD", "filename", crd.Filename, "error", err)
			shutdown.SendShutdownSignal(true)
			select {}
		}

		k8sLogger.Info("created/updated mogenius CRD 🚀", "filename", crd.Filename)
	}
}

func CreateYamlString(yamlContent string) error {
	dynamicClient := clientProvider.DynamicClient()

	decUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	_, groupVersionKind, err := decUnstructured.Decode([]byte(yamlContent), nil, nil)
	if err != nil {
		return err
	}

	resource := &unstructured.Unstructured{}
	_, _, err = decUnstructured.Decode([]byte(yamlContent), nil, resource)
	if err != nil {
		return err
	}

	groupVersionResource := schema.GroupVersionResource{
		Group:    groupVersionKind.Group,
		Version:  groupVersionKind.Version,
		Resource: strings.ToLower(groupVersionKind.Kind) + "s",
	}

	dynamicResource := dynamicClient.Resource(groupVersionResource).Namespace(resource.GetNamespace())
	_, err = dynamicResource.Create(
		context.Background(),
		resource,
		metav1.CreateOptions{},
	)
	if err != nil {
		return err
	}

	return nil
}

func CreateOrUpdateYamlString(yamlContent string) error {
	dynamicClient := clientProvider.DynamicClient()
	decUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	resource := &unstructured.Unstructured{}
	_, groupVersionKind, err := decUnstructured.Decode([]byte(yamlContent), nil, resource)
	if err != nil {
		return err
	}

	groupVersionResource := schema.GroupVersionResource{
		Group:    groupVersionKind.Group,
		Version:  groupVersionKind.Version,
		Resource: strings.ToLower(groupVersionKind.Kind) + "s", // todo: pluralization is more complex than this must be improved, currently only used for mogenius CRDs
	}

	dynamicResource := dynamicClient.Resource(groupVersionResource).Namespace(resource.GetNamespace())

	if _, err := dynamicResource.Create(
		context.Background(),
		resource,
		metav1.CreateOptions{},
	); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// get the current resourcerevision to update the existing object
			currentObject, _ := dynamicResource.Get(
				context.Background(),
				resource.GetName(),
				metav1.GetOptions{},
			)
			resource.SetResourceVersion(currentObject.GetResourceVersion())
			if _, err := dynamicResource.Update(
				context.Background(),
				resource,
				metav1.UpdateOptions{},
			); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}
