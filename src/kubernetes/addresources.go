package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"mogenius-k8s-manager/src/crds"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/utils"

	core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func CreateOrUpdateClusterSecret() (utils.ClusterSecret, error) {
	clientset := clientProvider.K8sClientSet()
	secretClient := clientset.CoreV1().Secrets(config.Get("MO_OWN_NAMESPACE"))

	existingSecret, getErr := secretClient.Get(context.TODO(), config.Get("MO_OWN_NAMESPACE"), metav1.GetOptions{})
	return writeMogeniusSecret(secretClient, existingSecret, getErr)
}

func GetValkeyPwd() (*string, error) {
	clientset := clientProvider.K8sClientSet()
	secretClient := clientset.CoreV1().Secrets(config.Get("MO_OWN_NAMESPACE"))

	existingSecret, getErr := secretClient.Get(context.TODO(), "mogenius-k8s-manager-valkey", metav1.GetOptions{})
	if getErr != nil {
		return nil, getErr
	}

	foundPwd := string(existingSecret.Data["valkey-password"])
	if foundPwd == "" {
		return nil, fmt.Errorf("valkey password not found")
	}

	return &foundPwd, nil
}

func writeMogeniusSecret(secretClient v1.SecretInterface, existingSecret *core.Secret, getErr error) (utils.ClusterSecret, error) {
	// CREATE NEW SECRET
	apikey := config.Get("MO_API_KEY")
	clusterName := config.Get("MO_CLUSTER_NAME")

	// Construct cluster secret object
	clusterSecret := utils.ClusterSecret{
		ApiKey:      apikey,
		ClusterName: clusterName,
	}
	if existingSecret != nil {
		if string(existingSecret.Data["cluster-mfa-id"]) != "" {
			clusterSecret.ClusterMfaId = string(existingSecret.Data["cluster-mfa-id"])
		}
	}
	if clusterSecret.ClusterMfaId == "" {
		clusterSecret.ClusterMfaId = utils.NanoId()
	}

	secret := utils.InitSecret()
	secret.ObjectMeta.Name = config.Get("MO_OWN_NAMESPACE")
	secret.ObjectMeta.Namespace = config.Get("MO_OWN_NAMESPACE")
	delete(secret.StringData, "exampleData") // delete example data
	secret.StringData["cluster-mfa-id"] = clusterSecret.ClusterMfaId
	secret.StringData["api-key"] = clusterSecret.ApiKey
	secret.StringData["cluster-name"] = clusterSecret.ClusterName

	if existingSecret == nil || getErr != nil {
		k8sLogger.Info("🔑 Creating new mogenius secret ...")
		result, err := secretClient.Create(context.TODO(), &secret, MoCreateOptions(config))
		if err != nil {
			k8sLogger.Error("Error creating mogenius secret.", "error", err)
			return clusterSecret, err
		}
		k8sLogger.Info("🔑 Created new mogenius secret", result.GetObjectMeta().GetName(), ".")
	} else {
		if string(existingSecret.Data["cluster-mfa-id"]) != clusterSecret.ClusterMfaId ||
			string(existingSecret.Data["api-key"]) != clusterSecret.ApiKey ||
			string(existingSecret.Data["cluster-name"]) != clusterSecret.ClusterName {
			k8sLogger.Info("🔑 Updating existing mogenius secret ...")
			result, err := secretClient.Update(context.TODO(), &secret, MoUpdateOptions(config))
			if err != nil {
				k8sLogger.Error("Error updating mogenius secret.", "error", err)
				return clusterSecret, err
			}
			k8sLogger.Info("🔑 Updated mogenius secret", result.GetObjectMeta().GetName(), ".")
		} else {
			k8sLogger.Info("🔑 Using existing mogenius secret.")
		}
	}

	return clusterSecret, nil
}

func InitOrUpdateCrds() {
	crds := crds.GetCRDs()
	for _, crd := range crds {
		err := CreateOrUpdateYamlString(crd.Content)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			k8sLogger.Error("error updating/creating mogenius CRD", "filename", crd.Filename, "error", err)
			shutdown.SendShutdownSignal(true)
			select {}
		} else {
			k8sLogger.Info("created/updated mogenius CRD 🚀", "filename", crd.Filename)
		}
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

	if _, err := dynamicResource.Create(context.TODO(), resource, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

// todo remove this function and move to new ApplyResource function
func CreateOrUpdateYamlString(yamlContent string) error {
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

	if _, err := dynamicResource.Create(context.TODO(), resource, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// get the current resourcerevision to update the existing object
			currentObject, _ := dynamicResource.Get(context.TODO(), resource.GetName(), metav1.GetOptions{})
			resource.SetResourceVersion(currentObject.GetResourceVersion())
			if _, err := dynamicResource.Update(context.TODO(), resource, metav1.UpdateOptions{}); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}
