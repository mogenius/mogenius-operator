package services

import (
	"encoding/json"
	"errors"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sort"
	"strings"
	"sync"
	"time"

	"context"
	"fmt"

	punq "github.com/mogenius/punq/kubernetes"
	log "github.com/sirupsen/logrus"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Due to issues importing the library, the following constants are copied from the library
var (
	// ErrImagePullBackOff - Container image pull failed, kubelet is backing off image pull
	ErrImagePullBackOff = errors.New("ImagePullBackOff")
	// ErrImageInspect - Unable to inspect image
	ErrImageInspect = errors.New("ImageInspectError")
	// ErrImagePull - General image pull error
	ErrImagePull = errors.New("ErrImagePull")
	// ErrImageNeverPull - Required Image is absent on host and PullPolicy is NeverPullImage
	ErrImageNeverPull = errors.New("ErrImageNeverPull")
	// ErrInvalidImageName - Unable to parse the image name.
	ErrInvalidImageName = errors.New("InvalidImageName")

	//
	ErrCrashLoopBackOff = errors.New("CrashLoopBackOff")
	// ErrContainerNotFound returned when a container in the given pod with the
	// given container name was not found, amongst those managed by the kubelet.
	ErrContainerNotFound = errors.New("no matching container")
	// ErrRunContainer returned when runtime failed to start any of pod's container.
	ErrRunContainer = errors.New("RunContainerError")
	// ErrKillContainer returned when runtime failed to kill any of pod's containers.
	ErrKillContainer = errors.New("KillContainerError")
	// ErrCreatePodSandbox returned when runtime failed to create a sandbox for pod.
	ErrCreatePodSandbox = errors.New("CreatePodSandboxError")
	// ErrConfigPodSandbox returned when runetime failed to get pod sandbox config from pod.
	ErrConfigPodSandbox = errors.New("ConfigPodSandboxError")
	// ErrKillPodSandbox returned when runtime failed to stop pod's sandbox.
	ErrKillPodSandbox = errors.New("KillPodSandboxError")

	// ErrRegistryUnavailable - Get http error on the PullImage RPC call.
	ErrRegistryUnavailable = errors.New("RegistryUnavailable")
	// ErrSignatureValidationFailed - Unable to validate the image signature on the PullImage RPC call.
	ErrSignatureValidationFailed = errors.New("SignatureValidationFailed")

	// ErrCreateContainerConfig - failed to create container config
	ErrCreateContainerConfig = errors.New("CreateContainerConfigError")
	// ErrPreCreateHook - failed to execute PreCreateHook
	ErrPreCreateHook = errors.New("PreCreateHookError")
	// ErrCreateContainer - failed to create container
	ErrCreateContainer = errors.New("CreateContainerError")
	// ErrPreStartHook - failed to execute PreStartHook
	ErrPreStartHook = errors.New("PreStartHookError")
	// ErrPostStartHook - failed to execute PostStartHook
	ErrPostStartHook = errors.New("PostStartHookError")

	//
	PodInitializing   = "PodInitializing"
	ContainerCreating = "ContainerCreating"
)

type ServiceStatusRequest struct {
	Namespace      string `json:"namespace" validate:"required"`
	ControllerName string `json:"controllerName" validate:"required"`
	Controller     string `json:"controller" validate:"required"`
}

func ServiceStatusRequestExample() ServiceStatusRequest {
	return ServiceStatusRequest{
		Namespace:      "YOUR-NAMESPACE",
		ControllerName: "YOUR-SERVICE-NAME",
		Controller:     Deployment.String(),
	}
}

// BEGIN new status and messages

type ServiceStatusKindType string

const (
	ServiceStatusKindTypeBuildJob    ServiceStatusKindType = "BuildJob"
	ServiceStatusKindTypeDeployment  ServiceStatusKindType = "Deployment"
	ServiceStatusKindTypeReplicaSet  ServiceStatusKindType = "ReplicaSet"
	ServiceStatusKindTypeStatefulSet ServiceStatusKindType = "StatefulSet"
	ServiceStatusKindTypeDaemonSet   ServiceStatusKindType = "DaemonSet"
	ServiceStatusKindTypeCronJob     ServiceStatusKindType = "CronJob"
	ServiceStatusKindTypeJob         ServiceStatusKindType = "Job"
	ServiceStatusKindTypePod         ServiceStatusKindType = "Pod"
	ServiceStatusKindTypeContainer   ServiceStatusKindType = "Container"
	ServiceStatusKindTypeUnkown      ServiceStatusKindType = "Unkown"
)

func NewServiceStatusKindType(serviceStatusKindType string) ServiceStatusKindType {
	switch serviceStatusKindType {
	case string(ServiceStatusKindTypeBuildJob):
		return ServiceStatusKindTypeBuildJob
	case string(ServiceStatusKindTypeDeployment):
		return ServiceStatusKindTypeDeployment
	case string(ServiceStatusKindTypeReplicaSet):
		return ServiceStatusKindTypeReplicaSet
	case string(ServiceStatusKindTypeStatefulSet):
		return ServiceStatusKindTypeStatefulSet
	case string(ServiceStatusKindTypeDaemonSet):
		return ServiceStatusKindTypeDaemonSet
	case string(ServiceStatusKindTypeCronJob):
		return ServiceStatusKindTypeCronJob
	case string(ServiceStatusKindTypeJob):
		return ServiceStatusKindTypeJob
	case string(ServiceStatusKindTypePod):
		return ServiceStatusKindTypePod
	case string(ServiceStatusKindTypeContainer):
		return ServiceStatusKindTypeContainer
	default:
		return ServiceStatusKindTypeUnkown
	}
}

type ServiceStatusType string

const (
	ServiceStatusTypePending ServiceStatusType = "PENDING"
	ServiceStatusTypeSuccess ServiceStatusType = "SUCCESS"
	ServiceStatusTypeWarning ServiceStatusType = "WARNING"
	ServiceStatusTypeError   ServiceStatusType = "ERROR"
	ServiceStatusTypeUnkown  ServiceStatusType = "UNKOWN"
)

type ServiceStatusMessageType string

const (
	ServiceStatusMessageTypeInfo    ServiceStatusMessageType = "INFO"
	ServiceStatusMessageTypeSuccess ServiceStatusMessageType = "SUCCESS"
	ServiceStatusMessageTypeError   ServiceStatusMessageType = "ERROR"
	ServiceStatusMessageTypeWarning ServiceStatusMessageType = "WARNING"
)

type ServiceStatusMessage struct {
	Type    ServiceStatusMessageType `json:"type"`
	Message string                   `json:"message"`
}

type ServiceStatusItem struct {
	Kind      ServiceStatusKindType  `json:"kind"`
	Name      string                 `json:"name"`
	Namespace string                 `json:"namespace"`
	OwnerName string                 `json:"ownerName,omitempty"`
	OwnerKind ServiceStatusKindType  `json:"ownerKind,omitempty"`
	Status    ServiceStatusType      `json:"status,omitempty"`
	Messages  []ServiceStatusMessage `json:"messages,omitempty"`
}

type ServiceStatusResponse struct {
	Items         []ServiceStatusItem    `json:"items"`
	SwitchedOn    bool                   `json:"switchedOn"`
	HasPods       bool                   `json:"hasPods"`
	HasContainers bool                   `json:"hasContainers"`
	HasDeployment bool                   `json:"hasDeployment"`
	HasCronJob    bool                   `json:"hasCronJob"`
	HasJob        bool                   `json:"hasJob"`
	HasBuild      bool                   `json:"hasBuild"`
	Warnings      []ServiceStatusMessage `json:"warnings,omitempty"`
}

// Process the status items and return the response
func ProcessServiceStatusResponse(r []ResourceItem) ServiceStatusResponse {
	s := ServiceStatusResponse{}

	for _, item := range r {
		newItem := NewServiceStatusItem(item, &s)
		s.Items = append(s.Items, newItem)

		switch item.Kind {
		case string(ServiceStatusKindTypeBuildJob):
			s.HasBuild = true
		case string(ServiceStatusKindTypeDeployment):
			s.HasDeployment = true
		case string(ServiceStatusKindTypeReplicaSet), string(ServiceStatusKindTypeStatefulSet), string(ServiceStatusKindTypeDaemonSet):
		case string(ServiceStatusKindTypeJob):
			s.HasJob = true
		case string(ServiceStatusKindTypeCronJob):
			s.HasCronJob = true
		case string(ServiceStatusKindTypePod):
			s.HasPods = true
		case string(ServiceStatusKindTypeContainer):
			s.HasContainers = true
		}

		// move warning messages into the response
		for _, message := range newItem.Messages {
			if message.Type == ServiceStatusMessageTypeWarning {
				s.Warnings = append(s.Warnings, message)
			}
		}
	}

	return s
}

func NewServiceStatusItem(item ResourceItem, s *ServiceStatusResponse) ServiceStatusItem {
	newItem := ServiceStatusItem{
		Kind:      NewServiceStatusKindType(item.Kind),
		Name:      item.Name,
		Namespace: item.Namespace,
		OwnerName: item.OwnerName,
	}

	if NewServiceStatusKindType(item.OwnerKind) != ServiceStatusKindTypeUnkown {
		newItem.OwnerKind = NewServiceStatusKindType(item.OwnerKind)
	}

	// Convert events to messages
	if item.Events != nil {
		for _, event := range item.Events {
			var messageType ServiceStatusMessageType
			if event.Type == "Warning" {
				messageType = ServiceStatusMessageTypeWarning
			} else {
				messageType = ServiceStatusMessageTypeInfo
			}

			newItem.Messages = append(newItem.Messages, ServiceStatusMessage{
				Type:    messageType,
				Message: event.Message,
			})
		}
	}

	// Set status
	if item.StatusObject != nil {
		switch item.Kind {
		case string(ServiceStatusKindTypeBuildJob):
			if status := item.BuildJobStatus(); status != nil {
				newItem.Status = *status
			}
		case string(ServiceStatusKindTypeCronJob):
			if status, switchedOn := item.CronJobStatus(); status != nil {
				newItem.Status = *status
				if s != nil {
					s.SwitchedOn = switchedOn
				}
			}
		case string(ServiceStatusKindTypeJob):
			if status := item.JobStatus(); status != nil {
				newItem.Status = *status
			}
		case string(ServiceStatusKindTypeDeployment):
			if status, switchedOn := item.DeploymentStatus(); status != nil {
				newItem.Status = *status
				if s != nil {
					s.SwitchedOn = switchedOn
				}
			}
		case string(ServiceStatusKindTypePod):
			status, messages := item.PodStatus()
			if status != nil {
				newItem.Status = *status
			}
			if messages != nil {
				newItem.Messages = append(newItem.Messages, messages...)
			}
		case string(ServiceStatusKindTypeContainer):
			if status := item.ContainerStatus(); status != nil {
				newItem.Status = *status
			}
		}
	}

	return newItem
}

func (r *ResourceItem) ContainerStatus() *ServiceStatusType {
	if r.StatusObject != nil {
		if containerStatus, ok := r.StatusObject.(corev1.ContainerStatus); ok {

			if containerStatus.State.Terminated != nil {
				status := ServiceStatusTypeError
				return &status
			}

			if containerStatus.State.Waiting != nil {
				switch reason := containerStatus.State.Waiting.Reason; reason {
				case ErrImagePull.Error(), ErrImagePullBackOff.Error(), ErrImageInspect.Error(), ErrImageNeverPull.Error(), ErrInvalidImageName.Error():
					status := ServiceStatusTypeError
					return &status
				case ErrCrashLoopBackOff.Error(), ErrContainerNotFound.Error(), ErrRunContainer.Error(), ErrKillContainer.Error(), ErrCreatePodSandbox.Error(), ErrConfigPodSandbox.Error(), ErrKillPodSandbox.Error():
					status := ServiceStatusTypeError
					return &status
				case ErrRegistryUnavailable.Error(), ErrSignatureValidationFailed.Error():
					status := ServiceStatusTypeError
					return &status
				case ErrCreateContainerConfig.Error(), ErrPreCreateHook.Error(), ErrCreateContainer.Error(), ErrPreStartHook.Error(), ErrPostStartHook.Error():
					status := ServiceStatusTypeError
					return &status
				case PodInitializing, ContainerCreating:
					status := ServiceStatusTypePending
					return &status
				default:
					log.Warningf("Unhandled status - Container '%s' waiting. %s: %s.", containerStatus.Name, containerStatus.State.Waiting.Reason, containerStatus.State.Waiting.Message)
				}

				status := ServiceStatusTypePending
				return &status
			}

			// readiness probe OR running without readiness probe, default is true
			ready := containerStatus.Ready
			// startup probe OR running without startup probe, nil considered as false
			started := false
			if containerStatus.Started != nil {
				started = *containerStatus.Started
			}

			if started && ready {
				status := ServiceStatusTypeSuccess
				return &status
			}

			status := ServiceStatusTypeWarning
			return &status
		}
	}
	return nil
}

func (r *ResourceItem) PodStatus() (*ServiceStatusType, []ServiceStatusMessage) {
	if r.StatusObject != nil {
		if podStatus, ok := r.StatusObject.(corev1.PodStatus); ok {
			// readiness probe
			ready := true
			for _, containerStatus := range podStatus.ContainerStatuses {
				ready = ready && containerStatus.Ready
			}
			// startup probe
			started := true
			for _, containerStatus := range podStatus.ContainerStatuses {
				if containerStatus.Started == nil {
					started = false
				} else {
					started = started && *containerStatus.Started
				}
			}

			// create container messages if not running
			var messages []ServiceStatusMessage
			for _, containerStatus := range podStatus.ContainerStatuses {
				if containerStatus.State.Terminated != nil {
					messages = append(messages, ServiceStatusMessage{
						Type:    ServiceStatusMessageTypeWarning,
						Message: fmt.Sprintf("Container '%s' terminated with exit code (%d). %s: %s.", containerStatus.Name, containerStatus.State.Terminated.ExitCode, containerStatus.State.Terminated.Reason, containerStatus.State.Terminated.Message),
					})
				}
				if containerStatus.State.Waiting != nil {
					messages = append(messages, ServiceStatusMessage{
						Type:    ServiceStatusMessageTypeWarning,
						Message: fmt.Sprintf("Container '%s' waiting. %s: %s.", containerStatus.Name, containerStatus.State.Waiting.Reason, containerStatus.State.Waiting.Message),
					})
				}
			}

			if podStatus.Reason != "" && podStatus.Message != "" {
				messages = append(messages, ServiceStatusMessage{
					Type:    ServiceStatusMessageTypeInfo,
					Message: fmt.Sprintf("Pod '%s' information. %s: %s.", r.Name, podStatus.Reason, podStatus.Message),
				})
			}

			switch podStatus.Phase {
			case corev1.PodRunning:
				if started && ready {
					status := ServiceStatusTypeSuccess
					return &status, messages
				}
				status := ServiceStatusTypeWarning
				return &status, messages
			case corev1.PodSucceeded:
				status := ServiceStatusTypeSuccess
				return &status, messages
			case corev1.PodPending:
				// if !started || !ready {
				// 	status := ServiceStatusTypeWarning
				// 	return &status, messages
				// }

				// container waiting status only considerd when pod is in pending state
				// for _, containerStatus := range podStatus.ContainerStatuses {
				// 	if containerStatus.State.Waiting != nil {
				// 		switch reason := containerStatus.State.Waiting.Reason; reason {
				// 		case ErrImagePull.Error(), ErrImagePullBackOff.Error(), ErrImageInspect.Error(), ErrImageNeverPull.Error(), ErrInvalidImageName.Error():
				// 			status := ServiceStatusTypeWarning
				// 			return &status, messages
				// 		}
				// 	}
				// }

				status := ServiceStatusTypePending
				return &status, messages
			case corev1.PodFailed:
				status := ServiceStatusTypeError
				return &status, messages
			default:
				status := ServiceStatusTypeUnkown
				return &status, messages
			}
		}
	}

	return nil, nil
}

func (r *ResourceItem) BuildJobStatus() *ServiceStatusType {
	// When StatusObject is not nil, then type casting to structs.BuildJob
	var success *bool
	// var messages []ServiceStatusMessage

	if r.StatusObject != nil {
		if buildJobInfo, ok := r.StatusObject.(structs.BuildJobInfo); ok {
			for _, task := range buildJobInfo.Tasks {
				switch task.State {
				case structs.JobStateStarted, structs.JobStatePending:
					status := ServiceStatusTypePending
					return &status
				case structs.JobStateSucceeded:
					if success == nil {
						success = new(bool)
						*success = true
					}
				case structs.JobStateFailed, structs.JobStateCanceled, structs.JobStateTimeout:
					// messages = append(messages, ServiceStatusMessage{
					// 	Type:    ServiceStatusMessageTypeError,
					// 	Message: fmt.Sprintf("BuildId '%d', step '%s' failed with state '%s'. Result:\n\n%s", buildJobInfo.BuildId, task.Prefix, task.State, task.Result),
					// })

					status := ServiceStatusTypeError
					return &status
				default:
					status := ServiceStatusTypeUnkown
					return &status
				}
			}
		}
	}

	if success != nil {
		status := ServiceStatusTypeSuccess
		return &status
	}

	return nil
}

func (r *ResourceItem) CronJobStatus() (*ServiceStatusType, bool) {
	if r.StatusObject != nil {
		if cronJob, ok := r.StatusObject.(CronJobStatus); ok {
			switchedOn := !cronJob.Suspend

			if cronJob.Image != "" && !strings.Contains(cronJob.Image, utils.IMAGE_PLACEHOLDER) && !cronJob.Suspend {
				status := ServiceStatusTypeSuccess
				return &status, switchedOn
			}
			if strings.Contains(cronJob.Image, utils.IMAGE_PLACEHOLDER) && !cronJob.Suspend {
				status := ServiceStatusTypeError
				return &status, switchedOn
			}
			if strings.Contains(cronJob.Image, utils.IMAGE_PLACEHOLDER) && cronJob.Suspend {
				status := ServiceStatusTypeUnkown
				return &status, switchedOn
			}

			status := ServiceStatusTypeSuccess
			return &status, switchedOn
		}
	}
	return nil, false
}

func (r *ResourceItem) JobStatus() *ServiceStatusType {
	if r.StatusObject != nil {
		if jobStatus, ok := r.StatusObject.(batchv1.JobStatus); ok {
			if jobStatus.Failed > 0 {
				status := ServiceStatusTypeError
				return &status
			}

			active := jobStatus.Active
			ready := *jobStatus.Ready
			if active != ready {
				status := ServiceStatusTypePending
				return &status
			}
			if active == ready || jobStatus.Succeeded > 0 {
				status := ServiceStatusTypeSuccess
				return &status
			}

			status := ServiceStatusTypeUnkown
			return &status
		}
	}
	return nil
}

func (r *ResourceItem) DeploymentStatus() (*ServiceStatusType, bool) {
	if r.StatusObject != nil {
		if deploymentStatus, ok := r.StatusObject.(DeploymentStatus); ok {
			if originalDeploymentStatus, ok := deploymentStatus.StatusObject.(appsv1.DeploymentStatus); ok {

				switchedOn := deploymentStatus.Replicas > 0

				// isHappy; if replicas == availableReplicas
				isHappy := deploymentStatus.Replicas == originalDeploymentStatus.AvailableReplicas
				if !isHappy {
					status := ServiceStatusTypeSuccess
					return &status, switchedOn
				}

				// placeholder image
				if strings.Contains(deploymentStatus.Image, utils.IMAGE_PLACEHOLDER) {
					status := ServiceStatusTypePending
					return &status, switchedOn
				}

				conditions := originalDeploymentStatus.Conditions

				// find condition type Available
				for _, condition := range conditions {
					if condition.Type == appsv1.DeploymentAvailable {
						if condition.Status == corev1.ConditionTrue {
							status := ServiceStatusTypeSuccess
							return &status, switchedOn
						}
					}
				}

				// find condition type ReplicaFailure
				for _, condition := range conditions {
					if condition.Type == appsv1.DeploymentReplicaFailure {
						if condition.Status == corev1.ConditionTrue {
							status := ServiceStatusTypeError
							return &status, switchedOn
						}
					}
				}

				// find condition type Progressing
				for _, condition := range conditions {
					if condition.Type == appsv1.DeploymentProgressing {
						if condition.Status == corev1.ConditionTrue {
							status := ServiceStatusTypePending
							return &status, switchedOn
						}
					}
				}

				if originalDeploymentStatus.UnavailableReplicas > 0 {
					status := ServiceStatusTypeWarning
					return &status, switchedOn
				}

				status := ServiceStatusTypeUnkown
				return &status, switchedOn
			}
		}
	}
	return nil, false
}

type CronJobStatus struct {
	Suspend      bool        `json:"suspend,omitempty"`
	Image        string      `json:"image,omitempty"`
	StatusObject interface{} `json:"status,omitempty"`
}

type DeploymentStatus struct {
	Replicas     int32       `json:"replicas,omitempty"`
	Paused       bool        `json:"paused,omitempty"`
	Image        string      `json:"image,omitempty"`
	StatusObject interface{} `json:"status,omitempty"`
}

// END new status and messages

type ResourceItem struct {
	Kind         string         `json:"kind"`
	Name         string         `json:"name"`
	Namespace    string         `json:"namespace"`
	OwnerName    string         `json:"ownerName,omitempty"`
	OwnerKind    string         `json:"ownerKind,omitempty"`
	StatusObject interface{}    `json:"statusObject,omitempty"`
	Events       []corev1.Event `json:"events,omitempty"`
}

func (item ResourceItem) String() string {
	return fmt.Sprintf("%s, %s, %s, %s, %s, %+v", item.Kind, item.Name, item.Namespace, item.OwnerKind, item.OwnerName, item.StatusObject)
}

type ResourceController int

// Keep the order, only add elements at end
const (
	Unkown ResourceController = iota
	Deployment
	ReplicaSet
	StatefulSet
	DaemonSet
	Job
	CronJob
)

// Keep the order with above structure...
//
//	otherwise everything will be messed up
func (ctrl ResourceController) String() string {
	return [...]string{"Unkown", "Deployment", "ReplicaSet", "StatefulSet", "DaemonSet", "Job", "CronJob"}[ctrl]
}

func NewResourceController(resourceController string) ResourceController {
	switch resourceController {
	case Deployment.String():
		return Deployment
	case ReplicaSet.String():
		return ReplicaSet
	case StatefulSet.String():
		return StatefulSet
	case DaemonSet.String():
		return DaemonSet
	case Job.String():
		return Job
	case CronJob.String():
		return CronJob
	default:
		return Unkown
	}
}

// Run a goroutine to fetch k8s events then push them into the channel before timeout
func requestEvents(namespace string, ctx context.Context, wg *sync.WaitGroup, eventsChan chan<- []corev1.Event) {
	defer wg.Done()

	r := punq.AllK8sEvents(namespace, nil)

	var events []corev1.Event
	if r.Error != nil {
		log.Warningf("Warning fetching events: %s", r.Error)
		events = []corev1.Event{}
		eventsChan <- events
		return
	}

	if r.Result != nil {
		events = r.Result.([]corev1.Event)
	}

	// Push the events into the channel
	select {
	case <-ctx.Done():
		log.Debugf("go: timeout waiting for events")
		return
	case eventsChan <- events:
		//log.Debugf("go: push the events into the channel")
	}
}

func StatusService(r ServiceStatusRequest) interface{} {
	//log.Debugf("StatusService for (%s): %s %s", r.ControllerName, r.Namespace, r.Controller)

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		log.Warningf("Warningf: %s", err.Error())
		return nil
	}

	// Create a channel to receive an array of events
	eventsChan := make(chan []corev1.Event, 1)
	var wg sync.WaitGroup

	// Context with timeout to handle cancellation and timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	wg.Add(1)
	// Run a goroutine to fetch k8s events then push them into the channel before timeout
	go requestEvents(r.Namespace, ctx, &wg, eventsChan)

	go func() {
		wg.Wait()         // Wait for all goroutines to finish.
		close(eventsChan) // IMPORTANT!: Safely close channel after all sends are done.
	}()

	resourceItems := []ResourceItem{}
	resourceItems, err = kubernetesItems(r.Namespace, r.ControllerName, NewResourceController(r.Controller), provider.ClientSet, resourceItems)
	if err != nil {
		log.Warningf("Warning statusItems: %v", err)
	}

	resourceItems, err = buildItem(r.Namespace, r.ControllerName, resourceItems)
	if err != nil {
		log.Warningf("Warning buildItem: %v", err)
	}

	// Wait for the result from the channel or timeout
	select {
	case events, ok := <-eventsChan:
		if !ok {
			log.Warningf("Warning event channel closed.")
			break
		}

		// Sort events by lastTimestamp from newest to oldest
		sort.SliceStable(events, func(i, j int) bool {
			return events[i].LastTimestamp.Time.After(events[j].LastTimestamp.Time)
		})

		// Iterate events and add them to resourceItems
	EventLoop:
		for _, event := range events {
			for i, item := range resourceItems {
				if item.Name == event.InvolvedObject.Name && item.Namespace == event.InvolvedObject.Namespace {
					resourceItems[i].Events = append(resourceItems[i].Events, event)
					continue EventLoop
				}
			}
		}
	case <-ctx.Done():
		log.Warningf("Warning timeout waiting for events")
	}

	// Debug logs
	// jsonData, err := json.MarshalIndent(resourceItems, "", "  ")
	// if err != nil {
	// 	log.Warningf("Warning marshaling JSON: %v", err)
	// 	return nil
	// }
	// log.Debugf("JSON: %s", jsonData)

	// return resourceItems

	return ProcessServiceStatusResponse(resourceItems)
}

func kubernetesItems(namespace string, name string, resourceController ResourceController, clientset *kubernetes.Clientset, resourceItems []ResourceItem) ([]ResourceItem, error) {
	resourceInterface, err := controller(namespace, name, resourceController, clientset)
	if err != nil {
		log.Warningf("\nWarning fetching controller: %s\n", err)
		return resourceItems, err
	}

	metaName, metaNamespace, kind, references, labelSelector, object := status(resourceInterface)
	resourceItems = controllerItem(metaName, kind, metaNamespace, resourceController.String(), references, object, resourceItems)

	pods, err := pods(namespace, labelSelector, clientset)
	if err != nil {
		log.Warningf("\nWarning fetching pods: %s\n", err)
		return resourceItems, err
	}

	for _, pod := range pods.Items {
		resourceItems = containerItems(pod, resourceItems)
		resourceItems = podItem(pod, resourceItems)
		// Owner reference kind and name
		if len(pod.OwnerReferences) > 0 {
			for _, ownerRef := range pod.OwnerReferences {
				// only controller parents
				if *ownerRef.Controller {
					resourceItems = recursiveOwnerRef(pod.Namespace, ownerRef, clientset, resourceItems)
				}
			}
		}
	}

	return resourceItems, nil
}

func controller(namespace string, controllerName string, resourceController ResourceController, clientset *kubernetes.Clientset) (interface{}, error) {
	var err error
	var resourceInterface interface{}

	switch resourceController {
	case Deployment:
		resourceInterface, err = clientset.AppsV1().Deployments(namespace).Get(context.TODO(), controllerName, metav1.GetOptions{})
	case ReplicaSet:
		resourceInterface, err = clientset.AppsV1().ReplicaSets(namespace).Get(context.TODO(), controllerName, metav1.GetOptions{})
	case StatefulSet:
		resourceInterface, err = clientset.AppsV1().StatefulSets(namespace).Get(context.TODO(), controllerName, metav1.GetOptions{})
	case DaemonSet:
		resourceInterface, err = clientset.AppsV1().DaemonSets(namespace).Get(context.TODO(), controllerName, metav1.GetOptions{})
	case Job:
		resourceInterface, err = clientset.BatchV1().Jobs(namespace).Get(context.TODO(), controllerName, metav1.GetOptions{})
	case CronJob:
		resourceInterface, err = clientset.BatchV1().CronJobs(namespace).Get(context.TODO(), controllerName, metav1.GetOptions{})
	}

	if err != nil {
		log.Warningf("\nWarning fetching resources %s, ns: %s, name: %s, err: %s\n", resourceController.String(), namespace, controllerName, err)
		return nil, err
	}

	return resourceInterface, nil
}

func pods(namespace string, labelSelector *metav1.LabelSelector, clientset *kubernetes.Clientset) (*corev1.PodList, error) {
	if labelSelector != nil {
		selector := metav1.FormatLabelSelector(labelSelector)
		pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: selector,
			FieldSelector: "status.phase!=Succeeded",
		})

		if err != nil {
			return nil, err
		}

		return pods, nil
	}

	return &corev1.PodList{}, nil
}

func buildItem(namespace, name string, resourceItems []ResourceItem) ([]ResourceItem, error) {
	lastJob := LastBuildForNamespaceAndControllerName(namespace, name)
	if lastJob.IsEmpty() {
		return resourceItems, nil
	}

	item := &ResourceItem{
		Kind:         "BuildJob",
		Name:         name,
		Namespace:    namespace,
		OwnerName:    "",
		OwnerKind:    "",
		StatusObject: lastJob,
	}

	resourceItems = append(resourceItems, *item)

	return resourceItems, nil
}

func containerItems(pod corev1.Pod, resourceItems []ResourceItem) []ResourceItem {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		item := &ResourceItem{
			Kind:         "Container",
			Name:         containerStatus.Name,
			Namespace:    pod.Namespace,
			OwnerName:    pod.Name,
			OwnerKind:    "Pod",
			StatusObject: containerStatus,
		}
		resourceItems = append(resourceItems, *item)
	}

	return resourceItems
}

func controllerItem(name, kind, namespace, resourceController string, references []metav1.OwnerReference, object interface{}, resourceItems []ResourceItem) []ResourceItem {
	if len(references) > 0 {
		for _, parentRef := range references {
			if *parentRef.Controller {
				item := &ResourceItem{
					Kind:         kind,
					Name:         name,
					Namespace:    namespace,
					OwnerName:    parentRef.Name,
					OwnerKind:    parentRef.Kind,
					StatusObject: object,
				}
				resourceItems = append(resourceItems, *item)

				break
			}
		}
	} else {
		item := &ResourceItem{
			Kind:         kind,
			Name:         name,
			Namespace:    namespace,
			OwnerName:    "",
			OwnerKind:    "",
			StatusObject: object,
		}
		resourceItems = append(resourceItems, *item)
	}

	return resourceItems
}

func podItem(pod corev1.Pod, resourceItems []ResourceItem) []ResourceItem {
	for _, ownerRef := range pod.OwnerReferences {
		if *ownerRef.Controller {
			item := &ResourceItem{
				Kind:         "Pod",
				Name:         pod.Name,
				Namespace:    pod.Namespace,
				OwnerName:    ownerRef.Name,
				OwnerKind:    ownerRef.Kind,
				StatusObject: pod.Status,
			}
			resourceItems = append(resourceItems, *item)
		}
	}

	return resourceItems
}

func recursiveOwnerRef(namespace string, ownerRef metav1.OwnerReference, clientset *kubernetes.Clientset, resourceItems []ResourceItem) []ResourceItem {
	// Skip already included resourceItems
	for _, item := range resourceItems {
		if item.Kind == ownerRef.Kind {
			return resourceItems
		}
	}

	// Fetch next k8s controller
	resourceInterface, err := controller(namespace, ownerRef.Name, NewResourceController(ownerRef.Kind), clientset)
	if err != nil {
		log.Warningf("\nWarning fetching resources: %s\n", err)
		return resourceItems
	}

	// Extract status data from controller
	name, namespace, kind, references, _, object := status(resourceInterface)
	resourceItems = controllerItem(name, kind, namespace, NewResourceController(kind).String(), references, object, resourceItems)

	// Fetch next parent controller
	if len(references) > 0 {
		for _, parentRef := range references {
			if *parentRef.Controller {
				return recursiveOwnerRef(namespace, parentRef, clientset, resourceItems)
			}
		}
	}

	return resourceItems

}

func status(resource interface{}) (string, string, string, []metav1.OwnerReference, *metav1.LabelSelector, interface{}) {
	switch r := resource.(type) {
	case *appsv1.Deployment:
		status := DeploymentStatus{
			Replicas:     *r.Spec.Replicas,
			Paused:       r.Spec.Paused,
			Image:        r.Spec.Template.Spec.Containers[0].Image,
			StatusObject: r.Status,
		}
		return r.ObjectMeta.Name, r.ObjectMeta.Namespace, Deployment.String(), r.OwnerReferences, r.Spec.Selector, status
	case *appsv1.ReplicaSet:
		return r.ObjectMeta.Name, r.ObjectMeta.Namespace, ReplicaSet.String(), r.OwnerReferences, r.Spec.Selector, r.Status
	case *appsv1.StatefulSet:
		return r.ObjectMeta.Name, r.ObjectMeta.Namespace, StatefulSet.String(), r.OwnerReferences, r.Spec.Selector, r.Status
	case *appsv1.DaemonSet:
		return r.ObjectMeta.Name, r.ObjectMeta.Namespace, DaemonSet.String(), r.OwnerReferences, r.Spec.Selector, r.Status
	case *batchv1.Job:
		var labelSelector = metav1.LabelSelector{
			MatchLabels: map[string]string{
				"ns":  r.Spec.Template.ObjectMeta.Labels["ns"],
				"app": r.Spec.Template.ObjectMeta.Labels["app"],
			},
		}

		return r.ObjectMeta.Name, r.ObjectMeta.Namespace, Job.String(), r.OwnerReferences, &labelSelector, r.Status
	case *batchv1.CronJob:
		status := CronJobStatus{
			Suspend:      *r.Spec.Suspend,
			Image:        r.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image,
			StatusObject: r.Status,
		}

		var labelSelector = metav1.LabelSelector{
			MatchLabels: map[string]string{
				"ns":  r.ObjectMeta.Labels["ns"],
				"app": r.ObjectMeta.Labels["app"],
			},
		}

		return r.ObjectMeta.Name, r.ObjectMeta.Namespace, CronJob.String(), r.OwnerReferences, &labelSelector, status
	default:
		return "", "", Unkown.String(), []metav1.OwnerReference{}, nil, nil
	}
}
