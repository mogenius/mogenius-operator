package kubernetes

import (
	"context"
	"os"

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
	applyconfapp "k8s.io/client-go/applyconfigurations/apps/v1"
	applyconfcore "k8s.io/client-go/applyconfigurations/core/v1"
	applyconfmeta "k8s.io/client-go/applyconfigurations/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func Deploy() {
	provider := punq.NewKubeProvider()
	if provider == nil {
		panic("Error creating kubeprovider")
	}

	applyNamespace(provider)
	addRbac(provider)
	addDeployment(provider)
	_, err := CreateClusterSecretIfNotExist(false)
	if err != nil {
		logger.Log.Fatalf("Error Creating cluster secret. Aborting: %s.", err.Error())
	}
}

func addRbac(kubeProvider *punq.KubeProvider) error {
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

func applyNamespace(kubeProvider *punq.KubeProvider) {
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

// func CreateNfsServiceIfNotExist(runsInCluster bool) (string, error) {
// 	var kubeProvider *KubeProvider
// 	var err error
// 	if runsInCluster {
// 		kubeProvider, err = NewKubeProviderInCluster()
// 	} else {
// 		kubeProvider, err = NewKubeProviderLocal()
// 	}

// 	if err != nil {
// 		logger.Log.Errorf("CreateNfsServiceIfNotExist ERROR: %s", err.Error())
// 	}

// 	serviceClient := kubeProvider.ClientSet.CoreV1().Services(NAMESPACE)

// 	// GET NFS SERVICE
// 	nfsServerService, getErr := serviceClient.Get(context.TODO(), utils.K8SNFS_SERVICE_NAME, metav1.GetOptions{})
// 	if getErr != nil && !apierrors.IsNotFound(getErr) {
// 		return "", fmt.Errorf("CreateNfsServiceIfNotExist ERROR: %s", err)
// 	}

// 	// SERVICE EXISTS -> GET CLUSTERIP
// 	if nfsServerService != nil && getErr == nil {
// 		if nfsServerService.Spec.ClusterIP != "" {
// 			return nfsServerService.Spec.ClusterIP, nil
// 		}
// 	}
// 	// // DETERMINE IP RANGE OF THE CLUSTER
// 	// allServicesClient := kubeProvider.ClientSet.CoreV1().Services("")
// 	// services, err := allServicesClient.List(context.TODO(), metav1.ListOptions{})
// 	// if err != nil {
// 	// 	return fmt.Errorf("CreateNfsServiceIfNotExist (IP) ERROR: %s", err.Error())
// 	// }
// 	// ips := []string{}
// 	// for _, service := range services.Items {
// 	// 	ips = append(ips, service.Spec.ClusterIP)
// 	// }
// 	// commonSubnet := utils.FindSmallestSubnet(ips)
// 	// if commonSubnet != nil {
// 	// 	logger.Log.Infof("🕸️ Cluster Common Service Subnet: %s\n", commonSubnet.String())
// 	// 	logger.Log.Infof("Last possible IP for NFS-Service -> %s\n", utils.LastIpMinusOne(commonSubnet))
// 	// }

// 	// NOT FOUND CREATE IT
// 	if apierrors.IsNotFound(getErr) {
// 		service := utils.InitMogeniusNfsK8sService()
// 		createdService, createError := serviceClient.Create(context.TODO(), &service, metav1.CreateOptions{})
// 		if createError != nil {
// 			return "", fmt.Errorf("CreateNfsServiceIfNotExist (SERVICE_CREATE) ERROR: %s", createError.Error())
// 		}
// 		if createdService != nil {
// 			return createdService.Spec.ClusterIP, nil
// 		}
// 	}

// 	return "", fmt.Errorf("IP COULD NOT BE DETERMINED")
// }

// func CheckIfDeploymentUpdateIsRequiredForNfs(nfsServerIp string, runsInCluster bool) {
// 	updateRequired := false

// 	var kubeProvider *KubeProvider
// 	var err error
// 	if runsInCluster {
// 		kubeProvider, err = NewKubeProviderInCluster()
// 	} else {
// 		kubeProvider, err = NewKubeProviderLocal()
// 	}

// 	if err != nil {
// 		logger.Log.Errorf("checkIfDeploymentUpdateIsRequiredForNfs ERROR: %s", err.Error())
// 	}

// 	deploymentClient := kubeProvider.ClientSet.AppsV1().Deployments(NAMESPACE)

// 	k8sMgrDeployment, getErr := deploymentClient.Get(context.TODO(), DEPLOYMENTNAME, metav1.GetOptions{})
// 	if getErr != nil {
// 		logger.Log.Errorf("checkIfDeploymentUpdateIsRequiredForNfs (SERVICE_GET) ERROR: %s", getErr.Error())
// 		return
// 	}

// 	if len(k8sMgrDeployment.Spec.Template.Spec.Volumes) >= 1 {
// 		if k8sMgrDeployment.Spec.Template.Spec.Volumes[0].NFS.Server != nfsServerIp {
// 			updateRequired = true
// 		}
// 	} else {
// 		updateRequired = true
// 	}

// 	if updateRequired {
// 		logger.Log.Notice("k8s-manager will update itself (volume/volume_mount) to contact the nfs-server.")
// 		k8sMgrDeployment.Spec.Template.Spec.Volumes = []core.Volume{}
// 		k8sMgrDeployment.Spec.Template.Spec.Volumes = append(k8sMgrDeployment.Spec.Template.Spec.Volumes, core.Volume{
// 			Name: "nfs",
// 			VolumeSource: core.VolumeSource{
// 				NFS: &core.NFSVolumeSource{
// 					Path:   "/exports",
// 					Server: nfsServerIp,
// 				},
// 			},
// 		})
// 		k8sMgrDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = []core.VolumeMount{}
// 		k8sMgrDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(k8sMgrDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, core.VolumeMount{
// 			Name:      "nfs",
// 			MountPath: utils.CONFIG.Misc.DefaultMountPath,
// 		})
// 		_, updateErr := deploymentClient.Update(context.TODO(), k8sMgrDeployment, metav1.UpdateOptions{})
// 		if updateErr != nil {
// 			logger.Log.Errorf("checkIfDeploymentUpdateIsRequiredForNfs (UPDATE_K8S_DEPL) ERROR: %s", getErr.Error())
// 			return
// 		}
// 		return
// 	}
// 	logger.Log.Info("k8s-manager (volume/volume_mount) are configured correctly.")
// }

func CreateClusterSecretIfNotExist(runsInCluster bool) (utils.ClusterSecret, error) {
	var kubeProvider *punq.KubeProvider = punq.NewKubeProvider()
	if kubeProvider == nil {
		logger.Log.Fatal("Error creating kubeprovider")
	}

	secretClient := kubeProvider.ClientSet.CoreV1().Secrets(NAMESPACE)

	existingSecret, getErr := secretClient.Get(context.TODO(), NAMESPACE, metav1.GetOptions{})
	return writeMogeniusSecret(secretClient, runsInCluster, existingSecret, getErr)
}

func writeMogeniusSecret(secretClient v1.SecretInterface, runsInCluster bool, existingSecret *core.Secret, getErr error) (utils.ClusterSecret, error) {
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

func addDeployment(kubeProvider *punq.KubeProvider) {
	deploymentClient := kubeProvider.ClientSet.AppsV1().Deployments(NAMESPACE)

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
