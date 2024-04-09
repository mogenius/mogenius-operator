package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	iacmanager "mogenius-k8s-manager/iac-manager"
	"mogenius-k8s-manager/structs"
	"strings"
	"sync"
	"time"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	v1Core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	v1depl "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

func CreateDeployment(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
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
		newController, err := CreateControllerConfiguration(namespace, service, true, deploymentClient, createDeploymentHandler)
		if err != nil {
			log.Errorf("error: %s", err.Error())
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
	cmd := structs.CreateCommand(fmt.Sprintf("Deleting Deployment '%s'.", service.ControllerName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting Deployment '%s'.", service.ControllerName))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespace.Name)

		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: punqUtils.Pointer[int64](5),
		}

		err = deploymentClient.Delete(context.TODO(), service.ControllerName, deleteOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted Deployment '%s'.", service.ControllerName))
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
		newController, err := CreateControllerConfiguration(namespace, service, false, deploymentClient, createDeploymentHandler)
		if err != nil {
			log.Errorf("error: %s", err.Error())
		}

		// deployment := generateDeployment(namespace, service, false, deploymentClient)
		deployment := newController.(*v1.Deployment)

		_, err = deploymentClient.Update(context.TODO(), deployment, MoUpdateOptions())
		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err = deploymentClient.Create(context.TODO(), deployment, MoCreateOptions())
				if err != nil {
					cmd.Fail(fmt.Sprintf("CreateDeployment ERROR: %s", err.Error()))
				} else {
					cmd.Success(fmt.Sprintf("Created deployment '%s'.", namespace.Name))
				}
			} else {
				cmd.Fail(fmt.Sprintf("UpdatingDeployment ERROR: %s", err.Error()))
			}
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
		cmd.Start(fmt.Sprintf("Starting Deployment '%s'.", service.ControllerName))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespace.Name)

		newController, err := CreateControllerConfiguration(namespace, service, false, deploymentClient, createDeploymentHandler)
		if err != nil {
			log.Errorf("error: %s", err.Error())
		}

		// deployment := generateDeployment(namespace, service, false, deploymentClient)
		deployment := newController.(*v1.Deployment)

		_, err = deploymentClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("StartingDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Started Deployment '%s'.", service.ControllerName))
		}
	}(cmd, wg)
	return cmd
}

func StopDeployment(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Stopping Deployment", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Stopping Deployment '%s'.", service.ControllerName))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespace.Name)
		newController, err := CreateControllerConfiguration(namespace, service, false, deploymentClient, createDeploymentHandler)
		if err != nil {
			log.Errorf("error: %s", err.Error())
		}

		// deployment := generateDeployment(namespace, service, false, deploymentClient)
		deployment := newController.(*v1.Deployment)

		deployment.Spec.Replicas = punqUtils.Pointer[int32](0)

		_, err = deploymentClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("StopDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Stopped Deployment '%s'.", service.ControllerName))
		}
	}(cmd, wg)
	return cmd
}

func RestartDeployment(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Restart Deployment", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Restarting Deployment '%s'.", service.ControllerName))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespace.Name)

		newController, err := CreateControllerConfiguration(namespace, service, false, deploymentClient, createDeploymentHandler)
		if err != nil {
			log.Errorf("error: %s", err.Error())
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
			cmd.Success(fmt.Sprintf("Restart Deployment '%s'.", service.ControllerName))
		}
	}(cmd, wg)
	return cmd
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
		newDeployment = *defaultDeployment
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
		spec.Template.Spec.Containers = []core.Container{}
	}
	for index, container := range service.Containers {
		if len(spec.Template.Spec.Containers) <= index {
			spec.Template.Spec.Containers = append(spec.Template.Spec.Containers, core.Container{})
		}

		// ImagePullPolicy
		if container.KubernetesLimits.ImagePullPolicy != "" {
			spec.Template.Spec.Containers[index].ImagePullPolicy = core.PullPolicy(container.KubernetesLimits.ImagePullPolicy)
		} else {
			spec.Template.Spec.Containers[index].ImagePullPolicy = core.PullAlways
		}

		// PORTS
		var internalHttpPort *int
		if len(container.Ports) > 0 {
			for _, port := range container.Ports {
				if port.PortType == dtos.PortTypeHTTPS {
					tmp := int(port.InternalPort)
					internalHttpPort = &tmp
				}
			}
		}

		// PROBES
		if !container.KubernetesLimits.ProbesOn {
			spec.Template.Spec.Containers[index].StartupProbe = nil
			spec.Template.Spec.Containers[index].LivenessProbe = nil
			spec.Template.Spec.Containers[index].ReadinessProbe = nil
		} else if internalHttpPort != nil {
			spec.Template.Spec.Containers[index].StartupProbe.HTTPGet.Port = intstr.FromInt(*internalHttpPort)
			spec.Template.Spec.Containers[index].LivenessProbe.HTTPGet.Port = intstr.FromInt(*internalHttpPort)
			spec.Template.Spec.Containers[index].ReadinessProbe.HTTPGet.Port = intstr.FromInt(*internalHttpPort)
		}
	}

	return objectMeta, &SpecDeployment{spec, previousSpec}, &newDeployment, nil
}

func SetDeploymentImage(job *structs.Job, namespaceName string, controllerName string, containerName string, imageName string, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Set Image '%s'", imageName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Set Image in Deployment '%s'.", controllerName))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespaceName)
		deploymentToUpdate, err := deploymentClient.Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("SetImage ERROR: %s", err.Error()))
			return
		}

		// SET NEW IMAGE
		for index, container := range deploymentToUpdate.Spec.Template.Spec.Containers {
			if container.Name == containerName {
				deploymentToUpdate.Spec.Template.Spec.Containers[index].Image = imageName
			}
		}
		deploymentToUpdate.Spec.Paused = false

		_, err = deploymentClient.Update(context.TODO(), deploymentToUpdate, metav1.UpdateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("SetImage ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Set new image in Deployment '%s'.", controllerName))
		}
	}(cmd, wg)
	return cmd
}

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

func WatchDeployments() {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		log.Fatalf("Error creating provider for watcher. Cannot continue because it is vital: %s", err.Error())
		return
	}

	// Retry watching resources with exponential backoff in case of failures
	retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}, apierrors.IsServiceUnavailable, func() error {
		return watchDeployments(provider, "deployments")
	})

	// Wait forever
	select {}
}

func watchDeployments(provider *punq.KubeProvider, kindName string) error {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			castedObj := obj.(*v1.Deployment)
			castedObj.Kind = "Deployment"
			castedObj.APIVersion = "apps/v1"
			iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			castedObj := newObj.(*v1.Deployment)
			castedObj.Kind = "Deployment"
			castedObj.APIVersion = "apps/v1"
			iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		DeleteFunc: func(obj interface{}) {
			castedObj := obj.(*v1.Deployment)
			castedObj.Kind = "Deployment"
			castedObj.APIVersion = "apps/v1"
			iacmanager.DeleteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, obj)
		},
	}
	listWatch := cache.NewListWatchFromClient(
		provider.ClientSet.AppsV1().RESTClient(),
		kindName,
		v1Core.NamespaceAll,
		fields.Nothing(),
	)
	resourceInformer := cache.NewSharedInformer(listWatch, &v1.Deployment{}, 0)
	resourceInformer.AddEventHandler(handler)

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
