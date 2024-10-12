package kubernetes

import (
	"context"
	"fmt"
	"os"
	"strings"

	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/utils"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"

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
	"k8s.io/client-go/dynamic"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func Deploy() {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		K8sLogger.Fatal("Error creating kubeprovider")
	}

	applyNamespace(provider)
	err = addRbac(provider)
	if err != nil {
		K8sLogger.Fatalf("Error Creating RBAC. Aborting: %s.", err.Error())
	}
	addDeployment(provider)
	_, err = CreateOrUpdateClusterSecret(nil)
	if err != nil {
		K8sLogger.Fatalf("Error Creating cluster secret. Aborting: %s.", err.Error())
	}
}

func addRbac(provider *punq.KubeProvider) error {
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
	K8sLogger.Info("Creating mogenius-k8s-manager RBAC ...")

	err := ApplyServiceAccount(SERVICEACCOUNTNAME, NAMESPACE, nil)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	_, err = provider.ClientSet.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, MoCreateOptions())
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	_, err = provider.ClientSet.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, MoCreateOptions())
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	K8sLogger.Info("Created mogenius-k8s-manager RBAC.")
	return nil
}

func applyNamespace(provider *punq.KubeProvider) {
	serviceClient := provider.ClientSet.CoreV1().Namespaces()

	namespace := applyconfcore.Namespace(NAMESPACE)

	applyOptions := metav1.ApplyOptions{
		Force:        true,
		FieldManager: DEPLOYMENTNAME,
	}

	K8sLogger.Info("Creating mogenius-k8s-manager namespace ...")
	result, err := serviceClient.Apply(context.TODO(), namespace, applyOptions)
	if err != nil {
		K8sLogger.Error(err)
	}
	K8sLogger.Info("Created mogenius-k8s-manager namespace", result.GetObjectMeta().GetName(), ".")
}

func UpdateSynRepoData(syncRepoReq *dtos.SyncRepoData) error {
	// Save previous data for comparison
	previousData, err := GetSyncRepoData()
	if err != nil {
		return err
	}

	// update data
	secret, err := CreateOrUpdateClusterSecret(syncRepoReq)
	if err == nil {
		utils.CONFIG.Iac.RepoUrl = secret.SyncRepoUrl
		utils.CONFIG.Iac.RepoPat = secret.SyncRepoPat
		utils.CONFIG.Iac.RepoBranch = secret.SyncRepoBranch
		utils.CONFIG.Iac.AllowPull = secret.SyncAllowPull
		utils.CONFIG.Iac.AllowPush = secret.SyncAllowPush
		utils.CONFIG.Iac.SyncFrequencyInSec = secret.SyncFrequencyInSec
		utils.CONFIG.Iac.SyncWorkloads = secret.SyncWorkloads
		utils.CONFIG.Iac.IgnoredNamespaces = secret.IgnoredNamespaces
	}

	// check if essential data is changed
	if previousData.Repo != syncRepoReq.Repo ||
		syncRepoReq.Pat != "***" ||
		previousData.Branch != syncRepoReq.Branch ||
		previousData.AllowPull != syncRepoReq.AllowPull ||
		previousData.AllowPush != syncRepoReq.AllowPush {
		K8sLogger.Warn("⚠️ ⚠️ ⚠️  SyncRepoData has changed in a way that requires the deletion of current repo ...")
		IacManagerSetupInProcess = true
		defer func() {
			IacManagerSetupInProcess = false
		}()
		// Push/Pull
		if syncRepoReq.AllowPull && syncRepoReq.AllowPush {
			err := ResetLocalRepo()
			if err != nil {
				return err
			}
		}
		// Push
		if !syncRepoReq.AllowPull && syncRepoReq.AllowPush {
			err = IacManagerResetCurrentRepoData(IacManagerDeleteDataRetries)
			if err != nil {
				return err
			}
			IacManagerSetupInProcess = false
			InitAllWorkloads()
		}
		// Pull
		if syncRepoReq.AllowPull && !syncRepoReq.AllowPush {
			err := ResetLocalRepo()
			if err != nil {
				return err
			}
		}
		// None
		if !syncRepoReq.AllowPull && !syncRepoReq.AllowPush {
			err = IacManagerResetCurrentRepoData(IacManagerDeleteDataRetries)
			if err != nil {
				return err
			}
		}
	}
	return err
}

func ResetLocalRepo() error {
	IacManagerSetupInProcess = true
	err := IacManagerResetCurrentRepoData(IacManagerDeleteDataRetries)
	if err != nil {
		return err
	}
	IacManagerSetupInProcess = false
	InitAllWorkloads()
	err = IacManagerSyncChanges()
	if err != nil {
		return err
	}
	err = IacManagerApplyRepoStateToCluster()
	if err != nil {
		return err
	}

	return nil
}

func CreateOrUpdateClusterSecret(syncRepoReq *dtos.SyncRepoData) (utils.ClusterSecret, error) {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		K8sLogger.Fatal("Error creating kubeprovider")
	}

	secretClient := provider.ClientSet.CoreV1().Secrets(NAMESPACE)

	existingSecret, getErr := secretClient.Get(context.TODO(), NAMESPACE, metav1.GetOptions{})
	return writeMogeniusSecret(secretClient, existingSecret, getErr, syncRepoReq)
}

func GetSyncRepoData() (*dtos.SyncRepoData, error) {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		K8sLogger.Fatal("Error creating kubeprovider")
	}

	secretClient := provider.ClientSet.CoreV1().Secrets(NAMESPACE)

	existingSecret, getErr := secretClient.Get(context.TODO(), NAMESPACE, metav1.GetOptions{})
	if getErr != nil {
		return nil, getErr
	}

	result := dtos.CreateSyncRepoDataFrom(existingSecret)
	if result.Pat != "" {
		result.Pat = "***"
	}
	return &result, nil
}

func writeMogeniusSecret(secretClient v1.SecretInterface, existingSecret *core.Secret, getErr error, syncRepoReq *dtos.SyncRepoData) (utils.ClusterSecret, error) {
	// CREATE NEW SECRET
	apikey := os.Getenv("api_key")
	if apikey == "" {
		if utils.CONFIG.Kubernetes.RunInCluster {
			K8sLogger.Fatal("Environment Variable 'api_key' is missing.")
		} else {
			apikey = utils.CONFIG.Kubernetes.ApiKey
		}
	}
	clusterName := os.Getenv("cluster_name")
	if clusterName == "" {
		if utils.CONFIG.Kubernetes.RunInCluster {
			K8sLogger.Fatal("Environment Variable 'cluster_name' is missing.")
		} else {
			clusterName = utils.CONFIG.Kubernetes.ClusterName
		}
	}

	// Construct cluster secret object
	clusterSecret := utils.ClusterSecret{
		ApiKey:      apikey,
		ClusterName: clusterName,
	}
	if existingSecret != nil {
		if string(existingSecret.Data["cluster-mfa-id"]) != "" {
			clusterSecret.ClusterMfaId = string(existingSecret.Data["cluster-mfa-id"])
		}
		syncdata := dtos.CreateSyncRepoDataFrom(existingSecret)
		clusterSecret.SyncRepoUrl = syncdata.Repo
		clusterSecret.SyncRepoPat = syncdata.Pat
		clusterSecret.SyncRepoBranch = syncdata.Branch
		clusterSecret.SyncAllowPull = syncdata.AllowPull
		clusterSecret.SyncAllowPush = syncdata.AllowPush
		clusterSecret.SyncFrequencyInSec = syncdata.SyncFrequencyInSec
		clusterSecret.SyncWorkloads = syncdata.SyncWorkloads
		if len(syncdata.IgnoredNamespaces) > 1 {
			clusterSecret.IgnoredNamespaces = syncdata.IgnoredNamespaces
		} else {
			clusterSecret.IgnoredNamespaces = dtos.DefaultIgnoredNamespaces()
		}
	}
	if clusterSecret.ClusterMfaId == "" {
		clusterSecret.ClusterMfaId = punqUtils.NanoId()
	}
	if syncRepoReq != nil {
		clusterSecret.SyncRepoUrl = syncRepoReq.Repo
		if syncRepoReq.Pat != "***" {
			clusterSecret.SyncRepoPat = syncRepoReq.Pat
		}
		clusterSecret.SyncRepoBranch = syncRepoReq.Branch
		clusterSecret.SyncAllowPull = syncRepoReq.AllowPull
		clusterSecret.SyncAllowPush = syncRepoReq.AllowPush
		clusterSecret.SyncFrequencyInSec = syncRepoReq.SyncFrequencyInSec
		clusterSecret.SyncWorkloads = syncRepoReq.SyncWorkloads
		if len(syncRepoReq.IgnoredNamespaces) > 1 {
			clusterSecret.IgnoredNamespaces = syncRepoReq.IgnoredNamespaces
		}
	}

	secret := punqUtils.InitSecret()
	secret.ObjectMeta.Name = NAMESPACE
	secret.ObjectMeta.Namespace = NAMESPACE
	delete(secret.StringData, "exampleData") // delete example data
	secret.StringData["cluster-mfa-id"] = clusterSecret.ClusterMfaId
	secret.StringData["api-key"] = clusterSecret.ApiKey
	secret.StringData["cluster-name"] = clusterSecret.ClusterName
	secret.StringData["sync-repo-url"] = clusterSecret.SyncRepoUrl
	secret.StringData["sync-repo-pat"] = clusterSecret.SyncRepoPat
	secret.StringData["sync-repo-branch"] = clusterSecret.SyncRepoBranch
	secret.StringData["sync-allow-pull"] = fmt.Sprintf("%t", clusterSecret.SyncAllowPull)
	secret.StringData["sync-allow-push"] = fmt.Sprintf("%t", clusterSecret.SyncAllowPush)
	secret.StringData["sync-frequency-in-sec"] = fmt.Sprintf("%d", clusterSecret.SyncFrequencyInSec)
	secret.StringData["sync-workloads"] = strings.Join(clusterSecret.SyncWorkloads, ",")
	secret.StringData["sync-ignored-namespaces"] = strings.Join(clusterSecret.IgnoredNamespaces, ",")

	if existingSecret == nil || getErr != nil {
		K8sLogger.Info("🔑 Creating new mogenius secret ...")
		result, err := secretClient.Create(context.TODO(), &secret, MoCreateOptions())
		if err != nil {
			K8sLogger.Error(err)
			return clusterSecret, err
		}
		K8sLogger.Info("🔑 Created new mogenius secret", result.GetObjectMeta().GetName(), ".")
	} else {
		if string(existingSecret.Data["cluster-mfa-id"]) != clusterSecret.ClusterMfaId ||
			string(existingSecret.Data["api-key"]) != clusterSecret.ApiKey ||
			string(existingSecret.Data["cluster-name"]) != clusterSecret.ClusterName ||
			string(existingSecret.Data["sync-repo-url"]) != clusterSecret.SyncRepoUrl ||
			string(existingSecret.Data["sync-repo-pat"]) != clusterSecret.SyncRepoPat ||
			string(existingSecret.Data["sync-repo-branch"]) != clusterSecret.SyncRepoBranch ||
			string(existingSecret.Data["sync-allow-pull"]) != fmt.Sprintf("%t", clusterSecret.SyncAllowPull) ||
			string(existingSecret.Data["sync-allow-push"]) != fmt.Sprintf("%t", clusterSecret.SyncAllowPush) ||
			string(existingSecret.Data["sync-frequency-in-sec"]) != fmt.Sprintf("%d", clusterSecret.SyncFrequencyInSec) ||
			string(existingSecret.Data["sync-workloads"]) != strings.Join(clusterSecret.SyncWorkloads, ",") ||
			string(existingSecret.Data["sync-ignored-namespaces"]) != strings.Join(clusterSecret.IgnoredNamespaces, ",") {
			K8sLogger.Info("🔑 Updating existing mogenius secret ...")
			result, err := secretClient.Update(context.TODO(), &secret, MoUpdateOptions())
			if err != nil {
				K8sLogger.Error(err)
				return clusterSecret, err
			}
			K8sLogger.Info("🔑 Updated mogenius secret", result.GetObjectMeta().GetName(), ".")
		} else {
			K8sLogger.Info("🔑 Using existing mogenius secret.")
		}
	}

	return clusterSecret, nil
}

func InitOrUpdateCrds() {
	err := CreateOrUpdateYamlString(utils.InitMogeniusCrdProjectsYaml())
	if err != nil && !apierrors.IsAlreadyExists(err) {
		K8sLogger.Fatalf("Error updating/creating mogenius Project-CRDs: %s", err.Error())
	} else {
		K8sLogger.Info("Created/updated mogenius Project-CRDs. 🚀")
	}

	err = CreateOrUpdateYamlString(utils.InitMogeniusCrdEnvironmentsYaml())
	if err != nil && !apierrors.IsAlreadyExists(err) {
		K8sLogger.Fatalf("Error updating/creating mogenius Environment-CRDs: %s", err.Error())
	} else {
		K8sLogger.Info("Created/updated mogenius Environment-CRDs. 🚀")
	}

	err = CreateOrUpdateYamlString(utils.InitMogeniusCrdApplicationKitYaml())
	if err != nil && !apierrors.IsAlreadyExists(err) {
		K8sLogger.Fatalf("Error updating/creating mogenius ApplicationKit-CRDs: %s", err.Error())
	} else {
		K8sLogger.Info("Created/updated mogenius ApplicationKit-CRDs. 🚀")
	}
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
	K8sLogger.Info("Creating mogenius-k8s-manager deployment ...")
	result, err := deploymentClient.Apply(context.TODO(), deployment, applyOptions)
	if err != nil {
		K8sLogger.Error(err)
	}
	K8sLogger.Info("Created mogenius-k8s-manager deployment.", result.GetObjectMeta().GetName(), ".")
}

func CreateYamlString(yamlContent string) error {
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

// todo remove this function and move to new ApplyResource function
func CreateOrUpdateYamlString(yamlContent string) error {
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
