package services

import (
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

func controllerAndPods(namespace string, controllerName string, resourceController ResourceController, clientset *kubernetes.Clientset, items []ResourceItem) (*corev1.PodList, []ResourceItem, error) {
	resourceInterface, err := controller(namespace, controllerName, resourceController, clientset)
	if err != nil {
		fmt.Printf("\nError fetching resource: %s\n", err)
		return nil, items, err
	}

	name, namespace, kind, references, labelSelector, object  := status(resourceInterface)
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
				items = append(items, *item)

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
		items = append(items, *item) 
	}


	pods, err := podItems(namespace, labelSelector, clientset)
	
	return pods, items, err
}

func podItems(namespace string, labelSelector *metav1.LabelSelector, clientset *kubernetes.Clientset) (*corev1.PodList, error) {
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

func containerItems(pod corev1.Pod, items []ResourceItem) []ResourceItem {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		item := &ResourceItem{
			Kind:      "Container",
			Name:      containerStatus.Name,
			Namespace: pod.Namespace,
			OwnerName: pod.Name,
			OwnerKind: "Pod",
			StatusObject: containerStatus,
		}
		items = append(items, *item) 
	}

	return items
}

func podItem(pod corev1.Pod, items []ResourceItem) []ResourceItem {
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
			items = append(items, *item) 
        }
    }

	return items
}

func statusItems(namespace string, name string, controllerType ResourceController, clientset *kubernetes.Clientset) ([]ResourceItem, error) {
	items := []ResourceItem{}

    pods, items, err := controllerAndPods(namespace, name, controllerType, clientset, items)
    if err != nil {
        return items, nil
    }

	for _, pod := range pods.Items {
		items = containerItems(pod, items)
		items = podItem(pod, items)
		// Owner reference kind and name
		if len(pod.OwnerReferences) > 0 {
            for _, ownerRef := range pod.OwnerReferences {
				// only controller parents
				if *ownerRef.Controller {
					items = ownerItem(pod.Namespace, ownerRef, clientset, items)
				}
            }
		}
	}

	return items, nil
}

func ownerItem(namespace string, ownerRef metav1.OwnerReference, clientset *kubernetes.Clientset, items []ResourceItem) []ResourceItem {
	// Skip already included items
	for _, item := range items {
		if item.Kind == ownerRef.Kind {
			return items
		}
	}

	resourceInterface, err := controller(namespace,ownerRef.Name, ResourceControllerFromString(ownerRef.Kind), clientset)
	if err != nil {
		fmt.Printf("\nError fetching resource: %s\n", err)
		return items
	}
	
	name, namespace, kind, references, _, object := status(resourceInterface)
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
				items = append(items, *item) 

				return ownerItem(namespace, parentRef, clientset, items)
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
		items = append(items, *item) 

		return items
	}

	return items

}

func status(resource interface{}) (string, string, string, []metav1.OwnerReference, *metav1.LabelSelector, interface{}) {
	switch r := resource.(type) {
	case *appsv1.Deployment:
		return  r.ObjectMeta.Name, r.ObjectMeta.Namespace, "Deployment", r.OwnerReferences, r.Spec.Selector, r.Status
	case *appsv1.ReplicaSet:
		return  r.ObjectMeta.Name, r.ObjectMeta.Namespace, "ReplicaSet", r.OwnerReferences, r.Spec.Selector, r.Status
	case *appsv1.StatefulSet:
		return  r.ObjectMeta.Name, r.ObjectMeta.Namespace, "StatefulSet", r.OwnerReferences, r.Spec.Selector, r.Status
	case *appsv1.DaemonSet:
		return  r.ObjectMeta.Name, r.ObjectMeta.Namespace, "DaemonSet", r.OwnerReferences, r.Spec.Selector, r.Status
	case *batchv1.Job:
		return  r.ObjectMeta.Name, r.ObjectMeta.Namespace, "Job", r.OwnerReferences, r.Spec.Selector, r.Status
	case *batchv1beta1.CronJob:
		return  r.ObjectMeta.Name, r.ObjectMeta.Namespace, "CronJob", r.OwnerReferences, r.Spec.JobTemplate.Spec.Selector, r.Status
	default:
		return "", "", "", []metav1.OwnerReference{}, nil, nil
	}
}

func StatusService(r ServiceStatusRequest) interface{} {

	logger.Log.Debugf("StatusService for (%s): %s %s", r.Namespace, r.ProjectId, r.ServiceName)

	// Collect status
	// ? Build : ?
	// ? Specs.Containers[n].image === mo-default : pending

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		logger.Log.Fatalf("Error: %s", err.Error())
		return nil
	}

	items, err := statusItems(r.Namespace, r.ServiceName, Deployment, provider.ClientSet)
	if err != nil {
		logger.Log.Fatalf("Error statusItems: %v", err)
		return nil
	}

	// jsonData, err := json.MarshalIndent(items, "", "  ")
    // if err != nil {
    //     logger.Log.Fatalf("Error marshaling JSON: %v", err)
	// 	return nil
    // }
	// logger.Log.Debugf("JOSN: %s", jsonData)

	return items
}

type ServiceStatusRequest struct {
	ProjectId   string `json:"projectId"`
	Namespace 	string `json:"namespace"`
	ServiceName string `json:"serviceName"`
}

func ServiceStatusRequestExample() ServiceStatusRequest {
	return ServiceStatusRequest{
		ProjectId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Namespace: "YOUR-NAMESPACE",
		ServiceName: "YOUR-SERVICE-NAME",
	}
}

type ResourceItem struct {
	Kind string              `json:"kind,omitempty"`
	Name string              `json:"name,omitempty"`
	Namespace string         `json:"namespace,omitempty"`
	OwnerName string         `json:"ownerName,omitempty"`
	OwnerKind string         `json:"ownerKind,omitempty"`
	StatusObject interface{} `json:"statusObject,omitempty"`
}

func (item ResourceItem) String() string {
    return fmt.Sprintf("%s, %s, %s, %s, %s, %+v", item.Kind, item.Name, item.Namespace, item.OwnerKind, item.OwnerName, item.StatusObject)
}

type ResourceController int

const (
	Unkown ResourceController = iota
	Deployment
    ReplicaSet
	StatefulSet
    DaemonSet
    Job
	CronJob
)

func (ctrl ResourceController) String() string {
	return [...]string{"Deployment", "ReplicaSet", "StatefulSet", "DaemonSet", "Job", "CronJob"}[ctrl]
}

func ResourceControllerFromString(resourceController string) ResourceController {
	switch resourceController {
	case "Deployment":
		return Deployment
	case "ReplicaSet":
		return ReplicaSet
	case "StatefulSet":
		return StatefulSet
	case "DaemonSet":
		return DaemonSet
	case "Job":
		return Job
	case "CronJob":
		return CronJob
	default:
		return Unkown
	}
}