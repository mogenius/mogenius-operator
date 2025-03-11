package services

import (
	"errors"
	"fmt"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/utils"
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	GitRepository  bool   `json:"gitRepository"`
}

// BEGIN new status and messages

type ServiceStatusKindType string

const (
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
	// Additional status information for different types which are omited if empty
	CreatedAt       *metav1.Time      `json:"createdAt,omitempty"`
	ContainerStatus *XContainerStatus `json:"containerStatus,omitempty"`
	// PodStatus       XPodStatus       `json:"podStatus,omitempty"`
}

type ServiceStatusObject struct {
	// Container status with restart count and creation or start time
	ContainerStatus XContainerStatus `json:"containerStatus,omitempty"`
	// Pod status with creation time
	PodStatus XPodStatus `json:"podStatus,omitempty"`
}

// Xustom container status
type XContainerStatus struct {
	RestartCount int32        `json:"restartCount,omitempty"`
	CreatedAt    *metav1.Time `json:"createdAt,omitempty"`
}

// Xustom pod status
type XPodStatus struct {
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`
}

type ServiceStatusResponse struct {
	Items         []ServiceStatusItem    `json:"items"`
	SwitchedOn    bool                   `json:"switchedOn"`
	HasPods       bool                   `json:"hasPods"`
	HasContainers bool                   `json:"hasContainers"`
	HasDeployment bool                   `json:"hasDeployment"`
	HasCronJob    bool                   `json:"hasCronJob"`
	HasJob        bool                   `json:"hasJob"`
	Warnings      []ServiceStatusMessage `json:"warnings,omitempty"`
}

// Process the status items and return the response
func ProcessServiceStatusResponse(r []ResourceItem) ServiceStatusResponse {
	s := ServiceStatusResponse{}

	for _, item := range r {
		newItem := NewServiceStatusItem(item, &s)
		s.Items = append(s.Items, newItem)

		switch item.Kind {
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
			status, messages, statusObject := item.PodStatus()
			if status != nil {
				newItem.Status = *status
			}
			if statusObject != nil {
				// newItem.PodStatus = statusObject.PodStatus
				newItem.CreatedAt = statusObject.PodStatus.CreatedAt
			}
			if messages != nil {
				newItem.Messages = append(newItem.Messages, messages...)
			}
		case string(ServiceStatusKindTypeContainer):
			status, statusObject := item.ContainerStatus()
			if status != nil {
				newItem.Status = *status
			}
			if statusObject != nil {
				// newItem.StatusObject = *statusObject
				newItem.CreatedAt = statusObject.ContainerStatus.CreatedAt
				newItem.ContainerStatus = &statusObject.ContainerStatus
			}

		}
	}

	return newItem
}

func (r *ResourceItem) ContainerStatus() (*ServiceStatusType, *ServiceStatusObject) {
	if r.StatusObject != nil {
		if containerStatus, ok := r.StatusObject.(corev1.ContainerStatus); ok {

			// retsart count & start time
			statusObject := ServiceStatusObject{
				ContainerStatus: XContainerStatus{
					RestartCount: containerStatus.RestartCount,
				},
			}
			if containerStatus.State.Running != nil {
				createdAt := &containerStatus.State.Running.StartedAt
				statusObject.ContainerStatus.CreatedAt = createdAt
			}

			if containerStatus.State.Terminated != nil {
				status := ServiceStatusTypeError
				return &status, &statusObject
			}

			if containerStatus.State.Waiting != nil {
				switch reason := containerStatus.State.Waiting.Reason; reason {
				case ErrImagePull.Error(), ErrImagePullBackOff.Error(), ErrImageInspect.Error(), ErrImageNeverPull.Error(), ErrInvalidImageName.Error():
					status := ServiceStatusTypeError
					return &status, &statusObject
				case ErrCrashLoopBackOff.Error(), ErrContainerNotFound.Error(), ErrRunContainer.Error(), ErrKillContainer.Error(), ErrCreatePodSandbox.Error(), ErrConfigPodSandbox.Error(), ErrKillPodSandbox.Error():
					status := ServiceStatusTypeError
					return &status, &statusObject
				case ErrRegistryUnavailable.Error(), ErrSignatureValidationFailed.Error():
					status := ServiceStatusTypeError
					return &status, &statusObject
				case ErrCreateContainerConfig.Error(), ErrPreCreateHook.Error(), ErrCreateContainer.Error(), ErrPreStartHook.Error(), ErrPostStartHook.Error():
					status := ServiceStatusTypeError
					return &status, &statusObject
				case PodInitializing, ContainerCreating:
					status := ServiceStatusTypePending
					return &status, &statusObject
				default:
					serviceLogger.Warn("Unhandled status - Container waiting.", "containerName", containerStatus.Name, "waitingReason", containerStatus.State.Waiting.Reason, "waitingMessage", containerStatus.State.Waiting.Message)
				}

				status := ServiceStatusTypePending
				return &status, &statusObject
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
				return &status, &statusObject
			}

			status := ServiceStatusTypeWarning
			return &status, &statusObject
		}
	}
	return nil, nil
}

func (r *ResourceItem) PodStatus() (*ServiceStatusType, []ServiceStatusMessage, *ServiceStatusObject) {
	if r.StatusObject != nil {
		if podStatus, ok := r.StatusObject.(corev1.PodStatus); ok {

			var statusObject ServiceStatusObject
			if podStatus.StartTime != nil {
				statusObject = ServiceStatusObject{
					PodStatus: XPodStatus{
						CreatedAt: podStatus.StartTime,
					},
				}
			}

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
					return &status, messages, &statusObject
				}
				status := ServiceStatusTypeWarning
				return &status, messages, &statusObject
			case corev1.PodSucceeded:
				status := ServiceStatusTypeSuccess
				return &status, messages, &statusObject
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
				return &status, messages, &statusObject
			case corev1.PodFailed:
				status := ServiceStatusTypeError
				return &status, messages, &statusObject
			default:
				status := ServiceStatusTypeUnkown
				return &status, messages, &statusObject
			}
		}
	}

	return nil, nil, nil
}

func (r *ResourceItem) CronJobStatus() (*ServiceStatusType, bool) {
	if r.StatusObject != nil {
		if cronJob, ok := r.StatusObject.(CronJobStatus); ok {
			switchedOn := !cronJob.Suspend

			if cronJob.Image != "" && !cronJob.Suspend {
				status := ServiceStatusTypeSuccess
				return &status, switchedOn
			}
			if !cronJob.Suspend {
				status := ServiceStatusTypeError
				return &status, switchedOn
			}
			if cronJob.Suspend {
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

				conditions := originalDeploymentStatus.Conditions

				// condtitions are weighted in order of importance
				// if multiple conditions are true, the highest value is returned
				// if no conditions are true, continue
				//
				// DeploymentAvailable = 4
				// DeploymentReplicaFailure = 2
				// DeploymentProgressing = 1

				// create inline struct for weighted
				type WeightedCondition struct {
					Type   appsv1.DeploymentConditionType
					Weight int
					Status ServiceStatusType
				}

				// create slice of weighted conditions
				weightedConditions := []WeightedCondition{}

				for _, condition := range conditions {
					switch condition.Type {
					case appsv1.DeploymentAvailable:
						if condition.Status == corev1.ConditionTrue {
							weightedConditions = append(weightedConditions, WeightedCondition{appsv1.DeploymentAvailable, 4, ServiceStatusTypeSuccess})
						}
					case appsv1.DeploymentReplicaFailure:
						if condition.Status == corev1.ConditionTrue {
							weightedConditions = append(weightedConditions, WeightedCondition{appsv1.DeploymentReplicaFailure, 2, ServiceStatusTypeError})
						}
					case appsv1.DeploymentProgressing:
						if condition.Status == corev1.ConditionTrue {
							weightedConditions = append(weightedConditions, WeightedCondition{appsv1.DeploymentProgressing, 1, ServiceStatusTypePending})
						}
					}
				}

				// return the first condition
				if len(weightedConditions) > 0 {
					// sort conditions by weight
					sort.Slice(weightedConditions, func(i, j int) bool {
						return weightedConditions[i].Weight > weightedConditions[j].Weight
					})

					status := weightedConditions[0].Status
					return &status, switchedOn
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

var statusServiceDebounce = utils.NewDebounce("statusServiceDebounce", 1000*time.Millisecond, 300*time.Millisecond)

func StatusServiceDebounced(r ServiceStatusRequest) ServiceStatusResponse {
	key := fmt.Sprintf("%s-%s-%s", r.Namespace, r.ControllerName, r.Controller)
	result, _ := statusServiceDebounce.CallFn(key, func() (interface{}, error) {
		return statusService(r), nil
	})
	return result.(ServiceStatusResponse)
}

func statusService(r ServiceStatusRequest) ServiceStatusResponse {
	events, err := store.ListEvents(r.Namespace)
	if err != nil {
		serviceLogger.Warn("failed to fetch events", "error", err)
	}

	resourceItems, err := kubernetesItems(r.Namespace, r.ControllerName, NewResourceController(r.Controller))
	if err != nil {
		serviceLogger.Warn("failed to get statusItems", "error", err)
	}

	for _, event := range events {
		for i, item := range resourceItems {
			if item.Name == event.InvolvedObject.Name && item.Namespace == event.InvolvedObject.Namespace {
				resourceItems[i].Events = append(resourceItems[i].Events, event)
			}
		}
	}

	return ProcessServiceStatusResponse(resourceItems)
}

func kubernetesItems(namespace string, name string, resourceController ResourceController) ([]ResourceItem, error) {
	resourceItems := []ResourceItem{}
	resourceInterface, err := controller(namespace, name, resourceController)
	if err != nil {
		serviceLogger.Warn("failed to fetch controller", "error", err)
		return resourceItems, err
	}

	metaName, metaNamespace, kind, references, labelSelector, object := status(resourceInterface)
	resourceItems = controllerItem(metaName, kind, metaNamespace, resourceController.String(), references, object, resourceItems)

	// Fetch pods
	pods, err := store.ListPods(metaNamespace, metaName)
	if err != nil {
		serviceLogger.Warn("failed to fetch pods", "error", err)
		return resourceItems, err
	}
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodSucceeded {
			continue
		}
		// check if labels match
		if labelSelector != nil {
			if !labels.SelectorFromSet(labelSelector.MatchLabels).Matches(labels.Set(pod.Labels)) {
				continue
			}
		}

		resourceItems = containerItems(pod, resourceItems)
		resourceItems = podItem(pod, resourceItems)

		// Owner reference kind and name
		if len(pod.OwnerReferences) > 0 {
			for _, ownerRef := range pod.OwnerReferences {
				// only controller parents
				if *ownerRef.Controller {
					resourceItems = recursiveOwnerRef(pod.Namespace, ownerRef, resourceItems)
				}
			}
		}
	}

	return resourceItems, nil
}

func controller(namespace string, controllerName string, resourceController ResourceController) (interface{}, error) {
	// var err error
	var resourceInterface interface{}

	// provider, err := NewKubeProvider()
	// if err != nil {
	// 	ServiceLogger.Warningf("Warningf: %s", err.Error())
	// 	return nil, nil
	// }

	switch resourceController {
	case Deployment:
		// TODO replace with GetAvailableResources in the future
		resourceNamespace := ""
		resource := utils.SyncResourceEntry{
			Kind:      "Deployment",
			Name:      "deployments",
			Namespace: &resourceNamespace,
			Group:     "apps/v1",
			Version:   "",
		}
		resourceInterface = store.GetByKeyParts[appsv1.Deployment](store.VALKEY_KEY_PREFIX, resource.Group, resourceController.String(), namespace, controllerName)
	case ReplicaSet:
		// TODO replace with GetAvailableResources in the future
		resourceNamespace := ""
		resource := utils.SyncResourceEntry{
			Kind:      "ReplicaSet",
			Name:      "replicasets",
			Namespace: &resourceNamespace,
			Group:     "apps/v1",
			Version:   "",
		}
		resourceInterface = store.GetByKeyParts[appsv1.ReplicaSet](store.VALKEY_KEY_PREFIX, resource.Group, resourceController.String(), namespace, controllerName)
	// case StatefulSet:
	// 	// ae: not used at the moment, old code
	// 	resourceInterface, err = provider.ClientSet.AppsV1().StatefulSets(namespace).Get(context.TODO(), controllerName, metav1.GetOptions{})
	// case DaemonSet:
	// 	// ae: not used at the moment, old code
	// 	resourceInterface, err = provider.ClientSet.AppsV1().DaemonSets(namespace).Get(context.TODO(), controllerName, metav1.GetOptions{})
	case Job:
		// TODO replace with GetAvailableResources in the future
		resourceNamespace := ""
		resource := utils.SyncResourceEntry{
			Kind:      "Job",
			Name:      "jobs",
			Namespace: &resourceNamespace,
			Group:     "batch/v1",
			Version:   "",
		}
		resourceInterface = store.GetByKeyParts[batchv1.Job](store.VALKEY_KEY_PREFIX, resource.Group, resourceController.String(), namespace, controllerName)
	case CronJob:
		// TODO replace with GetAvailableResources in the future
		resourceNamespace := ""
		resource := utils.SyncResourceEntry{
			Kind:      "CronJob",
			Name:      "cronjobs",
			Namespace: &resourceNamespace,
			Group:     "batch/v1",
			Version:   "",
		}
		resourceInterface = store.GetByKeyParts[batchv1.CronJob](store.VALKEY_KEY_PREFIX, resource.Group, resourceController.String(), namespace, controllerName)
	}

	if resourceInterface == nil {
		return nil, fmt.Errorf("Warning fetching controller: %s", controllerName)
	}

	return resourceInterface, nil
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
	_ = resourceController
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

func recursiveOwnerRef(namespace string, ownerRef metav1.OwnerReference, resourceItems []ResourceItem) []ResourceItem {
	// Skip already included resourceItems
	for _, item := range resourceItems {
		if item.Kind == ownerRef.Kind {
			return resourceItems
		}
	}

	// Fetch next k8s controller
	resourceInterface, err := controller(namespace, ownerRef.Name, NewResourceController(ownerRef.Kind))
	if err != nil {
		serviceLogger.Warn("failed to fetch resources", "error", err)
		return resourceItems
	}

	// Extract status data from controller
	name, namespace, kind, references, _, object := status(resourceInterface)
	resourceItems = controllerItem(name, kind, namespace, NewResourceController(kind).String(), references, object, resourceItems)

	// Fetch next parent controller
	if len(references) > 0 {
		for _, parentRef := range references {
			if *parentRef.Controller {
				return recursiveOwnerRef(namespace, parentRef, resourceItems)
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
