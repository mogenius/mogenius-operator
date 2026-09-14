package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mogenius-operator/src/crds"
	"mogenius-operator/src/shutdown"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

func GetValkeyPwd() (*string, error) {
	clientset := clientProvider.K8sClientSet()
	secretClient := clientset.CoreV1().Secrets(config.Get("MO_OWN_NAMESPACE"))

	existingSecret, getErr := secretClient.Get(context.Background(), "mogenius-operator-valkey", metav1.GetOptions{})
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

	// A freshly created CRD is only served once the API server reports it
	// Established, typically well under a second later. Everything after this
	// point assumes the mogenius types are servable — watches, reconcilers, the
	// PlatformConfig seeder — so the short wait happens here, once, instead of
	// each consumer discovering NotFound on a type that exists. Non-fatal: the
	// consumers still handle NotFound, this only takes the race out of the
	// common path.
	for _, crd := range crds {
		name, err := crdNameFromYaml(crd.Content)
		if err != nil {
			k8sLogger.Warn("could not determine CRD name for establishment wait", "filename", crd.Filename, "error", err)
			continue
		}
		if err := waitForCrdEstablished(name, 30*time.Second); err != nil {
			k8sLogger.Warn("CRD not established yet, continuing anyway", "crd", name, "error", err)
		}
	}
}

// crdNameFromYaml reads the metadata.name out of a CRD manifest — the same
// decode the apply above does, repeated because the apply does not return it.
func crdNameFromYaml(yamlContent string) (string, error) {
	resource := &unstructured.Unstructured{}
	decUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	if _, _, err := decUnstructured.Decode([]byte(yamlContent), nil, resource); err != nil {
		return "", err
	}
	return resource.GetName(), nil
}

// waitForCrdEstablished polls the CRD until its Established condition is True,
// or the timeout passes.
func waitForCrdEstablished(name string, timeout time.Duration) error {
	crdResource := clientProvider.DynamicClient().Resource(schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	})

	deadline := time.Now().Add(timeout)
	for {
		crd, err := crdResource.Get(context.Background(), name, metav1.GetOptions{})
		if err == nil && crdIsEstablished(crd) {
			return nil
		}

		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("read CRD %s: %w", name, err)
			}
			return fmt.Errorf("CRD %s not established within %s", name, timeout)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// crdIsEstablished reports whether the CRD carries the Established=True
// condition, meaning the API server serves the type.
func crdIsEstablished(crd *unstructured.Unstructured) bool {
	conditions, _, _ := unstructured.NestedSlice(crd.Object, "status", "conditions")
	for _, entry := range conditions {
		condition, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if condition["type"] == "Established" && condition["status"] == "True" {
			return true
		}
	}
	return false
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
