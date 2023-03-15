package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	v1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateDeployment(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, isPaused bool, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Creating Deployment '%s'.", stage.K8sName), job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}

		if err != nil {
			logger.Log.Errorf("CreateDeployment ERROR: %s", err.Error())
		}

		deploymentClient := kubeProvider.ClientSet.AppsV1().Deployments(stage.K8sName)
		newDeployment := generateDeployment(stage, service, isPaused)

		createOptions := metav1.CreateOptions{
			FieldManager: DEPLOYMENTNAME,
		}

		_, err = deploymentClient.Create(context.TODO(), &newDeployment, createOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateDeployment ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Created deployment '%s'.", stage.K8sName), c)
		}

	}(cmd, wg)
	return cmd
}

func DeleteDeployment(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Deleting Deployment '%s'.", service.K8sName), job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}

		if err != nil {
			logger.Log.Errorf("DeleteDeployment ERROR: %s", err.Error())
		}

		deploymentClient := kubeProvider.ClientSet.AppsV1().Deployments(stage.K8sName)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: utils.Pointer[int64](5),
		}

		err = deploymentClient.Delete(context.TODO(), service.K8sName, deleteOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteDeployment ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Deleted Deployment '%s'.", service.K8sName), c)
		}

	}(cmd, wg)
	return cmd
}

func UpdateDeployment(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, isPaused bool, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Updating Deployment '%s'.", stage.K8sName), job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}

		if err != nil {
			logger.Log.Errorf("UpdatingDeployment ERROR: %s", err.Error())
		}

		deploymentClient := kubeProvider.ClientSet.AppsV1().Deployments(stage.K8sName)
		newDeployment := generateDeployment(stage, service, isPaused)

		updateOptions := metav1.UpdateOptions{
			FieldManager: DEPLOYMENTNAME,
		}

		_, err = deploymentClient.Update(context.TODO(), &newDeployment, updateOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("UpdatingDeployment ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Updating deployment '%s'.", stage.K8sName), c)
		}

	}(cmd, wg)
	return cmd
}

func StartDeployment(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Starting Deployment", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Starting Deployment '%s'.", service.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}

		if err != nil {
			logger.Log.Errorf("StartingDeployment ERROR: %s", err.Error())
		}

		serviceClient := kubeProvider.ClientSet.AppsV1().Deployments(stage.K8sName)
		deployment := generateDeployment(stage, service, false)

		_, err = serviceClient.Update(context.TODO(), &deployment, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("StartingDeployment ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Started Deployment '%s'.", service.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func StopDeployment(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Stopping Deployment", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Stopping Deployment '%s'.", service.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}

		if err != nil {
			logger.Log.Errorf("StopDeployment ERROR: %s", err.Error())
		}

		serviceClient := kubeProvider.ClientSet.AppsV1().Deployments(stage.K8sName)
		deployment := generateDeployment(stage, service, false)
		deployment.Spec.Replicas = utils.Pointer[int32](0)

		_, err = serviceClient.Update(context.TODO(), &deployment, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("StopDeployment ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Stopped Deployment '%s'.", service.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func RestartDeployment(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Restart Deployment", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Restarting Deployment '%s'.", service.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}

		if err != nil {
			logger.Log.Errorf("RestartDeployment ERROR: %s", err.Error())
		}

		serviceClient := kubeProvider.ClientSet.AppsV1().Deployments(stage.K8sName)
		deployment := generateDeployment(stage, service, false)
		// KUBERNETES ISSUES A "rollout restart deployment" WHENETHER THE METADATA IS CHANGED.
		if deployment.ObjectMeta.Annotations == nil {
			deployment.Spec.Template.ObjectMeta.Annotations = map[string]string{}
			deployment.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
		} else {
			deployment.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
		}

		_, err = serviceClient.Update(context.TODO(), &deployment, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("RestartDeployment ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Restart Deployment '%s'.", service.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func generateDeployment(stage dtos.K8sStageDto, service dtos.K8sServiceDto, isPaused bool) v1.Deployment {
	// SANITIZE
	if service.K8sSettings.LimitCpuCores <= 0 {
		service.K8sSettings.LimitCpuCores = 0.1
	}
	if service.K8sSettings.LimitMemoryMB <= 0 {
		service.K8sSettings.LimitMemoryMB = 16
	}
	if service.K8sSettings.EphemeralStorageMB <= 0 {
		service.K8sSettings.EphemeralStorageMB = 100
	}
	if service.K8sSettings.ReplicaCount < 0 {
		service.K8sSettings.ReplicaCount = 0
	}

	newDeployment := utils.InitDeployment()
	newDeployment.ObjectMeta.Name = service.K8sName
	newDeployment.ObjectMeta.Namespace = stage.K8sName
	newDeployment.Spec.Selector.MatchLabels["app"] = service.K8sName
	newDeployment.Spec.Selector.MatchLabels["ns"] = stage.K8sName
	newDeployment.Spec.Template.ObjectMeta.Labels["app"] = service.K8sName
	newDeployment.Spec.Template.ObjectMeta.Labels["ns"] = stage.K8sName

	// STRATEGY
	if service.K8sSettings.DeploymentStrategy != "" {
		if service.K8sSettings.DeploymentStrategy != "rolling" {
			newDeployment.Spec.Strategy.Type = v1.RollingUpdateDeploymentStrategyType
		} else {
			newDeployment.Spec.Strategy.Type = v1.RecreateDeploymentStrategyType
		}
	} else {
		newDeployment.Spec.Strategy.Type = v1.RecreateDeploymentStrategyType
	}

	// SWITCHED ON
	if service.SwitchedOn {
		newDeployment.Spec.Replicas = utils.Pointer(service.K8sSettings.ReplicaCount)
	} else {
		newDeployment.Spec.Replicas = utils.Pointer[int32](0)
	}

	// PAUSE
	newDeployment.Spec.Paused = isPaused

	// PORTS
	if len(service.Ports) > 0 {
		newDeployment.Spec.Template.Spec.Containers[0].Ports = []core.ContainerPort{}
		for _, port := range service.Ports {
			if port.Expose {
				newDeployment.Spec.Template.Spec.Containers[0].Ports = append(newDeployment.Spec.Template.Spec.Containers[0].Ports, core.ContainerPort{
					ContainerPort: int32(port.InternalPort),
				})
			}
		}
	} else {
		newDeployment.Spec.Template.Spec.Containers[0].Ports = nil
	}

	// RESOURCES
	limits := core.ResourceList{}
	limits["cpu"] = resource.MustParse(fmt.Sprintf("%.2fm", service.K8sSettings.LimitCpuCores*1000))
	limits["memory"] = resource.MustParse(fmt.Sprintf("%dMi", service.K8sSettings.LimitMemoryMB))
	limits["ephemeral-storage"] = resource.MustParse(fmt.Sprintf("%dMi", service.K8sSettings.EphemeralStorageMB))
	newDeployment.Spec.Template.Spec.Containers[0].Resources.Limits = limits
	requests := core.ResourceList{}
	requests["cpu"] = resource.MustParse("1m")
	requests["memory"] = resource.MustParse("1Mi")
	requests["ephemeral-storage"] = resource.MustParse(fmt.Sprintf("%dMi", service.K8sSettings.EphemeralStorageMB))
	newDeployment.Spec.Template.Spec.Containers[0].Resources.Requests = requests

	newDeployment.Spec.Template.Spec.Containers[0].Name = service.K8sName

	// IMAGE
	if service.App.Type != "CONTAINER_IMAGE" && service.App.Type != "CONTAINER_IMAGE_TEMPLATE" {
		newDeployment.Spec.Template.Spec.Containers[0].Image = fmt.Sprintf("%s/%s-%s:latest", utils.CONFIG.Kubernetes.DefaultContainerRegistry, service.K8sName, stage.K8sName)
	} else {
		newDeployment.Spec.Template.Spec.Containers[0].Image = service.ContainerImage
		if service.ContainerImageCommand != "" {
			newDeployment.Spec.Template.Spec.Containers[0].Command = utils.ParseJsonStringArray(service.ContainerImageCommand)
		}
		if service.ContainerImageCommandArgs != "" {
			newDeployment.Spec.Template.Spec.Containers[0].Args = utils.ParseJsonStringArray(service.ContainerImageCommandArgs)
		}
		if service.ContainerImageRepoSecretDecryptValue != "" {
			newDeployment.Spec.Template.Spec.ImagePullSecrets = []core.LocalObjectReference{}
			newDeployment.Spec.Template.Spec.ImagePullSecrets = append(newDeployment.Spec.Template.Spec.ImagePullSecrets, core.LocalObjectReference{
				Name: fmt.Sprintf("%s-container-secret", service.K8sName),
			})
		}
	}

	// ENV VARS
	newDeployment.Spec.Template.Spec.Containers[0].Env = []core.EnvVar{}
	for _, envVar := range service.EnvVars {
		if envVar.Type == "KEY_VAULT" ||
			envVar.Type == "PLAINTEXT" ||
			envVar.Type == "HOSTNAME" {
			newDeployment.Spec.Template.Spec.Containers[0].Env = append(newDeployment.Spec.Template.Spec.Containers[0].Env, core.EnvVar{
				Name: envVar.Name,
				ValueFrom: &core.EnvVarSource{
					SecretKeyRef: &core.SecretKeySelector{
						Key: envVar.Name,
						LocalObjectReference: core.LocalObjectReference{
							Name: service.K8sName,
						},
					},
				},
			})
		}
	}

	// IMAGE PULL SECRET
	if ContainerSecretDoesExistForStage(stage) {
		containerSecretName := "container-secret-" + stage.K8sName
		newDeployment.Spec.Template.Spec.ImagePullSecrets = []core.LocalObjectReference{}
		newDeployment.Spec.Template.Spec.ImagePullSecrets = append(newDeployment.Spec.Template.Spec.ImagePullSecrets, core.LocalObjectReference{Name: containerSecretName})
	}

	// SECURITY CONTEXT
	structs.StateDebugLog(fmt.Sprintf("securityContext of '%s' removed from deployment. BENE MUST SOLVE THIS!", service.K8sName))
	newDeployment.Spec.Template.Spec.Containers[0].SecurityContext = nil

	// VOLUME MOUNT
	newDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = []core.VolumeMount{}
	// XXX TODO -> TEMPORARY DISABLED
	// if stage.StorageSizeInMb > 0 {
	// 	for _, envVar := range service.EnvVars {
	// 		if envVar.Type == "VOLUME_MOUNT" {
	// 			components := strings.Split(envVar.Value, ":")
	// 			storageSubDir := components[0]
	// 			containerPath := components[1]
	// 			newDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(newDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, core.VolumeMount{
	// 				MountPath: containerPath,
	// 				SubPath:   storageSubDir,
	// 				Name:      stage.K8sName,
	// 			})
	// 		}
	// 	}
	// 	// ALWAYS MOUNT MO_DATA
	// 	newDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(newDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, core.VolumeMount{
	// 		MountPath: utils.CONFIG.Misc.DefaultMountPath,
	// 		Name:      stage.K8sName,
	// 	})
	// }

	// VOLUMES
	newDeployment.Spec.Template.Spec.Volumes = []core.Volume{}
	// XXX TODO -> TEMPORARY DISABLED
	// if stage.StorageSizeInMb > 0 {
	// 	newDeployment.Spec.Template.Spec.Volumes = append(newDeployment.Spec.Template.Spec.Volumes, core.Volume{
	// 		Name: stage.K8sName,
	// 		VolumeSource: core.VolumeSource{
	// 			PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
	// 				ClaimName: stage.K8sName,
	// 			},
	// 		},
	// 	})
	// }
	return newDeployment
}

func SetImage(job *structs.Job, namespace dtos.K8sNamespaceDto, stage dtos.K8sStageDto, service dtos.K8sServiceDto, imageName string, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Set Image", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Set Image in Deployment '%s'.", service.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}

		if err != nil {
			logger.Log.Errorf("SetImage ERROR: %s", err.Error())
		}

		deploymentClient := kubeProvider.ClientSet.AppsV1().Deployments(stage.K8sName)
		deploymentToUpdate, err := deploymentClient.Get(context.TODO(), service.K8sName, metav1.GetOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("SetImage ERROR: %s", err.Error()), c)
			return
		}

		// SET NEW IMAGE
		deploymentToUpdate.Spec.Template.Spec.Containers[0].Image = imageName

		_, err = deploymentClient.Update(context.TODO(), deploymentToUpdate, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("SetImage ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Set new image in Deployment '%s'.", service.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func AllDeployments(namespaceName string) []v1.Deployment {
	result := []v1.Deployment{}

	var provider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		provider, err = NewKubeProviderLocal()
	} else {
		provider, err = NewKubeProviderInCluster()
	}
	if err != nil {
		logger.Log.Errorf("AllDeployments ERROR: %s", err.Error())
	}

	deploymentList, err := provider.ClientSet.AppsV1().Deployments(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllDeployments ERROR: %s", err.Error())
	}

	for _, deployment := range deploymentList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, deployment.ObjectMeta.Namespace) {
			result = append(result, deployment)
		}
	}
	return result
}
