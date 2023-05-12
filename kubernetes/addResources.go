package kubernetes

import (
	"context"
	"os"

	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	"github.com/google/uuid"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyconfapp "k8s.io/client-go/applyconfigurations/apps/v1"
	applyconfcore "k8s.io/client-go/applyconfigurations/core/v1"
	applyconfmeta "k8s.io/client-go/applyconfigurations/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func Deploy() {
	provider, err := NewKubeProviderLocal()
	if err != nil {
		panic(err)
	}

	applyNamespace(provider)
	addRbac(provider)
	addDeployment(provider)
	_, err = CreateClusterSecretIfNotExist(false)
	if err != nil {
		logger.Log.Fatalf("Error Creating cluster secret. Aborting: %s.", err.Error())
	}
}

func addRbac(kubeProvider *KubeProvider) error {
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
	_, err := kubeProvider.ClientSet.CoreV1().ServiceAccounts(NAMESPACE).Create(context.TODO(), serviceAccount, MoCreateOptions())
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	_, err = kubeProvider.ClientSet.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, MoCreateOptions())
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	_, err = kubeProvider.ClientSet.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, MoCreateOptions())
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	logger.Log.Info("Created mogenius-k8s-manager RBAC.")
	return nil
}

func applyNamespace(kubeProvider *KubeProvider) {
	serviceClient := kubeProvider.ClientSet.CoreV1().Namespaces()

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

func CreateClusterSecretIfNotExist(runsInCluster bool) (utils.ClusterSecret, error) {
	var kubeProvider *KubeProvider
	var err error
	if runsInCluster {
		kubeProvider, err = NewKubeProviderInCluster()
	} else {
		kubeProvider, err = NewKubeProviderLocal()
	}

	if err != nil {
		logger.Log.Errorf("CreateClusterSecretIfNotExist ERROR: %s", err.Error())
	}

	secretClient := kubeProvider.ClientSet.CoreV1().Secrets(NAMESPACE)

	existingSecret, _ := secretClient.Get(context.TODO(), NAMESPACE, metav1.GetOptions{})
	writeMogeniusSecret(secretClient, runsInCluster, existingSecret)

	logger.Log.Info("Using existing mogenius secret.")
	return utils.ClusterSecret{
		ApiKey:       string(existingSecret.Data["api-key"]),
		ClusterMfaId: string(existingSecret.Data["cluster-mfa-id"]),
		ClusterName:  string(existingSecret.Data["cluster-name"]),
	}, nil
}

func writeMogeniusSecret(secretClient v1.SecretInterface, runsInCluster bool, existingSecret *core.Secret) (utils.ClusterSecret, error) {
	// CREATE NEW SECRET
	apikey := os.Getenv("api_key")
	if apikey == "" {
		if runsInCluster {
			logger.Log.Fatal("Environment Variable 'api_key' is missing.")
		} else {
			apikey = utils.CONFIG.Kubernetes.ApiKey
		}
	}
	clusterName := os.Getenv("cluster_name")
	if clusterName == "" {
		if runsInCluster {
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
	if !runsInCluster {
		return clusterSecret, nil
	}

	secret := utils.InitSecret()
	secret.ObjectMeta.Name = NAMESPACE
	secret.ObjectMeta.Namespace = NAMESPACE
	delete(secret.StringData, "PRIVATE_KEY") // delete example data
	secret.StringData["cluster-mfa-id"] = clusterSecret.ClusterMfaId
	secret.StringData["api-key"] = clusterSecret.ApiKey
	secret.StringData["cluster-name"] = clusterSecret.ClusterName

	if existingSecret == nil {
		logger.Log.Info("Creating mogenius secret ...")
		result, err := secretClient.Create(context.TODO(), &secret, MoCreateOptions())
		if err != nil {
			logger.Log.Error(err)
			return clusterSecret, err
		}
		logger.Log.Info("Created mogenius secret", result.GetObjectMeta().GetName(), ".")
	} else {
		if string(existingSecret.Data["api-key"]) != clusterSecret.ApiKey ||
			string(existingSecret.Data["cluster-mfa-id"]) != clusterSecret.ClusterMfaId ||
			string(existingSecret.Data["cluster-name"]) != clusterName {
			logger.Log.Info("Updating mogenius secret ...")
			result, err := secretClient.Update(context.TODO(), &secret, MoUpdateOptions())
			if err != nil {
				logger.Log.Error(err)
				return clusterSecret, err
			}
			logger.Log.Info("Updated mogenius secret", result.GetObjectMeta().GetName(), ".")
		} else {
			logger.Log.Info("Using existing mogenius secret.")
		}
	}

	return clusterSecret, nil
}

func addDeployment(kubeProvider *KubeProvider) {
	deploymentClient := kubeProvider.ClientSet.AppsV1().Deployments(NAMESPACE)

	deploymentContainer := applyconfcore.Container()
	deploymentContainer.WithImagePullPolicy(core.PullAlways)
	deploymentContainer.WithName(DEPLOYMENTNAME)
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
