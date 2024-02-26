package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"strings"
	"sync"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func CreateService(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Creating service '%s'.", service.ControllerName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating service '%s'.", service.ControllerName))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		serviceClient := provider.ClientSet.CoreV1().Services(namespace.Name)
		newService := generateService(nil, namespace, service)

		newService.Labels = MoUpdateLabels(&newService.Labels, job.ProjectId, &namespace, &service)

		// bind/unbind ports globally
		UpdateTcpUdpPorts(namespace, service, true)

		_, err = serviceClient.Create(context.TODO(), &newService, MoCreateOptions())
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateService ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Created service '%s'.", namespace.Name))
		}

	}(cmd, wg)
	return cmd
}

func DeleteService(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete Service", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting service '%s'.", namespace.Name))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		serviceClient := provider.ClientSet.CoreV1().Services(namespace.Name)

		// bind/unbind ports globally
		UpdateTcpUdpPorts(namespace, service, false)

		err = serviceClient.Delete(context.TODO(), service.ControllerName, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteService ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted service '%s'.", namespace.Name))
		}
	}(cmd, wg)
	return cmd
}

func UpdateService(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Update Service", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Update service '%s'.", namespace.Name))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		serviceClient := provider.ClientSet.CoreV1().Services(namespace.Name)
		existingService, getSrvErr := serviceClient.Get(context.TODO(), service.ControllerName, metav1.GetOptions{})
		if getSrvErr != nil {
			existingService = nil
		}
		updateService := generateService(existingService, namespace, service)

		updateOptions := metav1.UpdateOptions{
			FieldManager: DEPLOYMENTNAME,
		}

		// bind/unbind ports globally
		UpdateTcpUdpPorts(namespace, service, true)

		if len(updateService.Spec.Ports) <= 0 {
			if getSrvErr == nil {
				err := serviceClient.Delete(context.TODO(), service.ControllerName, metav1.DeleteOptions{})
				if err != nil {
					cmd.Fail(fmt.Sprintf("UpdateService (Delete) ERROR: %s", err.Error()))
				} else {
					cmd.Success(fmt.Sprintf("Updated service '%s'.", namespace.Name))
				}
			} else {
				cmd.Success(fmt.Sprintf("Updated service '%s'.", namespace.Name))
			}
		} else {
			_, err = serviceClient.Update(context.TODO(), &updateService, updateOptions)
			if err != nil {
				cmd.Fail(fmt.Sprintf("UpdateService ERROR: %s", err.Error()))
			} else {
				cmd.Success(fmt.Sprintf("Updated service '%s'.", namespace.Name))
			}
		}
	}(cmd, wg)
	return cmd
}

func UpdateServiceWith(service *v1.Service) error {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return err
	}
	serviceClient := provider.ClientSet.CoreV1().Services("")
	_, err = serviceClient.Update(context.TODO(), service, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func UpdateTcpUdpPorts(namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, additive bool) {
	// 1. get configmap and ingress service
	tcpConfigmap := punq.ConfigMapFor(utils.CONFIG.Kubernetes.OwnNamespace, "mogenius-ingress-nginx-tcp", true, nil)
	udpConfigmap := punq.ConfigMapFor(utils.CONFIG.Kubernetes.OwnNamespace, "mogenius-ingress-nginx-udp", true, nil)
	ingControllerService := punq.ServiceFor(utils.CONFIG.Kubernetes.OwnNamespace, "mogenius-ingress-nginx-controller", nil)

	if tcpConfigmap == nil {
		logger.Log.Errorf("ConfigMap for %s/%s not found. Aborting UpdateTcpUdpPorts(). Please check why this ConfigMap does not exist. It is essential.", utils.CONFIG.Kubernetes.OwnNamespace, "mogenius-ingress-nginx-tcp")
		return
	}

	if udpConfigmap == nil {
		logger.Log.Errorf("ConfigMap for %s/%s not found. Aborting UpdateTcpUdpPorts(). Please check why this ConfigMap does not exist. It is essential.", utils.CONFIG.Kubernetes.OwnNamespace, "mogenius-ingress-nginx-udp")
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
			ingControllerService.Spec.Ports = punqUtils.Remove(ingControllerService.Spec.Ports, index)
		}
	}

	// 3. Add all entries for this servive
	if additive {
		for _, container := range service.Containers {
			for _, port := range container.Ports {
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
							TargetPort: intstr.FromInt(port.ExternalPort),
						})
					}
				}
			}
		}
	}

	// 4. write results to k8s
	tcpResult := punq.UpdateK8sConfigMap(*tcpConfigmap, nil)
	if tcpResult.Error != nil {
		logger.Log.Errorf("UpdateK8sConfigMap: %s", tcpResult)
	}
	udpResult := punq.UpdateK8sConfigMap(*udpConfigmap, nil)
	if udpResult.Error != nil {
		logger.Log.Errorf("UpdateK8sConfigMap: %s", udpResult)
	}
	ingContrResult := punq.UpdateK8sService(*ingControllerService, nil)
	if ingContrResult.Error != nil {
		logger.Log.Errorf("UpdateK8sConfigMap: %s", ingContrResult)
	}
}

func RemovePortFromService(job *structs.Job, namespace string, serviceName string, port int32, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Remove Port from Service", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Remove Port '%d'.", port))

		service := punq.ServiceFor(namespace, serviceName, nil)
		if service != nil {
			wasModified := false
			for index, aPort := range service.Spec.Ports {
				if aPort.Port == port {
					service.Spec.Ports = punqUtils.Remove(service.Spec.Ports, index)
					wasModified = true
					break
				}
			}

			if wasModified {
				provider, err := punq.NewKubeProvider(nil)
				if err != nil {
					cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
					return
				}
				updateOptions := metav1.UpdateOptions{
					FieldManager: DEPLOYMENTNAME,
				}
				serviceClient := provider.ClientSet.CoreV1().Services(namespace)
				_, err = serviceClient.Update(context.TODO(), service, updateOptions)
				if err != nil {
					cmd.Fail(fmt.Sprintf("RemoveKey ERROR: %s", err.Error()))
					return
				}
				cmd.Success(fmt.Sprintf("Port %d successfully removed.", port))
				return
			} else {
				cmd.Success(fmt.Sprintf("Port %d was not contained in list.", port))
				return
			}
		}
		cmd.Fail(fmt.Sprintf("Service '%s/%s' not found.", namespace, serviceName))
	}(cmd, wg)
	return cmd
}

func AddPortToService(job *structs.Job, namespace string, serviceName string, port int32, protocol string, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Add Port to Service", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Add Port '%d'.", port))

		service := punq.ServiceFor(namespace, serviceName, nil)
		if service != nil {
			provider, err := punq.NewKubeProvider(nil)
			if err != nil {
				cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
				return
			}
			service.Spec.Ports = append(service.Spec.Ports, v1.ServicePort{
				Name:       fmt.Sprintf("%d-%s", port, serviceName),
				Port:       port,
				Protocol:   v1.Protocol(protocol),
				TargetPort: intstr.FromInt(int(port)),
			})

			serviceClient := provider.ClientSet.CoreV1().Services(namespace)
			_, err = serviceClient.Update(context.TODO(), service, metav1.UpdateOptions{})
			if err != nil {
				cmd.Fail(fmt.Sprintf("AddPortToService ERROR: %s", err.Error()))
				return
			}
			cmd.Success(fmt.Sprintf("Port %d added successfully removed.", port))
			return
		}
		cmd.Fail(fmt.Sprintf("Service '%s/%s' not found.", namespace, serviceName))
	}(cmd, wg)
	return cmd
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
	for _, container := range service.Containers {
		for _, port := range container.Ports {
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
	}

	return *newService
}

func ServiceWithLabels(labelSelector string, contextId *string) *v1.Service {
	provider, err := punq.NewKubeProvider(contextId)
	if err != nil {
		logger.Log.Errorf("ServiceWith ERROR: %s", err.Error())
		return nil
	}
	serviceClient := provider.ClientSet.CoreV1().Services("")
	service, err := serviceClient.List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		logger.Log.Errorf("ServiceFor ERROR: %s", err.Error())
		return nil
	}

	if len(service.Items) == 1 {
		return &service.Items[0]
	} else if len(service.Items) > 1 {
		logger.Log.Errorf("ServiceFor ERR: More (%d) than one service found for '%s'. Returning first one.", len(service.Items), labelSelector)
		return &service.Items[0]
	} else {
		logger.Log.Errorf("ServiceFor ERR: No service found for labelsSelector '%s'.", labelSelector)
		return nil
	}
}
