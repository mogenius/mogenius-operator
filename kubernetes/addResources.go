package kubernetes

import (
	"context"
	"os"
	"strings"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"

	"github.com/google/uuid"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	applyconfapp "k8s.io/client-go/applyconfigurations/apps/v1"
	applyconfcore "k8s.io/client-go/applyconfigurations/core/v1"
	applyconfmeta "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/dynamic"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func Deploy() {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return
	}
	if provider == nil || err != nil {
		panic("Error creating kubeprovider")
	}

	applyNamespace(provider)
	addRbac(provider)
	addDeployment(provider)
	_, err = CreateClusterSecretIfNotExist()
	if err != nil {
		logger.Log.Fatalf("Error Creating cluster secret. Aborting: %s.", err.Error())
	}
}

func addRbac(provider *punq.KubeProvider) error {
	serviceAccount := &core.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: SERVICEACCOUNTNAME,
		},
	}
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
				Namespace: NAMESPACE,
			},
		},
	}

	// CREATE RBAC
	logger.Log.Info("Creating mogenius-k8s-manager RBAC ...")
	_, err := provider.ClientSet.CoreV1().ServiceAccounts(NAMESPACE).Create(context.TODO(), serviceAccount, MoCreateOptions())
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	_, err = provider.ClientSet.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, MoCreateOptions())
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	_, err = provider.ClientSet.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, MoCreateOptions())
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	logger.Log.Info("Created mogenius-k8s-manager RBAC.")
	return nil
}

func applyNamespace(provider *punq.KubeProvider) {
	serviceClient := provider.ClientSet.CoreV1().Namespaces()

	namespace := applyconfcore.Namespace(NAMESPACE)

	applyOptions := metav1.ApplyOptions{
		Force:        true,
		FieldManager: DEPLOYMENTNAME,
	}

	logger.Log.Info("Creating mogenius-k8s-manager namespace ...")
	result, err := serviceClient.Apply(context.TODO(), namespace, applyOptions)
	if err != nil {
		logger.Log.Error(err)
	}
	logger.Log.Info("Created mogenius-k8s-manager namespace", result.GetObjectMeta().GetName(), ".")
}

func CreateClusterSecretIfNotExist() (utils.ClusterSecret, error) {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		logger.Log.Fatal("Error creating kubeprovider")
	}

	secretClient := provider.ClientSet.CoreV1().Secrets(NAMESPACE)

	existingSecret, getErr := secretClient.Get(context.TODO(), NAMESPACE, metav1.GetOptions{})
	return writeMogeniusSecret(secretClient, existingSecret, getErr)
}

func writeMogeniusSecret(secretClient v1.SecretInterface, existingSecret *core.Secret, getErr error) (utils.ClusterSecret, error) {
	// CREATE NEW SECRET
	apikey := os.Getenv("api_key")
	if apikey == "" {
		if utils.CONFIG.Kubernetes.RunInCluster {
			logger.Log.Fatal("Environment Variable 'api_key' is missing.")
		} else {
			apikey = utils.CONFIG.Kubernetes.ApiKey
		}
	}
	clusterName := os.Getenv("cluster_name")
	if clusterName == "" {
		if utils.CONFIG.Kubernetes.RunInCluster {
			logger.Log.Fatal("Environment Variable 'cluster_name' is missing.")
		} else {
			clusterName = utils.CONFIG.Kubernetes.ClusterName
		}
	}

	clusterSecret := utils.ClusterSecret{
		ApiKey:       apikey,
		ClusterMfaId: uuid.New().String(),
		ClusterName:  clusterName,
	}

	// This prevents lokal k8s-manager installations from overwriting cluster secrets
	if !utils.CONFIG.Kubernetes.RunInCluster {
		return clusterSecret, nil
	}

	secret := punqUtils.InitSecret()
	secret.ObjectMeta.Name = NAMESPACE
	secret.ObjectMeta.Namespace = NAMESPACE
	delete(secret.StringData, "PRIVATE_KEY") // delete example data
	secret.StringData["cluster-mfa-id"] = clusterSecret.ClusterMfaId
	secret.StringData["api-key"] = clusterSecret.ApiKey
	secret.StringData["cluster-name"] = clusterSecret.ClusterName

	if existingSecret == nil || getErr != nil {
		logger.Log.Info("Creating new mogenius secret ...")
		result, err := secretClient.Create(context.TODO(), &secret, MoCreateOptions())
		if err != nil {
			logger.Log.Error(err)
			return clusterSecret, err
		}
		logger.Log.Info("Created new mogenius secret", result.GetObjectMeta().GetName(), ".")
	} else {
		if string(existingSecret.Data["api-key"]) != clusterSecret.ApiKey ||
			string(existingSecret.Data["cluster-name"]) != clusterSecret.ClusterName {
			logger.Log.Info("Updating existing mogenius secret ...")
			// keep existing mfa-id if possible
			if string(existingSecret.Data["cluster-mfa-id"]) != "" {
				clusterSecret.ClusterMfaId = string(existingSecret.Data["cluster-mfa-id"])
				secret.StringData["cluster-mfa-id"] = clusterSecret.ClusterMfaId
			}
			result, err := secretClient.Update(context.TODO(), &secret, MoUpdateOptions())
			if err != nil {
				logger.Log.Error(err)
				return clusterSecret, err
			}
			logger.Log.Info("Updated mogenius secret", result.GetObjectMeta().GetName(), ".")
		} else {
			clusterSecret.ClusterMfaId = string(existingSecret.Data["cluster-mfa-id"])
			logger.Log.Info("Using existing mogenius secret.")
		}
	}

	return clusterSecret, nil
}

func addDeployment(provider *punq.KubeProvider) {
	deploymentClient := provider.ClientSet.AppsV1().Deployments(NAMESPACE)

	deploymentContainer := applyconfcore.Container()
	deploymentContainer.WithImagePullPolicy(core.PullAlways)
	deploymentContainer.WithName(DEPLOYMENTNAME)
	deploymentContainer.WithImage(DEPLOYMENTIMAGE)

	envVars := []applyconfcore.EnvVarApplyConfiguration{}
	envVars = append(envVars, applyconfcore.EnvVarApplyConfiguration{
		Name:  punqUtils.Pointer("cluster_name"),
		Value: punqUtils.Pointer("TestClusterFromCode"),
	})
	envVars = append(envVars, applyconfcore.EnvVarApplyConfiguration{
		Name:  punqUtils.Pointer("api_key"),
		Value: punqUtils.Pointer("94E23575-A689-4F88-8D67-215A274F4E6E"), // dont worry. this is a test key
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
	deploymentContainer.WithName(DEPLOYMENTNAME)

	podSpec := applyconfcore.PodSpec()
	podSpec.WithTerminationGracePeriodSeconds(0)
	podSpec.WithServiceAccountName(SERVICEACCOUNTNAME)

	podSpec.WithContainers(deploymentContainer)

	applyOptions := metav1.ApplyOptions{
		Force:        true,
		FieldManager: DEPLOYMENTNAME,
	}

	labelSelector := applyconfmeta.LabelSelector()
	labelSelector.WithMatchLabels(map[string]string{"app": DEPLOYMENTNAME})

	podTemplate := applyconfcore.PodTemplateSpec()
	podTemplate.WithLabels(map[string]string{
		"app": DEPLOYMENTNAME,
	})
	podTemplate.WithSpec(podSpec)

	deployment := applyconfapp.Deployment(DEPLOYMENTNAME, NAMESPACE)
	deployment.WithSpec(applyconfapp.DeploymentSpec().WithSelector(labelSelector).WithTemplate(podTemplate))

	// Create Deployment
	logger.Log.Info("Creating mogenius-k8s-manager deployment ...")
	result, err := deploymentClient.Apply(context.TODO(), deployment, applyOptions)
	if err != nil {
		logger.Log.Error(err)
	}
	logger.Log.Info("Created mogenius-k8s-manager deployment.", result.GetObjectMeta().GetName(), ".")
}

func ApplyYamlString(yamlContent string) error {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(&provider.ClientConfig)
	if err != nil {
		return err
	}

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
