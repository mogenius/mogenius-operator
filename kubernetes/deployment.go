package kubernetes

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/intstr"
	"mogenius-k8s-manager/dtos"
	iacmanager "mogenius-k8s-manager/iac-manager"
	"mogenius-k8s-manager/store"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"strings"
	"sync"
	"time"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	v1 "k8s.io/api/apps/v1"
	v1Core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	v1depl "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

func DeleteDeployment(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", "Delete Deployment", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting Deployment")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: punqUtils.Pointer[int64](5),
		}

		err = deploymentClient.Delete(context.TODO(), service.ControllerName, deleteOptions)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("DeleteDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Deleted Deployment")
		}
		// EXTERNAL SECRETS OPERATOR - cleanup unused secrets
		if utils.CONFIG.Misc.ExternalSecretsEnabled && service.ExternalSecretsEnabled() {
			DeleteUnusedSecretsForNamespace(namespace.Name)
		}
	}(wg)
}

func UpdateDeployment(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("update", "Update Deployment", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Updating Deployment")

		deploymentClient := GetAppClient().Deployments(namespace.Name)

		newController, err := CreateControllerConfiguration(job.ProjectId, namespace, service, false, deploymentClient, createDeploymentHandler)
		if err != nil {
			K8sLogger.Errorf("error: %s", err.Error())
			cmd.Fail(job, fmt.Sprintf("UpdateDeployment ERROR: %s", err.Error()))
			return
		}
		// add resource creation for external secrets
		if utils.CONFIG.Misc.ExternalSecretsEnabled && service.ExternalSecretsEnabled() {
			CreateExternalSecret(CreateExternalSecretProps{
				Namespace:             namespace.Name,
				ServiceName:           service.ControllerName,
				ProjectName:           service.EsoSettings.ProjectName,
				SecretStoreNamePrefix: service.EsoSettings.SecretStoreNamePrefix,
			})
			DeleteUnusedSecretsForNamespace(namespace.Name)
		}

		deployment := newController.(*v1.Deployment)
		_, err = deploymentClient.Update(context.TODO(), deployment, MoUpdateOptions())
		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err = deploymentClient.Create(context.TODO(), deployment, MoCreateOptions())
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("CreateDeployment ERROR: %s", err.Error()))
				} else {
					cmd.Success(job, "Created deployment")
				}
			} else {
				cmd.Fail(job, fmt.Sprintf("UpdatingDeployment ERROR: %s", err.Error()))
			}
		} else {
			cmd.Success(job, "Updating deployment")
		}

		HandleHpa(job, namespace.Name, service.ControllerName, service, wg)
	}(wg)
}

func StartDeployment(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("start", "Start Deployment", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Starting Deployment")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespace.Name)

		newController, err := CreateControllerConfiguration(job.ProjectId, namespace, service, false, deploymentClient, createDeploymentHandler)
		if err != nil {
			K8sLogger.Errorf("error: %s", err.Error())
			cmd.Fail(job, fmt.Sprintf("StartDeployment ERROR: %s", err.Error()))
			return
		}

		// deployment := generateDeployment(namespace, service, false, deploymentClient)
		deployment := newController.(*v1.Deployment)

		_, err = deploymentClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("StartingDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Started Deployment")
		}

		HandleHpa(job, namespace.Name, service.ControllerName, service, wg)
	}(wg)
}

func StopDeployment(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("stop", "Stopping Deployment", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Stopping Deployment")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespace.Name)
		newController, err := CreateControllerConfiguration(job.ProjectId, namespace, service, false, deploymentClient, createDeploymentHandler)
		if err != nil {
			K8sLogger.Errorf("error: %s", err.Error())
			cmd.Fail(job, fmt.Sprintf("StopDeployment ERROR: %s", err.Error()))
			return
		}

		// deployment := generateDeployment(namespace, service, false, deploymentClient)
		deployment := newController.(*v1.Deployment)

		deployment.Spec.Replicas = punqUtils.Pointer[int32](0)

		_, err = deploymentClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("StopDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Stopped Deployment")
		}

		HandleHpa(job, namespace.Name, service.ControllerName, service, wg)
	}(wg)
}

func RestartDeployment(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("restart", "Restart Deployment", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Restarting Deployment")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespace.Name)

		newController, err := CreateControllerConfiguration(job.ProjectId, namespace, service, false, deploymentClient, createDeploymentHandler)
		if err != nil {
			K8sLogger.Errorf("error: %s", err.Error())
			cmd.Fail(job, fmt.Sprintf("RestartDeployment ERROR: %s", err.Error()))
			return
		}

		// deployment := generateDeployment(namespace, service, false, deploymentClient)
		deployment := newController.(*v1.Deployment)

		// KUBERNETES ISSUES A "rollout restart deployment" WHENETHER THE METADATA IS CHANGED.
		if deployment.Spec.Template.ObjectMeta.Annotations == nil {
			deployment.Spec.Template.ObjectMeta.Annotations = map[string]string{}
			deployment.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
		} else {
			deployment.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
		}

		_, err = deploymentClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("RestartDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Restart Deployment")
		}

		HandleHpa(job, namespace.Name, service.ControllerName, service, wg)
	}(wg)
}

func createDeploymentHandler(namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, freshlyCreated bool, client interface{}) (*metav1.ObjectMeta, HasSpec, interface{}, error) {
	var previousSpec *v1.DeploymentSpec
	previousDeployment, err := client.(v1depl.DeploymentInterface).Get(context.TODO(), service.ControllerName, metav1.GetOptions{})
	if err != nil {
		previousDeployment = nil
	} else {
		previousSpec = &(*previousDeployment).Spec
	}

	newDeployment := punqUtils.InitDeployment()

	// check if default deployment exists
	defaultDeployment := GetCustomDeploymentTemplate()
	if previousDeployment == nil && defaultDeployment != nil {
		// create new
		newDeployment = *defaultDeployment
	} else if previousDeployment != nil {
		// update existing
		newDeployment = *previousDeployment
	}

	objectMeta := &newDeployment.ObjectMeta
	spec := &newDeployment.Spec

	// STRATEGY
	if service.DeploymentStrategy == dtos.StrategyRolling {
		spec.Strategy.Type = v1.RollingUpdateDeploymentStrategyType
	} else if service.DeploymentStrategy == dtos.StrategyRecreate {
		spec.Strategy.Type = v1.RecreateDeploymentStrategyType
	} else {
		spec.Strategy.Type = v1.RecreateDeploymentStrategyType
	}

	// REPLICAS
	spec.Replicas = punqUtils.Pointer(int32(service.ReplicaCount))

	// PAUSE only on "freshly created" or Repository-Types which needs a build beforehand
	if freshlyCreated && service.HasContainerWithGitRepo() {
		spec.Paused = true
	} else {
		spec.Paused = false
	}

	// LABELS
	if spec.Selector == nil {
		spec.Selector = &metav1.LabelSelector{}
	}
	if spec.Selector.MatchLabels == nil {
		spec.Selector.MatchLabels = map[string]string{}
	}
	spec.Selector.MatchLabels["app"] = service.ControllerName
	spec.Selector.MatchLabels["ns"] = namespace.Name

	if spec.Template.ObjectMeta.Labels == nil {
		spec.Template.ObjectMeta.Labels = map[string]string{}
	}
	spec.Template.ObjectMeta.Labels["app"] = service.ControllerName
	spec.Template.ObjectMeta.Labels["ns"] = namespace.Name

	// CONTAINERS
	if spec.Template.Spec.Containers == nil {
		spec.Template.Spec.Containers = []v1Core.Container{}
	}
	for index, container := range service.Containers {
		if len(spec.Template.Spec.Containers) <= index {
			spec.Template.Spec.Containers = append(spec.Template.Spec.Containers, v1Core.Container{})
		}

		// ImagePullPolicy
		if container.KubernetesLimits.ImagePullPolicy != "" {
			spec.Template.Spec.Containers[index].ImagePullPolicy = v1Core.PullPolicy(container.KubernetesLimits.ImagePullPolicy)
		} else {
			spec.Template.Spec.Containers[index].ImagePullPolicy = v1Core.PullAlways
		}

		// LivenessProbe
		log.Infof("LivenessProbe: %v", container.Probes)
		if !container.Probes.LivenessProbe.ProbesOn {
			spec.Template.Spec.Containers[index].LivenessProbe = nil
		} else if container.Probes.LivenessProbe.ProbesOn {
			if spec.Template.Spec.Containers[index].LivenessProbe == nil {
				spec.Template.Spec.Containers[index].LivenessProbe = &v1Core.Probe{}
				spec.Template.Spec.Containers[index].LivenessProbe.HTTPGet = &v1Core.HTTPGetAction{}
			}
			spec.Template.Spec.Containers[index].LivenessProbe.InitialDelaySeconds = int32(container.Probes.LivenessProbe.InitialDelaySeconds)
			spec.Template.Spec.Containers[index].LivenessProbe.PeriodSeconds = int32(container.Probes.LivenessProbe.PeriodSeconds)
			spec.Template.Spec.Containers[index].LivenessProbe.TimeoutSeconds = int32(container.Probes.LivenessProbe.TimeoutSeconds)
			spec.Template.Spec.Containers[index].LivenessProbe.SuccessThreshold = int32(container.Probes.LivenessProbe.SuccessThreshold)
			spec.Template.Spec.Containers[index].LivenessProbe.FailureThreshold = int32(container.Probes.LivenessProbe.FailureThreshold)

			if container.Probes.LivenessProbe.HTTPGet != nil {
				spec.Template.Spec.Containers[index].LivenessProbe.HTTPGet.Path = container.Probes.LivenessProbe.HTTPGet.Path
				spec.Template.Spec.Containers[index].LivenessProbe.HTTPGet.Port = intstr.FromInt32(int32(container.Probes.LivenessProbe.HTTPGet.Port))
				if container.Probes.LivenessProbe.HTTPGet.Host != nil {
					spec.Template.Spec.Containers[index].LivenessProbe.HTTPGet.Host = *container.Probes.LivenessProbe.HTTPGet.Host
				} else {
					spec.Template.Spec.Containers[index].LivenessProbe.HTTPGet.Host = ""
				}
				if container.Probes.LivenessProbe.HTTPGet.Scheme != nil {
					spec.Template.Spec.Containers[index].LivenessProbe.HTTPGet.Scheme = *container.Probes.LivenessProbe.HTTPGet.Scheme
				} else {
					spec.Template.Spec.Containers[index].LivenessProbe.HTTPGet.Scheme = ""
				}
				spec.Template.Spec.Containers[index].LivenessProbe.HTTPGet.HTTPHeaders = []v1Core.HTTPHeader{}
				if container.Probes.LivenessProbe.HTTPGet.HTTPHeaders != nil {
					for _, header := range *container.Probes.LivenessProbe.HTTPGet.HTTPHeaders {
						spec.Template.Spec.Containers[index].LivenessProbe.HTTPGet.HTTPHeaders = append(spec.Template.Spec.Containers[index].LivenessProbe.HTTPGet.HTTPHeaders, v1Core.HTTPHeader{
							Name:  header.Name,
							Value: header.Value,
						})
					}
				} else if container.Probes.LivenessProbe.TCPSocket != nil {
					spec.Template.Spec.Containers[index].LivenessProbe.TCPSocket = &v1Core.TCPSocketAction{}
					spec.Template.Spec.Containers[index].LivenessProbe.TCPSocket.Port = intstr.FromInt32(int32(container.Probes.LivenessProbe.TCPSocket.Port))
				} else if container.Probes.LivenessProbe.Exec != nil {
					spec.Template.Spec.Containers[index].LivenessProbe.Exec = &v1Core.ExecAction{}
					spec.Template.Spec.Containers[index].LivenessProbe.Exec.Command = container.Probes.LivenessProbe.Exec.Command
				} else if container.Probes.LivenessProbe.GRPC != nil {
					spec.Template.Spec.Containers[index].LivenessProbe.GRPC = &v1Core.GRPCAction{}
					spec.Template.Spec.Containers[index].LivenessProbe.GRPC.Port = int32(container.Probes.LivenessProbe.GRPC.Port)
					spec.Template.Spec.Containers[index].LivenessProbe.GRPC.Service = container.Probes.LivenessProbe.GRPC.Service
				}
			}
		}

		// ReadinessProbe
		if !container.Probes.ReadinessProbe.ProbesOn {
			spec.Template.Spec.Containers[index].ReadinessProbe = nil
		} else if container.Probes.ReadinessProbe.ProbesOn {
			if spec.Template.Spec.Containers[index].ReadinessProbe == nil {
				spec.Template.Spec.Containers[index].ReadinessProbe = &v1Core.Probe{}
				spec.Template.Spec.Containers[index].ReadinessProbe.HTTPGet = &v1Core.HTTPGetAction{}
			}
			spec.Template.Spec.Containers[index].ReadinessProbe.InitialDelaySeconds = int32(container.Probes.ReadinessProbe.InitialDelaySeconds)
			spec.Template.Spec.Containers[index].ReadinessProbe.PeriodSeconds = int32(container.Probes.ReadinessProbe.PeriodSeconds)
			spec.Template.Spec.Containers[index].ReadinessProbe.TimeoutSeconds = int32(container.Probes.ReadinessProbe.TimeoutSeconds)
			spec.Template.Spec.Containers[index].ReadinessProbe.SuccessThreshold = int32(container.Probes.ReadinessProbe.SuccessThreshold)
			spec.Template.Spec.Containers[index].ReadinessProbe.FailureThreshold = int32(container.Probes.ReadinessProbe.FailureThreshold)

			if container.Probes.ReadinessProbe.HTTPGet != nil {
				spec.Template.Spec.Containers[index].ReadinessProbe.HTTPGet.Path = container.Probes.ReadinessProbe.HTTPGet.Path
				spec.Template.Spec.Containers[index].ReadinessProbe.HTTPGet.Port = intstr.FromInt32(int32(container.Probes.ReadinessProbe.HTTPGet.Port))
				if container.Probes.ReadinessProbe.HTTPGet.Host != nil {
					spec.Template.Spec.Containers[index].ReadinessProbe.HTTPGet.Host = *container.Probes.ReadinessProbe.HTTPGet.Host
				} else {
					spec.Template.Spec.Containers[index].ReadinessProbe.HTTPGet.Host = ""
				}
				if container.Probes.ReadinessProbe.HTTPGet.Scheme != nil {
					spec.Template.Spec.Containers[index].ReadinessProbe.HTTPGet.Scheme = *container.Probes.ReadinessProbe.HTTPGet.Scheme
				} else {
					spec.Template.Spec.Containers[index].ReadinessProbe.HTTPGet.Scheme = ""
				}
				spec.Template.Spec.Containers[index].ReadinessProbe.HTTPGet.HTTPHeaders = []v1Core.HTTPHeader{}
				if container.Probes.ReadinessProbe.HTTPGet.HTTPHeaders != nil {
					for _, header := range *container.Probes.ReadinessProbe.HTTPGet.HTTPHeaders {
						spec.Template.Spec.Containers[index].ReadinessProbe.HTTPGet.HTTPHeaders = append(spec.Template.Spec.Containers[index].ReadinessProbe.HTTPGet.HTTPHeaders, v1Core.HTTPHeader{
							Name:  header.Name,
							Value: header.Value,
						})
					}
				} else if container.Probes.ReadinessProbe.TCPSocket != nil {
					spec.Template.Spec.Containers[index].ReadinessProbe.TCPSocket = &v1Core.TCPSocketAction{}
					spec.Template.Spec.Containers[index].ReadinessProbe.TCPSocket.Port = intstr.FromInt32(int32(container.Probes.ReadinessProbe.TCPSocket.Port))
				} else if container.Probes.ReadinessProbe.Exec != nil {
					spec.Template.Spec.Containers[index].ReadinessProbe.Exec = &v1Core.ExecAction{}
					spec.Template.Spec.Containers[index].ReadinessProbe.Exec.Command = container.Probes.ReadinessProbe.Exec.Command
				} else if container.Probes.ReadinessProbe.GRPC != nil {
					spec.Template.Spec.Containers[index].ReadinessProbe.GRPC = &v1Core.GRPCAction{}
					spec.Template.Spec.Containers[index].ReadinessProbe.GRPC.Port = int32(container.Probes.ReadinessProbe.GRPC.Port)
					spec.Template.Spec.Containers[index].ReadinessProbe.GRPC.Service = container.Probes.ReadinessProbe.GRPC.Service
				}
			}
		}

		// StartupProbe
		if !container.Probes.StartupProbe.ProbesOn {
			spec.Template.Spec.Containers[index].StartupProbe = nil
		} else if container.Probes.StartupProbe.ProbesOn {
			if spec.Template.Spec.Containers[index].StartupProbe == nil {
				spec.Template.Spec.Containers[index].StartupProbe = &v1Core.Probe{}
				spec.Template.Spec.Containers[index].StartupProbe.HTTPGet = &v1Core.HTTPGetAction{}
			}
			spec.Template.Spec.Containers[index].StartupProbe.InitialDelaySeconds = int32(container.Probes.StartupProbe.InitialDelaySeconds)
			spec.Template.Spec.Containers[index].StartupProbe.PeriodSeconds = int32(container.Probes.StartupProbe.PeriodSeconds)
			spec.Template.Spec.Containers[index].StartupProbe.TimeoutSeconds = int32(container.Probes.StartupProbe.TimeoutSeconds)
			spec.Template.Spec.Containers[index].StartupProbe.SuccessThreshold = int32(container.Probes.StartupProbe.SuccessThreshold)
			spec.Template.Spec.Containers[index].StartupProbe.FailureThreshold = int32(container.Probes.StartupProbe.FailureThreshold)

			if container.Probes.StartupProbe.HTTPGet != nil {
				spec.Template.Spec.Containers[index].StartupProbe.HTTPGet.Path = container.Probes.StartupProbe.HTTPGet.Path
				spec.Template.Spec.Containers[index].StartupProbe.HTTPGet.Port = intstr.FromInt32(int32(container.Probes.StartupProbe.HTTPGet.Port))
				if container.Probes.StartupProbe.HTTPGet.Host != nil {
					spec.Template.Spec.Containers[index].StartupProbe.HTTPGet.Host = *container.Probes.StartupProbe.HTTPGet.Host
				} else {
					spec.Template.Spec.Containers[index].StartupProbe.HTTPGet.Host = ""
				}
				if container.Probes.StartupProbe.HTTPGet.Scheme != nil {
					spec.Template.Spec.Containers[index].StartupProbe.HTTPGet.Scheme = *container.Probes.StartupProbe.HTTPGet.Scheme
				} else {
					spec.Template.Spec.Containers[index].StartupProbe.HTTPGet.Scheme = ""
				}
				spec.Template.Spec.Containers[index].StartupProbe.HTTPGet.HTTPHeaders = []v1Core.HTTPHeader{}
				if container.Probes.StartupProbe.HTTPGet.HTTPHeaders != nil {
					for _, header := range *container.Probes.StartupProbe.HTTPGet.HTTPHeaders {
						spec.Template.Spec.Containers[index].StartupProbe.HTTPGet.HTTPHeaders = append(spec.Template.Spec.Containers[index].StartupProbe.HTTPGet.HTTPHeaders, v1Core.HTTPHeader{
							Name:  header.Name,
							Value: header.Value,
						})
					}
				} else if container.Probes.StartupProbe.TCPSocket != nil {
					spec.Template.Spec.Containers[index].StartupProbe.TCPSocket = &v1Core.TCPSocketAction{}
					spec.Template.Spec.Containers[index].StartupProbe.TCPSocket.Port = intstr.FromInt32(int32(container.Probes.StartupProbe.TCPSocket.Port))
				} else if container.Probes.StartupProbe.Exec != nil {
					spec.Template.Spec.Containers[index].StartupProbe.Exec = &v1Core.ExecAction{}
					spec.Template.Spec.Containers[index].StartupProbe.Exec.Command = container.Probes.StartupProbe.Exec.Command
				} else if container.Probes.StartupProbe.GRPC != nil {
					spec.Template.Spec.Containers[index].StartupProbe.GRPC = &v1Core.GRPCAction{}
					spec.Template.Spec.Containers[index].StartupProbe.GRPC.Port = int32(container.Probes.StartupProbe.GRPC.Port)
					spec.Template.Spec.Containers[index].StartupProbe.GRPC.Service = container.Probes.StartupProbe.GRPC.Service
				}
			}
		}

		// TODO REMOVE
		// PORTS
		//var internalHttpPort *int
		//if len(container.Ports) > 0 {
		//	for _, port := range container.Ports {
		//		if port.PortType == dtos.PortTypeHTTPS {
		//			tmp := int(port.InternalPort)
		//			internalHttpPort = &tmp
		//		}
		//	}
		//}
		//
		//// PROBES
		//if !container.KubernetesLimits.ProbesOn {
		//	spec.Template.Spec.Containers[index].StartupProbe = nil
		//	spec.Template.Spec.Containers[index].LivenessProbe = nil
		//	spec.Template.Spec.Containers[index].ReadinessProbe = nil
		//} else if internalHttpPort != nil {
		//	// StartupProbe
		//	if spec.Template.Spec.Containers[index].StartupProbe == nil {
		//		spec.Template.Spec.Containers[index].StartupProbe = &v1Core.Probe{}
		//		spec.Template.Spec.Containers[index].StartupProbe.HTTPGet = &v1Core.HTTPGetAction{}
		//	}
		//	spec.Template.Spec.Containers[index].StartupProbe.HTTPGet.Port = intstr.FromInt32(int32(*internalHttpPort))
		//
		//	// LivenessProbe
		//	if spec.Template.Spec.Containers[index].LivenessProbe == nil {
		//		spec.Template.Spec.Containers[index].LivenessProbe = &v1Core.Probe{}
		//		spec.Template.Spec.Containers[index].LivenessProbe.HTTPGet = &v1Core.HTTPGetAction{}
		//	}
		//	spec.Template.Spec.Containers[index].LivenessProbe.HTTPGet.Port = intstr.FromInt32(int32(*internalHttpPort))
		//
		//	// ReadinessProbe
		//	if spec.Template.Spec.Containers[index].ReadinessProbe == nil {
		//		spec.Template.Spec.Containers[index].ReadinessProbe = &v1Core.Probe{}
		//		spec.Template.Spec.Containers[index].ReadinessProbe.HTTPGet = &v1Core.HTTPGetAction{}
		//	}
		//	spec.Template.Spec.Containers[index].ReadinessProbe.HTTPGet.Port = intstr.FromInt32(int32(*internalHttpPort))
		//}
	}

	return objectMeta, &SpecDeployment{spec, previousSpec}, &newDeployment, nil
}

// func SetDeploymentImage(job *structs.Job, namespaceName string, controllerName string, containerName string, imageName string, wg *sync.WaitGroup) {
// 	cmd := structs.CreateCommand("setImage", "Set Deployment Image", job)
// 	wg.Add(1)
// 	go func(wg *sync.WaitGroup) {
// 		defer wg.Done()
// 		cmd.Start(job, "Set Image in Deployment")

// 		provider, err := punq.NewKubeProvider(nil)
// 		if err != nil {
// 			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
// 			return
// 		}
// 		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespaceName)
// 		deploymentToUpdate, err := deploymentClient.Get(context.TODO(), controllerName, metav1.GetOptions{})
// 		if err != nil {
// 			cmd.Fail(job, fmt.Sprintf("SetImage ERROR: %s", err.Error()))
// 			return
// 		}

// 		// SET NEW IMAGE
// 		for index, container := range deploymentToUpdate.Spec.Template.Spec.Containers {
// 			if container.Name == containerName {
// 				deploymentToUpdate.Spec.Template.Spec.Containers[index].Image = imageName
// 			}
// 		}
// 		deploymentToUpdate.Spec.Paused = false

// 		_, err = deploymentClient.Update(context.TODO(), deploymentToUpdate, metav1.UpdateOptions{})
// 		if err != nil {
// 			cmd.Fail(job, fmt.Sprintf("SetImage ERROR: %s", err.Error()))
// 		} else {
// 			cmd.Success(job, "Set new image in Deployment")
// 		}
// 	}(wg)
// }

func UpdateDeploymentImage(namespaceName string, controllerName string, containerName string, imageName string) error {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return err
	}
	deploymentClient := provider.ClientSet.AppsV1().Deployments(namespaceName)
	deploymentToUpdate, err := deploymentClient.Get(context.TODO(), controllerName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// SET NEW IMAGE
	for index, container := range deploymentToUpdate.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			deploymentToUpdate.Spec.Template.Spec.Containers[index].Image = imageName
		}
	}
	deploymentToUpdate.Spec.Paused = false

	_, err = deploymentClient.Update(context.TODO(), deploymentToUpdate, metav1.UpdateOptions{})
	return err
}

func GetDeploymentImage(namespaceName string, controllerName string, containerName string) (string, error) {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return "", err
	}
	deploymentClient := provider.ClientSet.AppsV1().Deployments(namespaceName)
	deploymentToUpdate, err := deploymentClient.Get(context.TODO(), controllerName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	for _, container := range deploymentToUpdate.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			return container.Image, nil
		}
	}
	return "", fmt.Errorf("Container '%s' not found in Deployment '%s'", containerName, controllerName)
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

func GetDeploymentsWithFieldSelector(namespace string, labelSelector string) ([]v1.Deployment, error) {
	result := []v1.Deployment{}
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return result, err
	}
	client := provider.ClientSet.AppsV1().Deployments(namespace)

	deployments, err := client.List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return result, err
	}

	return deployments.Items, err
}

func GetDeploymentResult(namespace string, name string) K8sWorkloadResult {
	deployment, err := punq.GetK8sDeployment(namespace, name, nil)
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(deployment, err)
}

func WatchDeployments() {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		K8sLogger.Fatalf("Error creating provider for watcher. Cannot continue because it is vital: %s", err.Error())
		return
	}

	// Retry watching resources with exponential backoff in case of failures
	err = retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}, apierrors.IsServiceUnavailable, func() error {
		return watchDeployments(provider, "deployments")
	})
	if err != nil {
		K8sLogger.Fatalf("Error watching deployments: %s", err.Error())
	}

	// Wait forever
	select {}
}

func watchDeployments(provider *punq.KubeProvider, kindName string) error {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			castedObj := obj.(*v1.Deployment)
			store.GlobalStore.Set(castedObj, "Deployment", castedObj.Namespace, castedObj.Name)

			if utils.IacWorkloadConfigMap[dtos.KindDeployments] {
				castedObj.Kind = "Deployment"
				castedObj.APIVersion = "apps/v1"
				iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			castedObj := newObj.(*v1.Deployment)
			store.GlobalStore.Set(castedObj, "Deployment", castedObj.Namespace, castedObj.Name)

			if utils.IacWorkloadConfigMap[dtos.KindDeployments] {
				castedObj.Kind = "Deployment"
				castedObj.APIVersion = "apps/v1"
				iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
			}
		},
		DeleteFunc: func(obj interface{}) {
			castedObj := obj.(*v1.Deployment)
			store.GlobalStore.Delete("Deployment", castedObj.Namespace, castedObj.Name)

			if utils.IacWorkloadConfigMap[dtos.KindDeployments] {
				castedObj.Kind = "Deployment"
				castedObj.APIVersion = "apps/v1"
				iacmanager.DeleteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, obj)
			}
		},
	}
	listWatch := cache.NewListWatchFromClient(
		provider.ClientSet.AppsV1().RESTClient(),
		kindName,
		v1Core.NamespaceAll,
		fields.Nothing(),
	)
	resourceInformer := cache.NewSharedInformer(listWatch, &v1.Deployment{}, 0)
	_, err := resourceInformer.AddEventHandler(handler)
	if err != nil {
		return err
	}

	stopCh := make(chan struct{})
	go resourceInformer.Run(stopCh)

	// Wait for the informer to sync and start processing events
	if !cache.WaitForCacheSync(stopCh, resourceInformer.HasSynced) {
		return fmt.Errorf("failed to sync cache")
	}

	// This loop will keep the function alive as long as the stopCh is not closed
	for {
		select {
		case <-stopCh:
			// stopCh closed, return from the function
			return nil
		case <-time.After(30 * time.Second):
			// This is to avoid a tight loop in case stopCh is never closed.
			// You can adjust the time as per your needs.
		}
	}
}
