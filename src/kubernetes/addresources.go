package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"mogenius-k8s-manager/src/crds"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/utils"

	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	applyconfapp "k8s.io/client-go/applyconfigurations/apps/v1"
	applyconfcore "k8s.io/client-go/applyconfigurations/core/v1"
	applyconfmeta "k8s.io/client-go/applyconfigurations/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func Deploy() {
	applyNamespace()
	err := addRbac()
	if err != nil {
		k8sLogger.Error("Error Creating RBAC. Aborting.", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	}
	addDeployment()
	_, err = CreateOrUpdateClusterSecret()
	if err != nil {
		k8sLogger.Error("Error Creating cluster secret. Aborting.", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	}
}

func addRbac() error {
	clientset := clientProvider.K8sClientSet()
	clusterRole := &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: CLUSTERROLENAME,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{"", "extensions", "apps"},
				Resources: RBACRESOURCES,
				Verbs:     []string{"list", "get", "watch", "create", "update"},
			},
		},
	}
	clusterRoleBinding := &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: CLUSTERROLEBINDINGNAME,
		},
		RoleRef: rbac.RoleRef{
			Name:     CLUSTERROLENAME,
			Kind:     "ClusterRole",
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbac.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      SERVICEACCOUNTNAME,
				Namespace: config.Get("MO_OWN_NAMESPACE"),
			},
		},
	}

	// CREATE RBAC
	k8sLogger.Info("Creating mogenius-k8s-manager RBAC ...")

	err := ApplyServiceAccount(SERVICEACCOUNTNAME, config.Get("MO_OWN_NAMESPACE"), nil)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	_, err = clientset.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, MoCreateOptions())
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	_, err = clientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, MoCreateOptions())
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	k8sLogger.Info("Created mogenius-k8s-manager RBAC.")
	return nil
}

func applyNamespace() {
	clientset := clientProvider.K8sClientSet()
	serviceClient := clientset.CoreV1().Namespaces()

	namespace := applyconfcore.Namespace(config.Get("MO_OWN_NAMESPACE"))

	applyOptions := metav1.ApplyOptions{
		Force:        true,
		FieldManager: GetOwnDeploymentName(),
	}

	k8sLogger.Info("Creating mogenius-k8s-manager namespace ...")
	result, err := serviceClient.Apply(context.TODO(), namespace, applyOptions)
	if err != nil {
		k8sLogger.Error("Error applying mogenius-k8s-manager namespace.", "error", err)
	}
	k8sLogger.Info("Created mogenius-k8s-manager namespace", result.GetObjectMeta().GetName(), ".")
}

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
		result, err := secretClient.Create(context.TODO(), &secret, MoCreateOptions())
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
			result, err := secretClient.Update(context.TODO(), &secret, MoUpdateOptions())
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
	err := CreateOrUpdateYamlString(utils.InitMogeniusCrdProjectsYaml())
	if err != nil && !apierrors.IsAlreadyExists(err) {
		k8sLogger.Error("Error updating/creating mogenius Project-CRDs.", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	} else {
		k8sLogger.Info("Created/updated mogenius Project-CRDs. 🚀")
	}

	err = CreateOrUpdateYamlString(utils.InitMogeniusCrdEnvironmentsYaml())
	if err != nil && !apierrors.IsAlreadyExists(err) {
		k8sLogger.Error("Error updating/creating mogenius Environment-CRDs.", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	} else {
		k8sLogger.Info("Created/updated mogenius Environment-CRDs. 🚀")
	}

	err = CreateOrUpdateYamlString(utils.InitMogeniusCrdApplicationKitYaml())
	if err != nil && !apierrors.IsAlreadyExists(err) {
		k8sLogger.Error("Error updating/creating mogenius ApplicationKit-CRDs.", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	} else {
		k8sLogger.Info("Created/updated mogenius ApplicationKit-CRDs. 🚀")
	}

	crds := crds.GetCRDs()
	for _, crd := range crds {
		err = CreateOrUpdateYamlString(crd.Content)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			k8sLogger.Error("error updating/creating mogenius CRD", "filename", crd.Filename, "error", err)
			shutdown.SendShutdownSignal(true)
			select {}
		} else {
			k8sLogger.Info("created/updated mogenius CRD 🚀", "filename", crd.Filename)
		}
	}
}

func addDeployment() {
	clientset := clientProvider.K8sClientSet()
	deploymentClient := clientset.AppsV1().Deployments(config.Get("MO_OWN_NAMESPACE"))

	deploymentContainer := applyconfcore.Container()
	deploymentContainer.WithImagePullPolicy(core.PullAlways)
	deploymentContainer.WithName(GetOwnDeploymentName())
	deploymentContainer.WithImage(DEPLOYMENTIMAGE)

	envVars := []applyconfcore.EnvVarApplyConfiguration{}
	envVars = append(envVars, applyconfcore.EnvVarApplyConfiguration{
		Name:  utils.Pointer("cluster_name"),
		Value: utils.Pointer("TestClusterFromCode"),
	})
	envVars = append(envVars, applyconfcore.EnvVarApplyConfiguration{
		Name:  utils.Pointer("api_key"),
		Value: utils.Pointer("94E23575-A689-4F88-8D67-215A274F4E6E"), // dont worry. this is a test key
	})
	deploymentContainer.Env = envVars
	agentResourceLimits := core.ResourceList{
		"cpu":               resource.MustParse("300m"),
		"memory":            resource.MustParse("256Mi"),
		"ephemeral-storage": resource.MustParse("100Mi"),
	}
	agentResourceRequests := core.ResourceList{
		"cpu":               resource.MustParse("100m"),
		"memory":            resource.MustParse("128Mi"),
		"ephemeral-storage": resource.MustParse("10Mi"),
	}
	agentResources := applyconfcore.ResourceRequirements().WithRequests(agentResourceRequests).WithLimits(agentResourceLimits)
	deploymentContainer.WithResources(agentResources)
	deploymentContainer.WithName(GetOwnDeploymentName())

	podSpec := applyconfcore.PodSpec()
	podSpec.WithTerminationGracePeriodSeconds(0)
	podSpec.WithServiceAccountName(SERVICEACCOUNTNAME)

	podSpec.WithContainers(deploymentContainer)

	applyOptions := metav1.ApplyOptions{
		Force:        true,
		FieldManager: GetOwnDeploymentName(),
	}

	labelSelector := applyconfmeta.LabelSelector()
	labelSelector.WithMatchLabels(map[string]string{"app": GetOwnDeploymentName()})

	podTemplate := applyconfcore.PodTemplateSpec()
	podTemplate.WithLabels(map[string]string{
		"app": GetOwnDeploymentName(),
	})
	podTemplate.WithSpec(podSpec)

	deployment := applyconfapp.Deployment(GetOwnDeploymentName(), config.Get("MO_OWN_NAMESPACE"))
	deployment.WithSpec(applyconfapp.DeploymentSpec().WithSelector(labelSelector).WithTemplate(podTemplate))

	// Create Deployment
	k8sLogger.Info("Creating mogenius-k8s-manager deployment ...")
	result, err := deploymentClient.Apply(context.TODO(), deployment, applyOptions)
	if err != nil {
		k8sLogger.Error("Error creating mogenius-k8s-manager deployment.", "error", err)
	}
	k8sLogger.Info("Created mogenius-k8s-manager deployment.", result.GetObjectMeta().GetName(), ".")
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
