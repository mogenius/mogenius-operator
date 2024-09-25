package kubernetes

import (
	"fmt"
	"mogenius-k8s-manager/db"
	"mogenius-k8s-manager/dtos"

	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"strings"

	punqUtils "github.com/mogenius/punq/utils"
	v1 "k8s.io/api/apps/v1"
	v1job "k8s.io/api/batch/v1"
	v1core "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HasSpec interface {
	GetSelector() *metav1.LabelSelector
	GetTemplate() *v1core.PodTemplateSpec
	PreviousGetTemplate() *v1core.PodTemplateSpec
}

type SpecDeployment struct {
	Spec         *v1.DeploymentSpec
	PreviousSpec *v1.DeploymentSpec
}

type SpecCronJob struct {
	Spec         *v1job.CronJobSpec
	PreviousSpec *v1job.CronJobSpec
}

func (spec *SpecDeployment) GetSelector() *metav1.LabelSelector {
	return spec.Spec.Selector
}

func (spec *SpecDeployment) GetTemplate() *v1core.PodTemplateSpec {
	return &spec.Spec.Template
}

func (spec *SpecDeployment) PreviousGetTemplate() *v1core.PodTemplateSpec {
	if spec.PreviousSpec != nil {
		return &(*spec.PreviousSpec).Template
	}
	return nil
}

func (spec *SpecCronJob) GetSelector() *metav1.LabelSelector {
	return spec.Spec.JobTemplate.Spec.Selector
}

func (spec *SpecCronJob) GetTemplate() *v1core.PodTemplateSpec {
	return &spec.Spec.JobTemplate.Spec.Template
}

func (spec *SpecCronJob) PreviousGetTemplate() *v1core.PodTemplateSpec {
	if spec.PreviousSpec != nil {
		return &(*spec.PreviousSpec).JobTemplate.Spec.Template
	}
	return nil
}

type customControllerConfigHandler func(namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, freshlyCreated bool, client interface{}) (*metav1.ObjectMeta, HasSpec, interface{}, error)

func CreateControllerConfiguration(projectId string, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, freshlyCreated bool, client interface{}, handler customControllerConfigHandler) (interface{}, error) {
	objectMeta, hasSpec, controller, err := handler(namespace, service, freshlyCreated, client)
	if err != nil {
		return nil, err
	}
	if objectMeta == nil || controller == nil {
		return nil, fmt.Errorf("one of objectMeta, ctrl is nil")
	}

	objectMeta.Name = service.ControllerName
	objectMeta.Namespace = namespace.Name

	// LABELS bugfix: labels are diffrent in deployment and cronjob and cannot be generalized
	// specSelector := hasSpec.GetSelector()
	specTemplate := hasSpec.GetTemplate()
	previousSpecTemplate := hasSpec.PreviousGetTemplate()

	// specSelector.MatchLabels["app"] = service.Name
	// specSelector.MatchLabels["ns"] = namespace.Name
	specTemplate.ObjectMeta.Labels["mo-app"] = service.ControllerName
	specTemplate.ObjectMeta.Labels["mo-ns"] = namespace.Name
	specTemplate.ObjectMeta.Labels["mo-project-id"] = projectId

	for index, container := range service.Containers {
		// TODO REMOVE
		// PORTS
		//if len(container.Ports) > 0 {
		//	specTemplate.Spec.Containers[index].Ports = []v1core.ContainerPort{}
		//	for _, port := range container.Ports {
		//		if port.Expose {
		//			specTemplate.Spec.Containers[index].Ports = append(specTemplate.Spec.Containers[index].Ports, v1core.ContainerPort{
		//				ContainerPort: int32(port.InternalPort),
		//			})
		//		}
		//	}
		//} else {
		//	specTemplate.Spec.Containers[index].Ports = nil
		//}

		// RESOURCES
		if container.KubernetesLimits.IsLimitSetup() {
			limits := v1core.ResourceList{}
			requests := v1core.ResourceList{}
			if container.KubernetesLimits.LimitCpuCores > 0 {
				limits["cpu"] = resource.MustParse(fmt.Sprintf("%.2fm", container.KubernetesLimits.LimitCpuCores*1000))
				requests["cpu"] = resource.MustParse(fmt.Sprintf("%.2fm", container.KubernetesLimits.LimitCpuCores*200)) // 20% of limit
			}
			if container.KubernetesLimits.LimitMemoryMB > 0 {
				limits["memory"] = resource.MustParse(fmt.Sprintf("%dMi", container.KubernetesLimits.LimitMemoryMB))
				requests["memory"] = resource.MustParse(fmt.Sprintf("%dMi", int(float64(container.KubernetesLimits.LimitMemoryMB)*0.2))) // 20% of limit
			}
			if container.KubernetesLimits.EphemeralStorageMB > 0 {
				limits["ephemeral-storage"] = resource.MustParse(fmt.Sprintf("%dMi", container.KubernetesLimits.EphemeralStorageMB))
				requests["ephemeral-storage"] = resource.MustParse(fmt.Sprintf("%dMi", container.KubernetesLimits.EphemeralStorageMB))
			}
			specTemplate.Spec.Containers[index].Resources.Limits = limits
			specTemplate.Spec.Containers[index].Resources.Requests = requests
		} else {
			specTemplate.Spec.Containers[index].Resources.Limits = nil
			specTemplate.Spec.Containers[index].Resources.Requests = nil
		}

		specTemplate.Spec.Containers[index].Name = container.Name
		specTemplate.Spec.Containers[index].Command = []string{}
		specTemplate.Spec.Containers[index].Args = []string{}

		// IMAGE
		if container.Type == dtos.CONTAINER_CONTAINER_IMAGE {
			specTemplate.Spec.Containers[index].Image = *container.ContainerImage
			if container.ContainerImageCommand != nil {
				specTemplate.Spec.Containers[index].Command = punqUtils.ParseJsonStringArray(*container.ContainerImageCommand)
			}
			if container.ContainerImageCommandArgs != nil {
				specTemplate.Spec.Containers[index].Args = punqUtils.ParseJsonStringArray(*container.ContainerImageCommandArgs)
			}
			if container.ContainerImageRepoSecretDecryptValue != nil {
				specTemplate.Spec.ImagePullSecrets = []v1core.LocalObjectReference{}
				specTemplate.Spec.ImagePullSecrets = append(specTemplate.Spec.ImagePullSecrets, v1core.LocalObjectReference{
					Name: fmt.Sprintf("%s-%s", ContainerImagePullSecretName, service.ControllerName),
				})
			} else if ExistsClusterImagePullSecret(namespace.Name) {
				specTemplate.Spec.ImagePullSecrets = []v1core.LocalObjectReference{}
				specTemplate.Spec.ImagePullSecrets = append(specTemplate.Spec.ImagePullSecrets, v1core.LocalObjectReference{
					Name: fmt.Sprintf("%s-%s", ClusterImagePullSecretName, namespace.Name),
				})
			} else {
				specTemplate.Spec.ImagePullSecrets = []v1core.LocalObjectReference{}
			}
		} else {
			// this will be setup UNTIL the buildserver overwrites the image with the real one.
			if previousSpecTemplate != nil {
				specTemplate.Spec.Containers[index].Image = (*previousSpecTemplate).Spec.Containers[index].Image
			} else {
				specTemplate.Spec.Containers[index].Image = utils.IMAGE_PLACEHOLDER
			}

			// cluster image pull secret
			if ExistsClusterImagePullSecret(namespace.Name) {
				specTemplate.Spec.ImagePullSecrets = []v1core.LocalObjectReference{}
				specTemplate.Spec.ImagePullSecrets = append(specTemplate.Spec.ImagePullSecrets, v1core.LocalObjectReference{
					Name: fmt.Sprintf("%s-%s", ClusterImagePullSecretName, namespace.Name),
				})
			}

			if specTemplate.Spec.Containers[index].Image == utils.IMAGE_PLACEHOLDER {
				imgName := imageNameForContainer(namespace, service, container)
				if imgName != "" {
					specTemplate.Spec.Containers[index].Image = imgName
				} else {
					imgErr := fmt.Errorf("No image found for '%s/%s'. Maybe the build failed or is still running.", namespace.Name, container.Name)
					K8sLogger.Errorf(imgErr.Error())
					return nil, imgErr
				}
			}
		}

		// ENV VARS
		specTemplate.Spec.Containers[index].Env = []v1core.EnvVar{}
		specTemplate.Spec.Containers[index].VolumeMounts = []v1core.VolumeMount{}
		specTemplate.Spec.Volumes = []v1core.Volume{}

		for _, envVar := range container.EnvVars {
			if envVar.Type == dtos.EnvVarKeyVault && envVar.Data.VaultType == dtos.EnvVarVaultTypeMogeniusVault {
				//envVar.Type == "PLAINTEXT" ||
				//envVar.Type == "HOSTNAME" {
				specTemplate.Spec.Containers[index].Env = append(specTemplate.Spec.Containers[index].Env, v1core.EnvVar{
					Name: envVar.Name,
					ValueFrom: &v1core.EnvVarSource{
						SecretKeyRef: &v1core.SecretKeySelector{
							Key: envVar.Name,
							LocalObjectReference: v1core.LocalObjectReference{
								Name: service.ControllerName,
							},
						},
					},
				})
			}
			// EXTERNAL SECRETS OPERATOR
			if utils.CONFIG.Misc.ExternalSecretsEnabled {
				if envVar.Type == dtos.EnvVarKeyVault && envVar.Data.VaultType == dtos.EnvVarVaultTypeHashicorpExternalVault {
					// create secret
					namePrefix, propertyName := dtos.SplitEsoEnvVarValues(envVar)
					SecretName, err := CreateExternalSecret(CreateExternalSecretProps{
						namespace.Name,
						propertyName,
						namePrefix,
						service.ControllerName,
					})
					if err != nil {
						K8sLogger.Errorf("Error creating external secret: %s, Secret %s will not be set for service %s", err.Error(), envVar.Name, service.ControllerName)
					} else {
						// link created secret to container env
						specTemplate.Spec.Containers[index].Env = append(specTemplate.Spec.Containers[index].Env, v1core.EnvVar{
							Name: envVar.Name,
							ValueFrom: &v1core.EnvVarSource{
								SecretKeyRef: &v1core.SecretKeySelector{
									Key: propertyName,
									LocalObjectReference: v1core.LocalObjectReference{
										Name: SecretName,
									},
								},
							},
						})
					}
				}
			}
			if envVar.Type == dtos.EnvVarPlainText || envVar.Type == dtos.EnvVarHostname {
				specTemplate.Spec.Containers[index].Env = append(specTemplate.Spec.Containers[index].Env, v1core.EnvVar{
					Name:  envVar.Name,
					Value: envVar.Value,
				})
			}
			if envVar.Type == dtos.EnvVarVolumeMount {
				// VOLUMEMOUNT
				// EXAMPLE FOR value CONTENTS: VOLUME_NAME:/LOCATION_CONTAINER_DIR
				// components := strings.Split(envVar.Value, ":")
				// if len(components) == 3 {
				//volumeName := components[0]    // e.g. MY_COOL_NAME
				//srcPath := components[1]       // e.g. subpath/to/heaven
				//containerPath := components[2] // e.g. /mo-data
				if envVar.Data.VolumeName != "" && envVar.Data.VolumeSource != "" && envVar.Data.VolumeDestination != "" {
					volumeName := envVar.Data.VolumeName           // e.g. MY_COOL_NAME
					srcPath := envVar.Data.VolumeSource            // e.g. subpath/to/heaven
					containerPath := envVar.Data.VolumeDestination // e.g. /mo-data

					// subPath must be relative
					if strings.HasPrefix(srcPath, "/") {
						srcPath = strings.Replace(srcPath, "/", "", 1)
					}
					specTemplate.Spec.Containers[index].VolumeMounts = append(specTemplate.Spec.Containers[index].VolumeMounts, v1core.VolumeMount{
						MountPath: containerPath,
						SubPath:   srcPath,
						Name:      volumeName,
					})

					// VOLUME
					nfsService := ServiceForNfsVolume(namespace.Name, volumeName)
					if nfsService != nil {
						// VolumeName cannot be duplicated
						isUnique := true
						for _, v := range specTemplate.Spec.Volumes {
							if v.Name == volumeName {
								isUnique = false
							}
						}
						if isUnique {
							specTemplate.Spec.Volumes = append(specTemplate.Spec.Volumes, v1core.Volume{
								Name: volumeName,
								VolumeSource: v1core.VolumeSource{
									PersistentVolumeClaim: &v1core.PersistentVolumeClaimVolumeSource{
										ClaimName: volumeName,
									},
								},
							})
						}
					} else {
						K8sLogger.Errorf("No Volume found for  '%s/%s'!!!", namespace.Name, volumeName)
					}
				} else {
					K8sLogger.Errorf("SKIPPING ENVVAR '%s' because data is missing", envVar.Name)
				}
				//} else {
				//	K8sLogger.Errorf("SKIPPING ENVVAR '%s' because value '%s' must conform to pattern XXX:YYY:ZZZ", envVar.Type, envVar.Value)
				//}
			}
		}
	}

	// IMAGE PULL SECRET
	// the second check because otherwise we would overwrite the imagePullSecrets which is only defined for the service
	//if ContainerSecretDoesExistForStage(namespace) && len(specTemplate.Spec.ImagePullSecrets) <= 0 {
	//	containerSecretName := "container-secret-" + namespace.Name
	//	specTemplate.Spec.ImagePullSecrets = []v1core.LocalObjectReference{}
	//	specTemplate.Spec.ImagePullSecrets = append(specTemplate.Spec.ImagePullSecrets, v1core.LocalObjectReference{Name: containerSecretName})
	//}

	return controller, nil
}

func imageNameForContainer(namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, container dtos.K8sContainerDto) string {
	if container.Type == dtos.CONTAINER_GIT_REPOSITORY {
		lastBuild := db.GetLastBuildJobInfosFromDb(structs.BuildTaskRequest{Namespace: namespace.Name, Controller: service.ControllerName, Container: container.Name})
		if lastBuild.FinishTime != "" {
			return lastBuild.Image
		}
	}
	return ""
}
