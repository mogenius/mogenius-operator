package services

import (
	"mogenius-k8s-manager/builder"
	"mogenius-k8s-manager/logger"
	"sort"
	"sync"
	"time"

	"context"
	"fmt"

	punq "github.com/mogenius/punq/kubernetes"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Run a goroutine to fetch k8s events then push them into the channel before timeout
func requestEvents(namespace string, ctx context.Context, wg *sync.WaitGroup, eventsChan chan<- []corev1.Event) {
	defer wg.Done()

	r := punq.AllEvents(namespace, nil)

	var events []corev1.Event
	if r.Error != nil {
		logger.Log.Warningf("Warning fetching events: %s", r.Error)
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
		logger.Log.Debugf("go: timeout waiting for events")
		return
	case eventsChan <- events:
		logger.Log.Debugf("go: push the events into the channel")
	}
}

func StatusService(r ServiceStatusRequest) interface{} {
	logger.Log.Debugf("StatusService for (%s): %s %s", r.ControllerName, r.Namespace, r.Controller)

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		logger.Log.Warningf("Warningf: %s", err.Error())
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
		logger.Log.Warningf("Warning statusItems: %v", err)
	}

	resourceItems, err = buildItem(r.Namespace, r.ControllerName, resourceItems)
	if err != nil {
		logger.Log.Warningf("Warning buildItem: %v", err)
	}

	// Wait for the result from the channel or timeout
	select {
	case events, ok := <-eventsChan:
		if !ok {
			logger.Log.Warningf("Warning event channel closed.")
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
		logger.Log.Warningf("Warning timeout waiting for events")
	}

	// Debug logs
	// jsonData, err := json.MarshalIndent(resourceItems, "", "  ")
	// if err != nil {
	// 	logger.Log.Warningf("Warning marshaling JSON: %v", err)
	// 	return nil
	// }
	// logger.Log.Debugf("JSON: %s", jsonData)

	return resourceItems
}

func kubernetesItems(namespace string, name string, resourceController ResourceController, clientset *kubernetes.Clientset, resourceItems []ResourceItem) ([]ResourceItem, error) {
	resourceInterface, err := controller(namespace, name, resourceController, clientset)
	if err != nil {
		logger.Log.Warningf("\nWarning fetching controller: %s\n", err)
		return resourceItems, err
	}

	metaName, metaNamespace, kind, references, labelSelector, object := status(resourceInterface)
	resourceItems = controllerItem(metaName, kind, metaNamespace, resourceController.String(), references, object, resourceItems)

	pods, err := pods(namespace, labelSelector, clientset)
	if err != nil {
		logger.Log.Warningf("\nWarning fetching pods: %s\n", err)
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
		logger.Log.Warningf("\nWarning fetching resources %s, ns: %s, name: %s, err: %s\n", resourceController.String(), namespace, controllerName, err)
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
	lastJob := builder.LastJobForNamespaceAndServiceName(namespace, name)
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
		logger.Log.Warningf("\nWarning fetching resources: %s\n", err)
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
		{
			status := struct {
				Replicas     int32       `json:"replicas,omitempty"`
				Paused       bool        `json:"paused,omitempty"`
				Image        string      `json:"image,omitempty"`
				StatusObject interface{} `json:"status,omitempty"`
			}{
				Replicas:     *r.Spec.Replicas,
				Paused:       r.Spec.Paused,
				Image:        r.Spec.Template.Spec.Containers[0].Image,
				StatusObject: r.Status,
			}
			return r.ObjectMeta.Name, r.ObjectMeta.Namespace, Deployment.String(), r.OwnerReferences, r.Spec.Selector, status
		}
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
		{
			status := struct {
				Suspend      bool        `json:"suspend,omitempty"`
				Image        string      `json:"image,omitempty"`
				StatusObject interface{} `json:"status,omitempty"`
			}{
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
		}
	default:
		return "", "", Unkown.String(), []metav1.OwnerReference{}, nil, nil
	}
}

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
