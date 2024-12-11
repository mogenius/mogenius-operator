package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"strings"
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

func DeleteService(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", "Delete Service", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting Service")

		clientset := clientProvider.K8sClientSet()
		serviceClient := clientset.CoreV1().Services(namespace.Name)

		// bind/unbind ports globally
		// TODO: rework TCP/UDP stuff
		// UpdateTcpUdpPorts(namespace, service, false)

		err := serviceClient.Delete(context.TODO(), service.ControllerName, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			cmd.Fail(job, fmt.Sprintf("DeleteService ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Deleted Service")
		}
	}(wg)
}

func UpdateService(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("update", "Update Application", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Update Application")

		existingService, getSrvErr := GetService(namespace.Name, service.ControllerName)
		if getSrvErr != nil {
			existingService = nil
		}

		clientset := clientProvider.K8sClientSet()
		serviceClient := clientset.CoreV1().Services(namespace.Name)
		updateService := generateService(existingService, namespace, service)

		updateOptions := metav1.UpdateOptions{
			FieldManager: DEPLOYMENTNAME,
		}

		// bind/unbind ports globally
		// TODO: rework TCP/UDP stuff
		// UpdateTcpUdpPorts(namespace, service, true)

		if len(updateService.Spec.Ports) <= 0 {
			if getSrvErr == nil {
				err := serviceClient.Delete(context.TODO(), service.ControllerName, metav1.DeleteOptions{})
				if err != nil {
					cmd.Fail(job, fmt.Sprintf("UpdateApplication (Delete) ERROR: %s", err.Error()))
				} else {
					cmd.Success(job, "Updated Application")
				}
			} else {
				cmd.Success(job, "Updated Application")
			}
		} else {
			_, err := serviceClient.Update(context.TODO(), &updateService, updateOptions)
			if err != nil {
				cmd.Fail(job, fmt.Sprintf("UpdateApplication ERROR: %s", err.Error()))
			} else {
				cmd.Success(job, "Updated Application")
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

func UpdateTcpUdpPorts(namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, additive bool) {
	// 1. get configmap and ingress service
	tcpConfigmap := ConfigMapFor(config.Get("MO_OWN_NAMESPACE"), "mogenius-ingress-nginx-tcp", true)
	udpConfigmap := ConfigMapFor(config.Get("MO_OWN_NAMESPACE"), "mogenius-ingress-nginx-udp", true)
	ingControllerService := ServiceFor(config.Get("MO_OWN_NAMESPACE"), "mogenius-ingress-nginx-controller")

	if tcpConfigmap == nil {
		k8sLogger.Error("ConfigMap for %s/%s not found. Aborting UpdateTcpUdpPorts(). Please check why this ConfigMap does not exist. It is essential.", config.Get("MO_OWN_NAMESPACE"), "mogenius-ingress-nginx-tcp")
		return
	}

	if udpConfigmap == nil {
		k8sLogger.Error("ConfigMap for %s/%s not found. Aborting UpdateTcpUdpPorts(). Please check why this ConfigMap does not exist. It is essential.", config.Get("MO_OWN_NAMESPACE"), "mogenius-ingress-nginx-udp")
		return
	}

	if tcpConfigmap.Data == nil {
		tcpConfigmap.Data = make(map[string]string)
	}
	if udpConfigmap.Data == nil {
		udpConfigmap.Data = make(map[string]string)
	}

	k8sName := fmt.Sprintf("%s/%s", namespace.Name, service.ControllerName)
	k8sNameIngresss := fmt.Sprintf("%s-%s", namespace.Name, service.ControllerName)

	// 2. Remove all entries for this service
	for cmKey, cmValue := range tcpConfigmap.Data {
		if strings.HasPrefix(cmValue, k8sName) {
			delete(tcpConfigmap.Data, cmKey)
		}
	}
	for cmKey, cmValue := range udpConfigmap.Data {
		if strings.HasPrefix(cmValue, k8sName) {
			delete(udpConfigmap.Data, cmKey)
		}
	}
	for index, port := range ingControllerService.Spec.Ports {
		if strings.HasPrefix(port.Name, k8sNameIngresss) {
			ingControllerService.Spec.Ports = utils.Remove(ingControllerService.Spec.Ports, index)
		}
	}

	// 3. Add all entries for this servive
	if additive {
		for _, port := range service.Ports {
			if port.Expose {
				updated := false
				if port.PortType == dtos.PortTypeTCP {
					tcpConfigmap.Data[fmt.Sprint(port.ExternalPort)] = fmt.Sprintf("%s:%d", k8sName, port.InternalPort)
					updated = true
				}
				if port.PortType == dtos.PortTypeUDP {
					udpConfigmap.Data[fmt.Sprint(port.ExternalPort)] = fmt.Sprintf("%s:%d", k8sName, port.InternalPort)
					updated = true
				}

				if updated {
					ingControllerService.Spec.Ports = append(ingControllerService.Spec.Ports, v1.ServicePort{
						Name:       fmt.Sprintf("%s-%d", k8sNameIngresss, port.ExternalPort),
						Protocol:   v1.Protocol(port.PortType),
						Port:       int32(port.ExternalPort),
						TargetPort: intstr.FromInt32(int32(port.ExternalPort)),
					})
				}
			}
		}
	}

	// 4. write results to k8s
	tcpErr := UpdateK8sConfigMap(*tcpConfigmap)
	if tcpErr != nil {
		k8sLogger.Error("UpdateK8sConfigMap", "error", tcpErr)
	}
	udpErr := UpdateK8sConfigMap(*udpConfigmap)
	if udpErr != nil {
		k8sLogger.Error("UpdateK8sConfigMap", "error", udpErr)
	}
	ingContrErr := UpdateK8sService(*ingControllerService)
	if ingContrErr != nil {
		k8sLogger.Error("UpdateK8sConfigMap", "error", ingContrErr)
	}
}

func UpdateK8sService(data v1.Service) error {
	clientset := clientProvider.K8sClientSet()
	client := clientset.CoreV1().Services(data.ObjectMeta.Namespace)
	_, err := client.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
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
				Port: int32(port.InternalPort),
				Name: fmt.Sprintf("%d-%s", port.InternalPort, service.ControllerName),
			})
		} else {
			newService.Spec.Ports = append(newService.Spec.Ports, v1.ServicePort{
				Port:     int32(port.InternalPort),
				Name:     fmt.Sprintf("%d-%s", port.InternalPort, service.ControllerName),
				Protocol: v1.Protocol(port.PortType),
			})
			if port.ExternalPort != 0 {
				newService.Spec.Ports = append(newService.Spec.Ports, v1.ServicePort{
					Port:     int32(port.ExternalPort),
					Name:     fmt.Sprintf("%d-%s", port.ExternalPort, service.ControllerName),
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
