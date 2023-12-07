package services

import (
	"encoding/json"
	"mogenius-k8s-manager/builder"
	"mogenius-k8s-manager/logger"

	"context"
	"fmt"

	punq "github.com/mogenius/punq/kubernetes"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func StatusService(r ServiceStatusRequest) interface{} {
	logger.Log.Debugf("StatusService for (%s): %s %s", r.ServiceName, r.Namespace, r.Controller)

	// Collect status
	// ? Specs.Containers[n].image === mo-default : pending

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		logger.Log.Fatalf("Error: %s", err.Error())
		return nil
	}

	resourceItems, err := statusItems(r.Namespace, r.ServiceName, NewResourceController(r.Controller), provider.ClientSet)
	if err != nil {
		logger.Log.Fatalf("Failed statusItems: %v", err)
	}

	resourceItems, err = buildItem(r.Namespace, r.ServiceName, resourceItems)
	if err != nil {
		logger.Log.Fatalf("Failed buildItem: %v", err)
	}

	// Debug logs
	jsonData, err := json.MarshalIndent(resourceItems, "", "  ")
	if err != nil {
		logger.Log.Fatalf("Error marshaling JSON: %v", err)
		return nil
	}
	logger.Log.Debugf("JOSN: %s", jsonData)

	return resourceItems
}

func statusItems(namespace string, name string, resourceController ResourceController, clientset *kubernetes.Clientset) ([]ResourceItem, error) {
	resourceItems := []ResourceItem{}

	resourceInterface, err := controller(namespace, name, resourceController, clientset)
	if err != nil {
		fmt.Printf("\nError fetching controller: %s\n", err)
		return resourceItems, err
	}

	metaName, metaNamespace, kind, references, labelSelector, object  := status(resourceInterface)
	resourceItems = controllerItem(metaName, kind, metaNamespace, resourceController.String(), references, object, resourceItems)

	pods, err := pods(namespace, labelSelector, clientset)
	if err != nil {
		fmt.Printf("\nError fetching pods: %s\n", err)
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
		resourceInterface, err = clientset.BatchV1beta1().CronJobs(namespace).Get(context.TODO(), controllerName, metav1.GetOptions{})
	}

	if err != nil {
		fmt.Printf("\nError fetching resource: %s\n", err)
		return nil, err
	}

	return resourceInterface, nil
}


func pods(namespace string, labelSelector *metav1.LabelSelector, clientset *kubernetes.Clientset) (*corev1.PodList, error) {
	if labelSelector != nil {
		selector := metav1.FormatLabelSelector(labelSelector)
		pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: selector,
		})

		if err != nil {
			return nil, err
		}

		return pods, nil
	}

	return &corev1.PodList{}, nil
}

func buildItem(namespace, name string, resourceItems []ResourceItem) ([]ResourceItem, error) {
	info, err := builder.BuildJobInfoEntry(namespace, name)
	if err != nil {
		return resourceItems, err
	}

	item := &ResourceItem{
		Kind:      "Build",
		Name:      name,
		Namespace: namespace,
		OwnerName: "",
		OwnerKind: "",
		StatusObject: info,
	}

	resourceItems = append(resourceItems, *item) 

	return resourceItems, nil
}

func containerItems(pod corev1.Pod, resourceItems []ResourceItem) []ResourceItem {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		item := &ResourceItem{
			Kind:      "Container",
			Name:      containerStatus.Name,
			Namespace: pod.Namespace,
			OwnerName: pod.Name,
			OwnerKind: "Pod",
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
					Kind:      kind,
					Name:      name,
					Namespace: namespace,
					OwnerName:  parentRef.Name,
					OwnerKind: parentRef.Kind,
					StatusObject: object,
				}
				resourceItems = append(resourceItems, *item)

				break
			}
		}
	} else {
		item := &ResourceItem{
			Kind:      kind,
			Name:      name,
			Namespace: namespace,
			OwnerName:  "",
			OwnerKind: "",
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
	resourceInterface, err := controller(namespace,ownerRef.Name, NewResourceController(ownerRef.Kind), clientset)
	if err != nil {
		fmt.Printf("\nError fetching resource: %s\n", err)
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
		return  r.ObjectMeta.Name, r.ObjectMeta.Namespace, Deployment.String(), r.OwnerReferences, r.Spec.Selector, r.Status
	case *appsv1.ReplicaSet:
		return  r.ObjectMeta.Name, r.ObjectMeta.Namespace, ReplicaSet.String(), r.OwnerReferences, r.Spec.Selector, r.Status
	case *appsv1.StatefulSet:
		return  r.ObjectMeta.Name, r.ObjectMeta.Namespace, StatefulSet.String(), r.OwnerReferences, r.Spec.Selector, r.Status
	case *appsv1.DaemonSet:
		return  r.ObjectMeta.Name, r.ObjectMeta.Namespace, DaemonSet.String(), r.OwnerReferences, r.Spec.Selector, r.Status
	case *batchv1.Job:
		return  r.ObjectMeta.Name, r.ObjectMeta.Namespace, Job.String(), r.OwnerReferences, r.Spec.Selector, r.Status
	case *batchv1beta1.CronJob:
		return  r.ObjectMeta.Name, r.ObjectMeta.Namespace, CronJob.String(), r.OwnerReferences, r.Spec.JobTemplate.Spec.Selector, r.Status
	default:
		return "", "", Unkown.String(), []metav1.OwnerReference{}, nil, nil
	}
}

type ServiceStatusRequest struct {
	Namespace 	string `json:"namespace"`
	ServiceName string `json:"serviceName"`
	Controller  string `json:"controller"`
}

func ServiceStatusRequestExample() ServiceStatusRequest {
	return ServiceStatusRequest{
		Namespace: "YOUR-NAMESPACE",
		ServiceName: "YOUR-SERVICE-NAME",
		Controller: Deployment.String(),
	}
}

type ResourceItem struct {
	Kind string              `json:"kind"`
	Name string              `json:"name"`
	Namespace string         `json:"namespace"`
	OwnerName string         `json:"ownerName,omitempty"`
	OwnerKind string         `json:"ownerKind,omitempty"`
	StatusObject interface{} `json:"statusObject,omitempty"`
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
//   otherwise everything will be messed up
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