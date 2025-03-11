package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/websocket"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "k8s.io/api/apps/v1"
	v1Core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1depl "k8s.io/client-go/kubernetes/typed/apps/v1"
)

func AllDeployments(namespaceName string) []v1.Deployment {
	result := []v1.Deployment{}

	clientset := clientProvider.K8sClientSet()
	deploymentList, err := clientset.AppsV1().Deployments(namespaceName).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		k8sLogger.Error("AllDeployments", "error", err.Error())
		return result
	}

	for _, deployment := range deploymentList.Items {
		deployment.Kind = "Deployment"
		deployment.APIVersion = "apps/v1"
		result = append(result, deployment)
	}
	return result
}

func DeleteDeployment(eventClient websocket.WebsocketClient, job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand(eventClient, "delete", "Delete Deployment", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(eventClient, job, "Deleting Deployment")

		clientset := clientProvider.K8sClientSet()
		deploymentClient := clientset.AppsV1().Deployments(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: utils.Pointer[int64](5),
		}

		err := deploymentClient.Delete(context.TODO(), service.ControllerName, deleteOptions)
		if err != nil {
			cmd.Fail(eventClient, job, fmt.Sprintf("DeleteDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(eventClient, job, "Deleted Deployment")
		}
	}(wg)
}

func GetDeployment(namespaceName string, controllerName string) (*v1.Deployment, error) {
	clientset := clientProvider.K8sClientSet()
	client := clientset.AppsV1().Deployments(namespaceName)
	return client.Get(context.TODO(), controllerName, metav1.GetOptions{})
}

func UpdateDeployment(eventClient websocket.WebsocketClient, job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand(eventClient, "update", "Update Deployment", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(eventClient, job, "Updating Deployment")

		deploymentClient := clientProvider.K8sClientSet().AppsV1().Deployments(namespace.Name)

		newController, err := CreateControllerConfiguration(job.ProjectId, namespace, service, false, deploymentClient, createDeploymentHandler)
		if err != nil {
			k8sLogger.Error("Failed to create controller configuration", "error", err)
			cmd.Fail(eventClient, job, fmt.Sprintf("UpdateDeployment ERROR: %s", err.Error()))
			return
		}

		deployment := newController.(*v1.Deployment)
		_, err = deploymentClient.Update(context.TODO(), deployment, MoUpdateOptions())
		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err = deploymentClient.Create(context.TODO(), deployment, MoCreateOptions())
				if err != nil {
					cmd.Fail(eventClient, job, fmt.Sprintf("CreateDeployment ERROR: %s", err.Error()))
				} else {
					cmd.Success(eventClient, job, "Created deployment")
				}
			} else {
				cmd.Fail(eventClient, job, fmt.Sprintf("UpdatingDeployment ERROR: %s", err.Error()))
			}
		} else {
			cmd.Success(eventClient, job, "Updating deployment")
		}

		HandleHpa(eventClient, job, namespace.Name, service.ControllerName, service, wg)
	}(wg)
}

func StartDeployment(eventClient websocket.WebsocketClient, job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand(eventClient, "start", "Start Deployment", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(eventClient, job, "Starting Deployment")

		clientset := clientProvider.K8sClientSet()
		deploymentClient := clientset.AppsV1().Deployments(namespace.Name)

		newController, err := CreateControllerConfiguration(job.ProjectId, namespace, service, false, deploymentClient, createDeploymentHandler)
		if err != nil {
			k8sLogger.Error("Failed to create controller configuration", "error", err)
			cmd.Fail(eventClient, job, fmt.Sprintf("StartDeployment ERROR: %s", err.Error()))
			return
		}

		// deployment := generateDeployment(namespace, service, false, deploymentClient)
		deployment := newController.(*v1.Deployment)

		_, err = deploymentClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(eventClient, job, fmt.Sprintf("StartingDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(eventClient, job, "Started Deployment")
		}

		HandleHpa(eventClient, job, namespace.Name, service.ControllerName, service, wg)
	}(wg)
}

func StopDeployment(eventClient websocket.WebsocketClient, job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand(eventClient, "stop", "Stopping Deployment", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(eventClient, job, "Stopping Deployment")

		clientset := clientProvider.K8sClientSet()
		deploymentClient := clientset.AppsV1().Deployments(namespace.Name)
		newController, err := CreateControllerConfiguration(job.ProjectId, namespace, service, false, deploymentClient, createDeploymentHandler)
		if err != nil {
			k8sLogger.Error("Failed to create controller configuration", "error", err)
			cmd.Fail(eventClient, job, fmt.Sprintf("StopDeployment ERROR: %s", err.Error()))
			return
		}

		// deployment := generateDeployment(namespace, service, false, deploymentClient)
		deployment := newController.(*v1.Deployment)

		deployment.Spec.Replicas = utils.Pointer[int32](0)

		_, err = deploymentClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(eventClient, job, fmt.Sprintf("StopDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(eventClient, job, "Stopped Deployment")
		}

		HandleHpa(eventClient, job, namespace.Name, service.ControllerName, service, wg)
	}(wg)
}

func RestartDeployment(eventClient websocket.WebsocketClient, job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand(eventClient, "restart", "Restart Deployment", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(eventClient, job, "Restarting Deployment")

		clientset := clientProvider.K8sClientSet()
		deploymentClient := clientset.AppsV1().Deployments(namespace.Name)

		newController, err := CreateControllerConfiguration(job.ProjectId, namespace, service, false, deploymentClient, createDeploymentHandler)
		if err != nil {
			k8sLogger.Error("Failed to create controller configuration", "error", err)
			cmd.Fail(eventClient, job, fmt.Sprintf("RestartDeployment ERROR: %s", err.Error()))
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
			cmd.Fail(eventClient, job, fmt.Sprintf("RestartDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(eventClient, job, "Restart Deployment")
		}

		HandleHpa(eventClient, job, namespace.Name, service.ControllerName, service, wg)
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

	newDeployment := utils.InitDeployment()

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
	spec.Replicas = utils.Pointer(int32(service.ReplicaCount))

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

		if container.Probes != nil {
			// LivenessProbe
			if !container.Probes.LivenessProbe.IsActive {
				spec.Template.Spec.Containers[index].LivenessProbe = nil
			} else if container.Probes.LivenessProbe.IsActive {
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
			if !container.Probes.ReadinessProbe.IsActive {
				spec.Template.Spec.Containers[index].ReadinessProbe = nil
			} else if container.Probes.ReadinessProbe.IsActive {
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
			if !container.Probes.StartupProbe.IsActive {
				spec.Template.Spec.Containers[index].StartupProbe = nil
			} else if container.Probes.StartupProbe.IsActive {
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
		}
	}

	return objectMeta, &SpecDeployment{spec, previousSpec}, &newDeployment, nil
}

func UpdateDeploymentImage(namespaceName string, controllerName string, containerName string, imageName string) error {
	clientset := clientProvider.K8sClientSet()
	deploymentClient := clientset.AppsV1().Deployments(namespaceName)
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
	// deploymentToUpdate.Spec.Paused = false

	_, err = deploymentClient.Update(context.TODO(), deploymentToUpdate, metav1.UpdateOptions{})
	return err
}

func GetDeploymentImage(namespaceName string, controllerName string, containerName string) (string, error) {
	clientset := clientProvider.K8sClientSet()
	deploymentClient := clientset.AppsV1().Deployments(namespaceName)
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
	clientset := clientProvider.K8sClientSet()
	client := clientset.AppsV1().Deployments(namespace)

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
	clientset := clientProvider.K8sClientSet()
	client := clientset.AppsV1().Deployments(namespace)

	deployments, err := client.List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return result, err
	}

	return deployments.Items, err
}

func GetDeploymentResult(namespace string, name string) K8sWorkloadResult {
	deployment, err := GetK8sDeployment(namespace, name)
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(deployment, err)
}

func GetK8sDeployment(namespaceName string, name string) (*v1.Deployment, error) {
	clientset := clientProvider.K8sClientSet()
	deployment, err := clientset.AppsV1().Deployments(namespaceName).Get(context.TODO(), name, metav1.GetOptions{})
	deployment.Kind = "Deployment"
	deployment.APIVersion = "apps/v1"

	return deployment, err
}

func IsDeploymentInstalled(namespaceName string, name string) (string, error) {
	ownDeployment, err := GetDeployment(namespaceName, name)
	if err != nil {
		return "", err
	}

	result := ""
	split := strings.Split(ownDeployment.Spec.Template.Spec.Containers[0].Image, ":")
	if len(split) > 1 {
		result = split[1]
	}

	return result, nil
}

func AllDeploymentsIncludeIgnored(namespaceName string) []v1.Deployment {
	result := []v1.Deployment{}
	clientset := clientProvider.K8sClientSet()
	deploymentList, err := clientset.AppsV1().Deployments(namespaceName).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		k8sLogger.Error("AllDeployment", "error", err.Error())
		return result
	}

	for _, deployment := range deploymentList.Items {
		deployment.Kind = "Deployment"
		deployment.APIVersion = "apps/v1"
		result = append(result, deployment)
	}

	return result
}
