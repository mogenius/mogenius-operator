package kubernetes

import (
	"context"
	"fmt"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/websocket"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func AllServices(namespaceName string) []v1.Service {
	result := []v1.Service{}

	clientset := clientProvider.K8sClientSet()
	serviceList, err := clientset.CoreV1().Services(namespaceName).List(context.Background(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
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

func DeleteService(eventClient websocket.WebsocketClient, job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto) {
	cmd := structs.CreateCommand(eventClient, "delete", "Delete Service", job)
	cmd.Start(eventClient, job, "Deleting Service")

	clientset := clientProvider.K8sClientSet()
	serviceClient := clientset.CoreV1().Services(namespace.Name)

	// bind/unbind ports globally
	// TODO: rework TCP/UDP stuff
	// UpdateTcpUdpPorts(namespace, service, false)

	err := serviceClient.Delete(context.Background(), service.ControllerName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		cmd.Fail(eventClient, job, fmt.Sprintf("DeleteService ERROR: %s", err.Error()))
	} else {
		cmd.Success(eventClient, job, "Deleted Service")
	}
}

func UpdateService(eventClient websocket.WebsocketClient, job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, config cfg.ConfigModule) {
	cmd := structs.CreateCommand(eventClient, "update", "Update Application", job)
	cmd.Start(eventClient, job, "Update Application")

	existingService, getSrvErr := GetService(namespace.Name, service.ControllerName)
	if getSrvErr != nil {
		existingService = nil
	}

	clientset := clientProvider.K8sClientSet()
	serviceClient := clientset.CoreV1().Services(namespace.Name)
	updateService := generateService(existingService, namespace, service)

	updateOptions := metav1.UpdateOptions{
		FieldManager: GetOwnDeploymentName(config),
	}

	// bind/unbind ports globally
	// TODO: rework TCP/UDP stuff
	// UpdateTcpUdpPorts(namespace, service, true)

	if len(updateService.Spec.Ports) <= 0 {
		if getSrvErr == nil {
			err := serviceClient.Delete(context.Background(), service.ControllerName, metav1.DeleteOptions{})
			if err != nil {
				cmd.Fail(eventClient, job, fmt.Sprintf("UpdateApplication (Delete) ERROR: %s", err.Error()))
			} else {
				cmd.Success(eventClient, job, "Updated Application")
			}
		} else {
			cmd.Success(eventClient, job, "Updated Application")
		}
	} else {
		_, err := serviceClient.Update(context.Background(), &updateService, updateOptions)
		if err != nil {
			cmd.Fail(eventClient, job, fmt.Sprintf("UpdateApplication ERROR: %s", err.Error()))
		} else {
			cmd.Success(eventClient, job, "Updated Application")
		}
	}
}

func GetService(namespace string, serviceName string) (*v1.Service, error) {
	clientset := clientProvider.K8sClientSet()
	serviceClient := clientset.CoreV1().Services(namespace)
	service, err := serviceClient.Get(context.Background(), serviceName, metav1.GetOptions{})
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

func FindPrometheusService() (namespace string, service string, port int32, err error) {
	clientset := clientProvider.K8sClientSet()
	serviceClient := clientset.CoreV1().Services("")
	serviceList, err := serviceClient.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		k8sLogger.Error("findPrometheusService", "error", err.Error())
		return "", "", -1, fmt.Errorf("failed to list services: %v", err)
	}
	for _, service := range serviceList.Items {
		if service.Name == "prometheus-kube-prometheus-prometheus" ||
			service.Name == "kube-prometheus-stack-prometheus" ||
			service.Name == "prometheus-server" ||
			service.Name == "prometheus-service" ||
			service.Name == "prometheus" ||
			service.Name == "prometheus-prometheus-server" {
			if len(service.Spec.Ports) > 0 {
				return service.Namespace, service.Name, service.Spec.Ports[0].Port, nil
			}
		}
	}
	return "", "", -1, fmt.Errorf("prometheus service not found in any namespace")
}

func FindSealedSecretsService(cfg cfg.ConfigModule) (namespace string, service string, port int32, err error) {
	clientset := clientProvider.K8sClientSet()

	ownNamespace := cfg.Get("MO_OWN_NAMESPACE")

	// exists mogenius sealed-secrets config
	sealedSecretsConfig, err := clientset.CoreV1().ConfigMaps(ownNamespace).Get(context.Background(), "sealed-secrets-config", metav1.GetOptions{})
	if err == nil {
		if namespaceName, ok := sealedSecretsConfig.Data["namespaceName"]; ok {
			if releaseName, ok := sealedSecretsConfig.Data["releaseName"]; ok {
				sealedSecretsService, err := clientset.CoreV1().Services(namespaceName).Get(context.Background(), releaseName, metav1.GetOptions{})
				if err == nil && len(sealedSecretsService.Spec.Ports) > 0 && sealedSecretsService.Spec.Ports[0].Port != 0 {
					return sealedSecretsService.Namespace, sealedSecretsService.Name, sealedSecretsService.Spec.Ports[0].Port, nil
				}
			}
		}
	}

	serviceClient := clientset.CoreV1().Services("")
	serviceList, err := serviceClient.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		k8sLogger.Error("findSealedSecretsService", "error", err.Error())
		return "", "", -1, fmt.Errorf("failed to list services: %v", err)
	}
	for _, service := range serviceList.Items {
		if service.Name == "sealed-secrets" {
			if len(service.Spec.Ports) > 0 {
				return service.Namespace, service.Name, service.Spec.Ports[0].Port, nil
			}
		}
	}
	return "", "", -1, fmt.Errorf("sealed-secrets service not found in any namespace")
}
