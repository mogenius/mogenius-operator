package kubernetes

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"strings"

	punqUtils "github.com/mogenius/punq/utils"
	v1 "k8s.io/api/apps/v1"
	v1job "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	v1core "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HasSpec interface {
	GetSelector() *metav1.LabelSelector
	GetTemplate() v1core.PodTemplateSpec
}

type SpecDeployment struct {
	Spec v1.DeploymentSpec
}

type SpecCronJob struct {
	Spec v1job.CronJobSpec
}

func (spec SpecDeployment) GetSelector() *metav1.LabelSelector {
	return spec.Spec.Selector
}

func (spec SpecDeployment) GetTemplate() v1core.PodTemplateSpec {
	return spec.Spec.Template
}

func (spec SpecCronJob) GetSelector() *metav1.LabelSelector {
	return spec.Spec.JobTemplate.Spec.Selector
}

func (spec SpecCronJob) GetTemplate() v1core.PodTemplateSpec {
	return spec.Spec.JobTemplate.Spec.Template
}

type customControllerConfigHandler func(namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, freshlyCreated bool, client interface{}) (*metav1.ObjectMeta, HasSpec, interface{}, error)

func CreateControllerConfiguration(namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, freshlyCreated bool, client interface{}, handler customControllerConfigHandler) (interface{}, error) {
	objectMeta, hasSpec, controller, err := handler(namespace, service, freshlyCreated, client)
	if err != nil {
		return nil, err
	}
	if objectMeta == nil || controller == nil {
		return nil, fmt.Errorf("one of objectMeta, ctrl is nil")
	}

	objectMeta.Name = service.Name
	objectMeta.Namespace = namespace.Name

	specSelector := hasSpec.GetSelector()
	specTemplate := hasSpec.GetTemplate()

	if specSelector == nil {
		specSelector = &metav1.LabelSelector{}
	}
	if specSelector.MatchLabels == nil {
		specSelector.MatchLabels = map[string]string{}
	}
	specSelector.MatchLabels["app"] = service.Name
	specSelector.MatchLabels["ns"] = namespace.Name
	if specTemplate.ObjectMeta.Labels == nil {
		specTemplate.ObjectMeta.Labels = map[string]string{}
	}
	specTemplate.ObjectMeta.Labels["app"] = service.Name
	specTemplate.ObjectMeta.Labels["ns"] = namespace.Name

	if len(specTemplate.Spec.Containers) == 0 {
		specTemplate.Spec.Containers = []core.Container{}
		specTemplate.Spec.Containers = append(specTemplate.Spec.Containers, core.Container{})
	}

	// PORTS
	if len(service.Ports) > 0 {
		specTemplate.Spec.Containers[0].Ports = []core.ContainerPort{}
		for _, port := range service.Ports {
			if port.Expose {
				specTemplate.Spec.Containers[0].Ports = append(specTemplate.Spec.Containers[0].Ports, core.ContainerPort{
					ContainerPort: int32(port.InternalPort),
				})
			}
		}
	} else {
		specTemplate.Spec.Containers[0].Ports = nil
	}

	// RESOURCES
	if service.K8sSettings.IsLimitSetup() {
		limits := core.ResourceList{}
		requests := core.ResourceList{}
		limits["cpu"] = resource.MustParse(fmt.Sprintf("%.2fm", service.K8sSettings.LimitCpuCores*1000))
		limits["memory"] = resource.MustParse(fmt.Sprintf("%dMi", service.K8sSettings.LimitMemoryMB))
		limits["ephemeral-storage"] = resource.MustParse(fmt.Sprintf("%dMi", service.K8sSettings.EphemeralStorageMB))
		requests["cpu"] = resource.MustParse(fmt.Sprintf("%.2fm", service.K8sSettings.LimitCpuCores*200))                 // 20% of limit
		requests["memory"] = resource.MustParse(fmt.Sprintf("%dMi", int(float64(service.K8sSettings.LimitMemoryMB)*0.2))) // 20% of limit
		requests["ephemeral-storage"] = resource.MustParse(fmt.Sprintf("%dMi", service.K8sSettings.EphemeralStorageMB))
		specTemplate.Spec.Containers[0].Resources.Limits = limits
		specTemplate.Spec.Containers[0].Resources.Requests = requests
	} else {
		specTemplate.Spec.Containers[0].Resources.Limits = nil
		specTemplate.Spec.Containers[0].Resources.Requests = nil
	}

	specTemplate.Spec.Containers[0].Name = service.Name
	specTemplate.Spec.Containers[0].Command = []string{}
	specTemplate.Spec.Containers[0].Args = []string{}

	// IMAGE
	if service.ContainerImage != "" {
		specTemplate.Spec.Containers[0].Image = service.ContainerImage
		if service.ContainerImageCommand != "" {
			specTemplate.Spec.Containers[0].Command = punqUtils.ParseJsonStringArray(service.ContainerImageCommand)
		}
		if service.ContainerImageCommandArgs != "" {
			specTemplate.Spec.Containers[0].Args = punqUtils.ParseJsonStringArray(service.ContainerImageCommandArgs)
		}
		if service.ContainerImageRepoSecretDecryptValue != "" {
			specTemplate.Spec.ImagePullSecrets = []core.LocalObjectReference{}
			specTemplate.Spec.ImagePullSecrets = append(specTemplate.Spec.ImagePullSecrets, core.LocalObjectReference{
				Name: fmt.Sprintf("container-secret-service-%s", service.Name),
			})
		}
	} else {
		specTemplate.Spec.Containers[0].Image = "PLACEHOLDER-UNTIL-BUILDSERVER-OVERWRITES-THIS-IMAGE"
		// this will be setup UNTIL the buildserver overwrites the image with the real one.
		// ------------------------------------ @todo: check
		// if previousDeployment != nil {
		// 	specTemplate.Spec.Containers[0].Image = previousDeployment.specTemplate.Spec.Containers[0].Image
		// } else {
		// 	specTemplate.Spec.Containers[0].Image = "ghcr.io/mogenius/mo-default-backend:latest"
		// }
	}

	// ENV VARS
	specTemplate.Spec.Containers[0].Env = []core.EnvVar{}
	specTemplate.Spec.Containers[0].VolumeMounts = []core.VolumeMount{}
	specTemplate.Spec.Volumes = []core.Volume{}
	for _, envVar := range service.EnvVars {
		if envVar.Type == "KEY_VAULT" ||
			envVar.Type == "PLAINTEXT" ||
			envVar.Type == "HOSTNAME" {
			specTemplate.Spec.Containers[0].Env = append(specTemplate.Spec.Containers[0].Env, core.EnvVar{
				Name: envVar.Name,
				ValueFrom: &core.EnvVarSource{
					SecretKeyRef: &core.SecretKeySelector{
						Key: envVar.Name,
						LocalObjectReference: core.LocalObjectReference{
							Name: service.Name,
						},
					},
				},
			})
		}
		if envVar.Type == "VOLUME_MOUNT" {
			// VOLUMEMOUNT
			// EXAMPLE FOR value CONTENTS: VOLUME_NAME:/LOCATION_CONTAINER_DIR
			components := strings.Split(envVar.Value, ":")
			if len(components) == 3 {
				volumeName := components[0]    // e.g. MY_COOL_NAME
				srcPath := components[1]       // e.g. subpath/to/heaven
				containerPath := components[2] // e.g. /mo-data

				// subPath must be relative
				if strings.HasPrefix(srcPath, "/") {
					srcPath = strings.Replace(srcPath, "/", "", 1)
				}
				specTemplate.Spec.Containers[0].VolumeMounts = append(specTemplate.Spec.Containers[0].VolumeMounts, core.VolumeMount{
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
						specTemplate.Spec.Volumes = append(specTemplate.Spec.Volumes, core.Volume{
							Name: volumeName,
							VolumeSource: core.VolumeSource{
								PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
									ClaimName: volumeName,
								},
							},
						})
					}
				} else {
					logger.Log.Errorf("No Volume found for  '%s/%s'!!!", namespace.Name, volumeName)
				}
			} else {
				logger.Log.Errorf("SKIPPING ENVVAR '%s' because value '%s' must conform to pattern XXX:YYY:ZZZ", envVar.Type, envVar.Value)
			}
		}
	}

	// IMAGE PULL SECRET
	if ContainerSecretDoesExistForStage(namespace) && service.ContainerImageRepoSecretDecryptValue == "" {
		containerSecretName := "container-secret-" + namespace.Name
		specTemplate.Spec.ImagePullSecrets = []core.LocalObjectReference{}
		specTemplate.Spec.ImagePullSecrets = append(specTemplate.Spec.ImagePullSecrets, core.LocalObjectReference{Name: containerSecretName})
	}

	// SECURITY CONTEXT
	// TODO wieder in betrieb nehmen
	//structs.StateDebugLog(fmt.Sprintf("securityContext of '%s' removed from deployment. BENE MUST SOLVE THIS!", service.K8sName))
	specTemplate.Spec.Containers[0].SecurityContext = nil

	return controller, nil
}
