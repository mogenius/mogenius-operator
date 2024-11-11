package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"mogenius-k8s-manager/assert"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/shutdown"
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
		k8sLogger.Error("Error creating kubeprovider")
		shutdown.SendShutdownSignal(true)
		select {}
	}

	applyNamespace(provider)
	err = addRbac(provider)
	if err != nil {
		k8sLogger.Error("Error Creating RBAC. Aborting.", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	}
	addDeployment(provider)
	_, err = CreateOrUpdateClusterSecret(nil)
	if err != nil {
		k8sLogger.Error("Error Creating cluster secret. Aborting.", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
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
	_, err = provider.ClientSet.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, MoCreateOptions())
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	_, err = provider.ClientSet.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, MoCreateOptions())
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	k8sLogger.Info("Created mogenius-k8s-manager RBAC.")
	return nil
}

func applyNamespace(provider *punq.KubeProvider) {
	serviceClient := provider.ClientSet.CoreV1().Namespaces()

	namespace := applyconfcore.Namespace(config.Get("MO_OWN_NAMESPACE"))

	applyOptions := metav1.ApplyOptions{
		Force:        true,
		FieldManager: DEPLOYMENTNAME,
	}

	k8sLogger.Info("Creating mogenius-k8s-manager namespace ...")
	result, err := serviceClient.Apply(context.TODO(), namespace, applyOptions)
	if err != nil {
		k8sLogger.Error("Error applying mogenius-k8s-manager namespace.", "error", err)
	}
	k8sLogger.Info("Created mogenius-k8s-manager namespace", result.GetObjectMeta().GetName(), ".")
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
	}

	// check if essential data is changed
	if previousData.Repo != syncRepoReq.Repo ||
		syncRepoReq.Pat != "***" ||
		previousData.Branch != syncRepoReq.Branch ||
		previousData.AllowPull != syncRepoReq.AllowPull ||
		previousData.AllowPush != syncRepoReq.AllowPush {
		k8sLogger.Warn("⚠️ ⚠️ ⚠️  SyncRepoData has changed in a way that requires the deletion of current repo ...")
		IacManagerSetupInProcess.Store(true)
		defer IacManagerSetupInProcess.Store(false)
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
			IacManagerSetupInProcess.Store(false)
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
	IacManagerSetupInProcess.Store(true)
	err := IacManagerResetCurrentRepoData(IacManagerDeleteDataRetries)
	if err != nil {
		return err
	}
	IacManagerSetupInProcess.Store(false)
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
		k8sLogger.Error("Error creating kubeprovider")
		shutdown.SendShutdownSignal(true)
		select {}
	}

	secretClient := provider.ClientSet.CoreV1().Secrets(config.Get("MO_OWN_NAMESPACE"))

	existingSecret, getErr := secretClient.Get(context.TODO(), config.Get("MO_OWN_NAMESPACE"), metav1.GetOptions{})
	return writeMogeniusSecret(secretClient, existingSecret, getErr, syncRepoReq)
}

func CreateAndUpdateClusterConfigmap() (utils.ClusterConfigmap, error) {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		k8sLogger.Error("Error creating kubeprovider")
		shutdown.SendShutdownSignal(true)
		select {}
	}

	configmapClient := provider.ClientSet.CoreV1().ConfigMaps(config.Get("MO_OWN_NAMESPACE"))

	configMap, getErr := configmapClient.Get(context.TODO(), config.Get("MO_OWN_NAMESPACE"), metav1.GetOptions{})

	if getErr != nil {
		if apierrors.IsNotFound(getErr) {
			// create empty config map
			newConfigmap := core.ConfigMap{}
			newConfigmap.ObjectMeta.Name = config.Get("MO_OWN_NAMESPACE")
			newConfigmap.ObjectMeta.Namespace = config.Get("MO_OWN_NAMESPACE")
			newConfigmap.Data = make(map[string]string)
			newConfigmap.Data["syncWorkloads"] = ""
			newConfigmap.Data["availableWorkloads"] = ""
			newConfigmap.Data["ignoredNamespaces"] = ""
			newConfigmap.Data["ignoredNames"] = ""
			configMap, err = configmapClient.Create(context.TODO(), &newConfigmap, MoCreateOptions())
			if err != nil {
				k8sLogger.Error("failed to create mogenius configmap.", "error", err)
				return utils.ClusterConfigmap{}, err
			}
			k8sLogger.Debug("🗺️ Created new mogenius configmap.")
		} else {
			k8sLogger.Error("failed to get mogenius configmap.", "error", getErr)
			return utils.ClusterConfigmap{}, getErr
		}
	}
	assert.Assert(configMap != nil, "configMap cant be nil at this point")

	availableRes, err := GetAvailableResources()
	if err != nil {
		return utils.ClusterConfigmap{}, err
	}

	// CONSTRUCT THE OBJECT
	configMapData := utils.ClusterConfigmap{}
	configMapData.SyncWorkloads = availableRes
	configMapData.AvailableWorkloads = availableRes
	configMapData.IgnoredNamespaces = dtos.DefaultIgnoredNamespaces()
	configMapData.IgnoredNames = []string{""}

	// TODO: this field should not reflect ALL resources but only the resources we want to actually watch!
	syncWorkloadsYaml, err := utils.ToYaml(configMapData.SyncWorkloads)
	assert.Assert(err == nil, fmt.Sprintf("serializing the SyncWorkloads struct field should never fail: %#v", err))
	configMap.Data["syncWorkloads"] = syncWorkloadsYaml

	availableWorkloadsYaml, err := utils.ToYaml(configMapData.AvailableWorkloads)
	assert.Assert(err == nil, fmt.Sprintf("serializing the SyncWorkloads struct field should never fail: %#v", err))
	configMap.Data["availableWorkloads"] = availableWorkloadsYaml

	configMap.Data["ignoredNamespaces"] = strings.Join(configMapData.IgnoredNamespaces, ",")
	configMap.Data["ignoredNames"] = strings.Join(configMapData.IgnoredNames, ",")

	_, err = configmapClient.Update(context.TODO(), configMap, MoUpdateOptions())
	if err != nil {
		k8sLogger.Error("failed to update mogenius configmap.", "error", err)
		return utils.ClusterConfigmap{}, err
	}
	k8sLogger.Debug("🗺️ Updated mogenius configmap.")

	return configMapData, nil
}

func GetSyncRepoData() (*dtos.SyncRepoData, error) {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		k8sLogger.Error("Error creating kubeprovider")
		shutdown.SendShutdownSignal(true)
		select {}
	}

	secretClient := provider.ClientSet.CoreV1().Secrets(config.Get("MO_OWN_NAMESPACE"))

	existingSecret, getErr := secretClient.Get(context.TODO(), config.Get("MO_OWN_NAMESPACE"), metav1.GetOptions{})
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
		syncdata := dtos.CreateSyncRepoDataFrom(existingSecret)
		clusterSecret.SyncRepoUrl = syncdata.Repo
		clusterSecret.SyncRepoPat = syncdata.Pat
		clusterSecret.SyncRepoBranch = syncdata.Branch
		clusterSecret.SyncAllowPull = syncdata.AllowPull
		clusterSecret.SyncAllowPush = syncdata.AllowPush
		clusterSecret.SyncFrequencyInSec = syncdata.SyncFrequencyInSec
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
	}

	secret := punqUtils.InitSecret()
	secret.ObjectMeta.Name = config.Get("MO_OWN_NAMESPACE")
	secret.ObjectMeta.Namespace = config.Get("MO_OWN_NAMESPACE")
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
			string(existingSecret.Data["cluster-name"]) != clusterSecret.ClusterName ||
			string(existingSecret.Data["sync-repo-url"]) != clusterSecret.SyncRepoUrl ||
			string(existingSecret.Data["sync-repo-pat"]) != clusterSecret.SyncRepoPat ||
			string(existingSecret.Data["sync-repo-branch"]) != clusterSecret.SyncRepoBranch ||
			string(existingSecret.Data["sync-allow-pull"]) != fmt.Sprintf("%t", clusterSecret.SyncAllowPull) ||
			string(existingSecret.Data["sync-allow-push"]) != fmt.Sprintf("%t", clusterSecret.SyncAllowPush) ||
			string(existingSecret.Data["sync-frequency-in-sec"]) != fmt.Sprintf("%d", clusterSecret.SyncFrequencyInSec) {
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
}

func addDeployment(provider *punq.KubeProvider) {
	deploymentClient := provider.ClientSet.AppsV1().Deployments(config.Get("MO_OWN_NAMESPACE"))

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

	deployment := applyconfapp.Deployment(DEPLOYMENTNAME, config.Get("MO_OWN_NAMESPACE"))
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
