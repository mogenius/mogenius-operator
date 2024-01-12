package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"strings"
	"sync"
	"time"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	v1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	v1depl "k8s.io/client-go/kubernetes/typed/apps/v1"
)

func CreateDeployment(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	logger.Log.Infof("CreateDeployment K8sServiceDto: %s", service)

	cmd := structs.CreateCommand(fmt.Sprintf("Creating Deployment '%s'.", namespace.Name), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating Deployment '%s'.", namespace.Name))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespace.Name)
		newController, err := GenerateController(namespace, service, false, deploymentClient, generateHandler)
		if  err != nil {
			logger.Log.Errorf("error: %s", err.Error())
		}
		
		// deployment := generateDeployment(namespace, service, false, deploymentClient)
		deployment := newController.(*v1.Deployment)

		deployment.Labels = MoUpdateLabels(&deployment.Labels, job.ProjectId, &namespace, &service)

		_, err = deploymentClient.Create(context.TODO(), deployment, MoCreateOptions())
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Created deployment '%s'.", namespace.Name))
		}

	}(cmd, wg)
	return cmd
}

func DeleteDeployment(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Deleting Deployment '%s'.", service.Name), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting Deployment '%s'.", service.Name))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: punqUtils.Pointer[int64](5),
		}

		err = deploymentClient.Delete(context.TODO(), service.Name, deleteOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted Deployment '%s'.", service.Name))
		}

	}(cmd, wg)
	return cmd
}

func UpdateDeployment(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Updating Deployment '%s'.", namespace.Name), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Updating Deployment '%s'.", namespace.Name))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespace.Name)
		newController, err := GenerateController(namespace, service, false, deploymentClient, generateHandler)
		if  err != nil {
			logger.Log.Errorf("error: %s", err.Error())
		}
		
		// deployment := generateDeployment(namespace, service, false, deploymentClient)
		deployment := newController.(*v1.Deployment)

		updateOptions := metav1.UpdateOptions{
			FieldManager: DEPLOYMENTNAME,
		}

		_, err = deploymentClient.Update(context.TODO(), deployment, updateOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("UpdatingDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Updating deployment '%s'.", namespace.Name))
		}

	}(cmd, wg)
	return cmd
}

func StartDeployment(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Starting Deployment", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Starting Deployment '%s'.", service.Name))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespace.Name)

		newController, err := GenerateController(namespace, service, false, deploymentClient, generateHandler)
		if  err != nil {
			logger.Log.Errorf("error: %s", err.Error())
		}
		
		// deployment := generateDeployment(namespace, service, false, deploymentClient)
		deployment := newController.(*v1.Deployment)

		_, err = deploymentClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("StartingDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Started Deployment '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}

func StopDeployment(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Stopping Deployment", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Stopping Deployment '%s'.", service.Name))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespace.Name)
		newController, err := GenerateController(namespace, service, false, deploymentClient, generateHandler)
		if  err != nil {
			logger.Log.Errorf("error: %s", err.Error())
		}
		
		// deployment := generateDeployment(namespace, service, false, deploymentClient)
		deployment := newController.(*v1.Deployment)

		deployment.Spec.Replicas = punqUtils.Pointer[int32](0)

		_, err = deploymentClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("StopDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Stopped Deployment '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}

func RestartDeployment(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Restart Deployment", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Restarting Deployment '%s'.", service.Name))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespace.Name)
		
		newController, err := GenerateController(namespace, service, false, deploymentClient, generateHandler)
		if  err != nil {
			logger.Log.Errorf("error: %s", err.Error())
		}
		
		// deployment := generateDeployment(namespace, service, false, deploymentClient)
		deployment := newController.(*v1.Deployment)

		// KUBERNETES ISSUES A "rollout restart deployment" WHENETHER THE METADATA IS CHANGED.
		if deployment.ObjectMeta.Annotations == nil {
			deployment.Spec.Template.ObjectMeta.Annotations = map[string]string{}
			deployment.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
		} else {
			deployment.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
		}

		_, err = deploymentClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("RestartDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Restart Deployment '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}

func generateHandler(namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, freshlyCreated bool, client interface{}) (*metav1.ObjectMeta, HasSpec, interface{}, error) {
	previousDeployment, err := client.(v1depl.DeploymentInterface).Get(context.TODO(), service.Name, metav1.GetOptions{})
	if err != nil {
		previousDeployment = nil
	}

	newDeployment := punqUtils.InitDeployment()

	// check if default deployment exists
	defaultDeployment := GetCustomDeploymentTemplate()
	if previousDeployment == nil && defaultDeployment != nil {
		newDeployment = *defaultDeployment
	}

	objectMeta := &newDeployment.ObjectMeta
	spec := &newDeployment.Spec

	// STRATEGY
	if service.K8sSettings.DeploymentStrategy != "" {
		if service.K8sSettings.DeploymentStrategy == "rolling" {
			spec.Strategy.Type = v1.RollingUpdateDeploymentStrategyType
		} else if service.K8sSettings.DeploymentStrategy == "recreate" {
			spec.Strategy.Type = v1.RecreateDeploymentStrategyType
		} else {
			spec.Strategy.Type = v1.RecreateDeploymentStrategyType
		}
	} else {
		spec.Strategy.Type = v1.RecreateDeploymentStrategyType
	}

	// SWITCHED ON
	if service.SwitchedOn {
		spec.Replicas = punqUtils.Pointer(service.K8sSettings.ReplicaCount)
	} else {
		spec.Replicas = punqUtils.Pointer[int32](0)
	}

	// PAUSE
	if freshlyCreated && service.ServiceType == dtos.CONTAINER_IMAGE_TEMPLATE {
		spec.Paused = true
	} else {
		spec.Paused = false
	}

	// ImagePullPolicy
	if service.K8sSettings.ImagePullPolicy != "" {
		spec.Template.Spec.Containers[0].ImagePullPolicy = core.PullPolicy(service.K8sSettings.ImagePullPolicy)
	} else {
		spec.Template.Spec.Containers[0].ImagePullPolicy = core.PullAlways
	}

	// PORTS
	var internalHttpPort *int
	if len(service.Ports) > 0 {
		for _, port := range service.Ports {
			if port.PortType == "HTTPS" {
				tmp := int(port.InternalPort)
				internalHttpPort = &tmp
			}
		}
	}

	// PROBES
	if !service.K8sSettings.ProbesOn {
		spec.Template.Spec.Containers[0].StartupProbe = nil
		spec.Template.Spec.Containers[0].LivenessProbe = nil
		spec.Template.Spec.Containers[0].ReadinessProbe = nil
	} else if internalHttpPort != nil {
		spec.Template.Spec.Containers[0].StartupProbe.HTTPGet.Port = intstr.FromInt(*internalHttpPort)
		spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet.Port = intstr.FromInt(*internalHttpPort)
		spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Port = intstr.FromInt(*internalHttpPort)
	}

	return objectMeta, &SpecDeployment{*spec}, &newDeployment, nil
}

// func generateDeployment(namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, freshlyCreated bool, deploymentclient v1depl.DeploymentInterface) v1.Deployment {
// 	previousDeployment, err := deploymentclient.Get(context.TODO(), service.Name, metav1.GetOptions{})
// 	if err != nil {
// 		//logger.Log.Infof("No previous deployment found for %s/%s.", namespace.Name, service.Name)
// 		previousDeployment = nil
// 	}

// 	newDeployment := punqUtils.InitDeployment()

// 	// check if default deployment exists
// 	defaultDeployment := GetCustomDeploymentTemplate()
// 	if previousDeployment == nil && defaultDeployment != nil {
// 		newDeployment = *defaultDeployment
// 	}

// 	newDeployment.ObjectMeta.Name = service.Name
// 	newDeployment.ObjectMeta.Namespace = namespace.Name
// 	if newDeployment.Spec.Selector == nil {
// 		newDeployment.Spec.Selector = &metav1.LabelSelector{}
// 	}
// 	if newDeployment.Spec.Selector.MatchLabels == nil {
// 		newDeployment.Spec.Selector.MatchLabels = map[string]string{}
// 	}
// 	newDeployment.Spec.Selector.MatchLabels["app"] = service.Name
// 	newDeployment.Spec.Selector.MatchLabels["ns"] = namespace.Name
// 	if newDeployment.Spec.Template.ObjectMeta.Labels == nil {
// 		newDeployment.Spec.Template.ObjectMeta.Labels = map[string]string{}
// 	}
// 	newDeployment.Spec.Template.ObjectMeta.Labels["app"] = service.Name
// 	newDeployment.Spec.Template.ObjectMeta.Labels["ns"] = namespace.Name

// 	if len(newDeployment.Spec.Template.Spec.Containers) == 0 {
// 		newDeployment.Spec.Template.Spec.Containers = []core.Container{}
// 		newDeployment.Spec.Template.Spec.Containers = append(newDeployment.Spec.Template.Spec.Containers, core.Container{})
// 	}

// 	// STRATEGY
// 	if service.K8sSettings.DeploymentStrategy != "" {
// 		if service.K8sSettings.DeploymentStrategy == "rolling" {
// 			newDeployment.Spec.Strategy.Type = v1.RollingUpdateDeploymentStrategyType
// 		} else if service.K8sSettings.DeploymentStrategy == "recreate" {
// 			newDeployment.Spec.Strategy.Type = v1.RecreateDeploymentStrategyType
// 		} else {
// 			newDeployment.Spec.Strategy.Type = v1.RecreateDeploymentStrategyType
// 		}
// 	} else {
// 		newDeployment.Spec.Strategy.Type = v1.RecreateDeploymentStrategyType
// 	}

// 	// SWITCHED ON
// 	if service.SwitchedOn {
// 		newDeployment.Spec.Replicas = punqUtils.Pointer(service.K8sSettings.ReplicaCount)
// 	} else {
// 		newDeployment.Spec.Replicas = punqUtils.Pointer[int32](0)
// 	}

// 	// PAUSE
// 	if freshlyCreated && service.ServiceType == dtos.CONTAINER_IMAGE_TEMPLATE {
// 		newDeployment.Spec.Paused = true
// 	} else {
// 		newDeployment.Spec.Paused = false
// 	}

// 	// PORTS
// 	var internalHttpPort *int
// 	if len(service.Ports) > 0 {
// 		newDeployment.Spec.Template.Spec.Containers[0].Ports = []core.ContainerPort{}
// 		for _, port := range service.Ports {
// 			if port.Expose {
// 				newDeployment.Spec.Template.Spec.Containers[0].Ports = append(newDeployment.Spec.Template.Spec.Containers[0].Ports, core.ContainerPort{
// 					ContainerPort: int32(port.InternalPort),
// 				})
// 			}
// 			if port.PortType == "HTTPS" {
// 				tmp := int(port.InternalPort)
// 				internalHttpPort = &tmp
// 			}
// 		}
// 	} else {
// 		newDeployment.Spec.Template.Spec.Containers[0].Ports = nil
// 	}

// 	// RESOURCES
// 	if service.K8sSettings.IsLimitSetup() {
// 		limits := core.ResourceList{}
// 		requests := core.ResourceList{}
// 		limits["cpu"] = resource.MustParse(fmt.Sprintf("%.2fm", service.K8sSettings.LimitCpuCores*1000))
// 		limits["memory"] = resource.MustParse(fmt.Sprintf("%dMi", service.K8sSettings.LimitMemoryMB))
// 		limits["ephemeral-storage"] = resource.MustParse(fmt.Sprintf("%dMi", service.K8sSettings.EphemeralStorageMB))
// 		requests["cpu"] = resource.MustParse(fmt.Sprintf("%.2fm", service.K8sSettings.LimitCpuCores*200))                 // 20% of limit
// 		requests["memory"] = resource.MustParse(fmt.Sprintf("%dMi", int(float64(service.K8sSettings.LimitMemoryMB)*0.2))) // 20% of limit
// 		requests["ephemeral-storage"] = resource.MustParse(fmt.Sprintf("%dMi", service.K8sSettings.EphemeralStorageMB))
// 		newDeployment.Spec.Template.Spec.Containers[0].Resources.Limits = limits
// 		newDeployment.Spec.Template.Spec.Containers[0].Resources.Requests = requests
// 	} else {
// 		newDeployment.Spec.Template.Spec.Containers[0].Resources.Limits = nil
// 		newDeployment.Spec.Template.Spec.Containers[0].Resources.Requests = nil
// 	}

// 	newDeployment.Spec.Template.Spec.Containers[0].Name = service.Name

// 	// IMAGE
// 	if service.ContainerImage != "" {
// 		newDeployment.Spec.Template.Spec.Containers[0].Image = service.ContainerImage
// 		if service.ContainerImageCommand != "" {
// 			newDeployment.Spec.Template.Spec.Containers[0].Command = punqUtils.ParseJsonStringArray(service.ContainerImageCommand)
// 		}
// 		if service.ContainerImageCommandArgs != "" {
// 			newDeployment.Spec.Template.Spec.Containers[0].Args = punqUtils.ParseJsonStringArray(service.ContainerImageCommandArgs)
// 		}
// 		if service.ContainerImageRepoSecretDecryptValue != "" {
// 			newDeployment.Spec.Template.Spec.ImagePullSecrets = []core.LocalObjectReference{}
// 			newDeployment.Spec.Template.Spec.ImagePullSecrets = append(newDeployment.Spec.Template.Spec.ImagePullSecrets, core.LocalObjectReference{
// 				Name: fmt.Sprintf("container-secret-service-%s", service.Name),
// 			})
// 		}
// 	} else {
// 		// this will be setup UNTIL the buildserver overwrites the image with the real one.
// 		if previousDeployment != nil {
// 			newDeployment.Spec.Template.Spec.Containers[0].Image = previousDeployment.Spec.Template.Spec.Containers[0].Image
// 		} else {
// 			newDeployment.Spec.Template.Spec.Containers[0].Image = "ghcr.io/mogenius/mo-default-backend:latest"
// 		}
// 	}

// 	// ImagePullPolicy
// 	if service.K8sSettings.ImagePullPolicy != "" {
// 		newDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = core.PullPolicy(service.K8sSettings.ImagePullPolicy)
// 	} else {
// 		newDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = core.PullAlways
// 	}

// 	// ENV VARS
// 	newDeployment.Spec.Template.Spec.Containers[0].Env = []core.EnvVar{}
// 	newDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = []core.VolumeMount{}
// 	newDeployment.Spec.Template.Spec.Volumes = []core.Volume{}
// 	for _, envVar := range service.EnvVars {
// 		if envVar.Type == "KEY_VAULT" ||
// 			envVar.Type == "PLAINTEXT" ||
// 			envVar.Type == "HOSTNAME" {
// 			newDeployment.Spec.Template.Spec.Containers[0].Env = append(newDeployment.Spec.Template.Spec.Containers[0].Env, core.EnvVar{
// 				Name: envVar.Name,
// 				ValueFrom: &core.EnvVarSource{
// 					SecretKeyRef: &core.SecretKeySelector{
// 						Key: envVar.Name,
// 						LocalObjectReference: core.LocalObjectReference{
// 							Name: service.Name,
// 						},
// 					},
// 				},
// 			})
// 		}
// 		if envVar.Type == "VOLUME_MOUNT" {
// 			// VOLUMEMOUNT
// 			// EXAMPLE FOR value CONTENTS: VOLUME_NAME:/LOCATION_CONTAINER_DIR
// 			components := strings.Split(envVar.Value, ":")
// 			if len(components) == 3 {
// 				volumeName := components[0]    // e.g. MY_COOL_NAME
// 				srcPath := components[1]       // e.g. subpath/to/heaven
// 				containerPath := components[2] // e.g. /mo-data

// 				// subPath must be relative
// 				if strings.HasPrefix(srcPath, "/") {
// 					srcPath = strings.Replace(srcPath, "/", "", 1)
// 				}
// 				newDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(newDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, core.VolumeMount{
// 					MountPath: containerPath,
// 					SubPath:   srcPath,
// 					Name:      volumeName,
// 				})

// 				// VOLUME
// 				nfsService := ServiceForNfsVolume(namespace.Name, volumeName)
// 				if nfsService != nil {
// 					// VolumeName cannot be duplicated
// 					isUnique := true
// 					for _, v := range newDeployment.Spec.Template.Spec.Volumes {
// 						if v.Name == volumeName {
// 							isUnique = false
// 						}
// 					}
// 					if isUnique {
// 						newDeployment.Spec.Template.Spec.Volumes = append(newDeployment.Spec.Template.Spec.Volumes, core.Volume{
// 							Name: volumeName,
// 							VolumeSource: core.VolumeSource{
// 								PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
// 									ClaimName: volumeName,
// 								},
// 							},
// 						})
// 					}
// 				} else {
// 					logger.Log.Errorf("No Volume found for  '%s/%s'!!!", namespace.Name, volumeName)
// 				}
// 			} else {
// 				logger.Log.Errorf("SKIPPING ENVVAR '%s' because value '%s' must conform to pattern XXX:YYY:ZZZ", envVar.Type, envVar.Value)
// 			}
// 		}
// 	}

// 	// IMAGE PULL SECRET
// 	if ContainerSecretDoesExistForStage(namespace) && service.ContainerImageRepoSecretDecryptValue == "" {
// 		containerSecretName := "container-secret-" + namespace.Name
// 		newDeployment.Spec.Template.Spec.ImagePullSecrets = []core.LocalObjectReference{}
// 		newDeployment.Spec.Template.Spec.ImagePullSecrets = append(newDeployment.Spec.Template.Spec.ImagePullSecrets, core.LocalObjectReference{Name: containerSecretName})
// 	}

// 	// PROBES OFF
// 	if !service.K8sSettings.ProbesOn {
// 		newDeployment.Spec.Template.Spec.Containers[0].StartupProbe = nil
// 		newDeployment.Spec.Template.Spec.Containers[0].LivenessProbe = nil
// 		newDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe = nil
// 	} else if internalHttpPort != nil {
// 		newDeployment.Spec.Template.Spec.Containers[0].StartupProbe.HTTPGet.Port = intstr.FromInt(*internalHttpPort)
// 		newDeployment.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet.Port = intstr.FromInt(*internalHttpPort)
// 		newDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Port = intstr.FromInt(*internalHttpPort)
// 	}

// 	// SECURITY CONTEXT
// 	// TODO wieder in betrieb nehmen
// 	//structs.StateDebugLog(fmt.Sprintf("securityContext of '%s' removed from deployment. BENE MUST SOLVE THIS!", service.K8sName))
// 	newDeployment.Spec.Template.Spec.Containers[0].SecurityContext = nil

// 	return newDeployment
// }

func SetDeploymentImage(job *structs.Job, namespaceName string, serviceName string, imageName string, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Set Image '%s'", imageName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Set Image in Deployment '%s'.", serviceName))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespaceName)
		deploymentToUpdate, err := deploymentClient.Get(context.TODO(), serviceName, metav1.GetOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("SetImage ERROR: %s", err.Error()))
			return
		}

		// SET NEW IMAGE
		deploymentToUpdate.Spec.Template.Spec.Containers[0].Image = imageName
		deploymentToUpdate.Spec.Paused = false

		_, err = deploymentClient.Update(context.TODO(), deploymentToUpdate, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("SetImage ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Set new image in Deployment '%s'.", serviceName))
		}
	}(cmd, wg)
	return cmd
}

func UpdateDeploymentImage(namespaceName string, serviceName string, imageName string) error {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return err
	}
	deploymentClient := provider.ClientSet.AppsV1().Deployments(namespaceName)
	deploymentToUpdate, err := deploymentClient.Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// SET NEW IMAGE
	deploymentToUpdate.Spec.Template.Spec.Containers[0].Image = imageName
	deploymentToUpdate.Spec.Paused = false

	_, err = deploymentClient.Update(context.TODO(), deploymentToUpdate, metav1.UpdateOptions{})
	return err
}

func GetDeploymentImage(namespaceName string, serviceName string) (string, error) {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return "", err
	}
	deploymentClient := provider.ClientSet.AppsV1().Deployments(namespaceName)
	deploymentToUpdate, err := deploymentClient.Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return deploymentToUpdate.Spec.Template.Spec.Containers[0].Image, nil
}

func ListDeploymentsWithFieldSelector(namespace string, labelSelector string, prefix string) K8sWorkloadResult {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return WorkloadResult(nil, err)
	}
	client := provider.ClientSet.AppsV1().Deployments(namespace)

	deployments, err := client.List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return WorkloadResult(nil, err)
	}

	// delete all deployments that do not start with prefix
	if prefix != "" {
		for i := len(deployments.Items) - 1; i >= 0; i-- {
			if !strings.HasPrefix(deployments.Items[i].Name, prefix) {
				deployments.Items = append(deployments.Items[:i], deployments.Items[i+1:]...)
			}
		}
	}

	return WorkloadResult(deployments.Items, err)
}

func GetDeployment(namespace string, name string) K8sWorkloadResult {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return WorkloadResult(nil, err)
	}
	client := provider.ClientSet.AppsV1().Deployments(namespace)

	deployment, err := client.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(deployment, err)
}
