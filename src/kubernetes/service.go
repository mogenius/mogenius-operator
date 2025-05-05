package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/websocket"
	"sync"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func AllServices(namespaceName string) []v1.Service {
	result := []v1.Service{}

	clientset := clientProvider.K8sClientSet()
	serviceList, err := clientset.CoreV1().Services(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		k8sLogger.Error("AllServices", "error", err.Error())
		return result
	}

	for _, service := range serviceList.Items {
		service.Kind = "Service"
		service.APIVersion = "v1"
		result = append(result, service)
	}
	return result
}

func DeleteService(eventClient websocket.WebsocketClient, job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand(eventClient, "delete", "Delete Service", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(eventClient, job, "Deleting Service")

		clientset := clientProvider.K8sClientSet()
		serviceClient := clientset.CoreV1().Services(namespace.Name)

		// bind/unbind ports globally
		// TODO: rework TCP/UDP stuff
		// UpdateTcpUdpPorts(namespace, service, false)

		err := serviceClient.Delete(context.TODO(), service.ControllerName, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			cmd.Fail(eventClient, job, fmt.Sprintf("DeleteService ERROR: %s", err.Error()))
		} else {
			cmd.Success(eventClient, job, "Deleted Service")
		}
	}(wg)
}

func UpdateService(eventClient websocket.WebsocketClient, job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand(eventClient, "update", "Update Application", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(eventClient, job, "Update Application")

		existingService, getSrvErr := GetService(namespace.Name, service.ControllerName)
		if getSrvErr != nil {
			existingService = nil
		}

		clientset := clientProvider.K8sClientSet()
		serviceClient := clientset.CoreV1().Services(namespace.Name)
		updateService := generateService(existingService, namespace, service)

		updateOptions := metav1.UpdateOptions{
			FieldManager: GetOwnDeploymentName(),
		}

		// bind/unbind ports globally
		// TODO: rework TCP/UDP stuff
		// UpdateTcpUdpPorts(namespace, service, true)

		if len(updateService.Spec.Ports) <= 0 {
			if getSrvErr == nil {
				err := serviceClient.Delete(context.TODO(), service.ControllerName, metav1.DeleteOptions{})
				if err != nil {
					cmd.Fail(eventClient, job, fmt.Sprintf("UpdateApplication (Delete) ERROR: %s", err.Error()))
				} else {
					cmd.Success(eventClient, job, "Updated Application")
				}
			} else {
				cmd.Success(eventClient, job, "Updated Application")
			}
		} else {
			_, err := serviceClient.Update(context.TODO(), &updateService, updateOptions)
			if err != nil {
				cmd.Fail(eventClient, job, fmt.Sprintf("UpdateApplication ERROR: %s", err.Error()))
			} else {
				cmd.Success(eventClient, job, "Updated Application")
			}
		}
	}(wg)
}

func GetService(namespace string, serviceName string) (*v1.Service, error) {
	clientset := clientProvider.K8sClientSet()
	serviceClient := clientset.CoreV1().Services(namespace)
	service, err := serviceClient.Get(context.TODO(), serviceName, metav1.GetOptions{})
	service.Kind = "Service"
	service.APIVersion = "v1"

	return service, err
}

func generateService(existingService *v1.Service, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto) v1.Service {
	newService := existingService
	if newService == nil {
		newService = &v1.Service{}
		newService.ObjectMeta.Name = service.ControllerName
		newService.ObjectMeta.Namespace = namespace.Name
		newService.Spec.Selector = make(map[string]string)
		newService.Spec.Selector["app"] = service.ControllerName
	}
	newService.Spec.Ports = []v1.ServicePort{} // reset before using
	for _, port := range service.Ports {
		if port.PortType == dtos.PortTypeHTTPS {
			newService.Spec.Ports = append(newService.Spec.Ports, v1.ServicePort{
				Port: intstr.Parse(port.InternalPort).IntVal,
				Name: fmt.Sprintf("%s-%s", port.InternalPort, service.ControllerName),
			})
		} else {
			newService.Spec.Ports = append(newService.Spec.Ports, v1.ServicePort{
				Port:     intstr.Parse(port.InternalPort).IntVal,
				Name:     fmt.Sprintf("%s-%s", port.InternalPort, service.ControllerName),
				Protocol: v1.Protocol(port.PortType),
			})
			if intstr.Parse(port.ExternalPort).IntVal != 0 {
				newService.Spec.Ports = append(newService.Spec.Ports, v1.ServicePort{
					Port:     intstr.Parse(port.ExternalPort).IntVal,
					Name:     fmt.Sprintf("%s-%s", port.ExternalPort, service.ControllerName),
					Protocol: v1.Protocol(port.PortType),
				})
			}
		}
	}

	return *newService
}

func ServiceWithLabels(labelSelector string) *v1.Service {
	clientset := clientProvider.K8sClientSet()
	namespace := ""
	serviceClient := clientset.CoreV1().Services(namespace)
	service, err := serviceClient.List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		k8sLogger.Error("ServiceFor: failed to list services", "namespace", namespace, "labelSelector", labelSelector, "error", err)
		return nil
	}

	if len(service.Items) == 1 {
		return &service.Items[0]
	} else if len(service.Items) > 1 {
		k8sLogger.Error("ServiceFor: More than one service found. Returning first one.", "amountFoundServices", len(service.Items), "labelSelector", labelSelector)
		return &service.Items[0]
	} else {
		k8sLogger.Error("ServiceFor: No service found for labelSelector.", "labelSelector", labelSelector)
		return nil
	}
}

func ServiceFor(namespace string, serviceName string) *v1.Service {
	clientset := clientProvider.K8sClientSet()
	serviceClient := clientset.CoreV1().Services(namespace)
	service, err := serviceClient.Get(context.TODO(), serviceName, metav1.GetOptions{})
	service.Kind = "Service"
	service.APIVersion = "v1"
	if err != nil {
		k8sLogger.Error("ServiceFor", "error", err.Error())
		return nil
	}
	return service
}
