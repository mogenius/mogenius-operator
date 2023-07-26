package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"os/exec"
	"strings"
	"sync"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func CreateService(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Creating service '%s'.", service.Name), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating service '%s'.", service.Name))

		kubeProvider := NewKubeProvider()
		serviceClient := kubeProvider.ClientSet.CoreV1().Services(namespace.Name)
		newService := generateService(namespace, service)

		newService.Labels = MoUpdateLabels(&newService.Labels, job.ProjectId, &namespace, &service)

		// bind/unbind ports globally
		UpdateTcpUdpPorts(namespace, service, true)

		_, err := serviceClient.Create(context.TODO(), &newService, MoCreateOptions())
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

		kubeProvider := NewKubeProvider()
		serviceClient := kubeProvider.ClientSet.CoreV1().Services(namespace.Name)

		// bind/unbind ports globally
		UpdateTcpUdpPorts(namespace, service, false)

		err := serviceClient.Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
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

		kubeProvider := NewKubeProvider()
		serviceClient := kubeProvider.ClientSet.CoreV1().Services(namespace.Name)
		updateService := generateService(namespace, service)

		updateOptions := metav1.UpdateOptions{
			FieldManager: DEPLOYMENTNAME,
		}

		// bind/unbind ports globally
		UpdateTcpUdpPorts(namespace, service, true)

		_, err := serviceClient.Update(context.TODO(), &updateService, updateOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("UpdateService ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Updated service '%s'.", namespace.Name))
		}
	}(cmd, wg)
	return cmd
}

func UpdateServiceWith(service *v1.Service) error {
	kubeProvider := NewKubeProvider()
	serviceClient := kubeProvider.ClientSet.CoreV1().Services("")
	_, err := serviceClient.Update(context.TODO(), service, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// func BindPort(job structs.Job, namespaceName string, serviceName string, port dtos.K8sPortsDto, wg *sync.WaitGroup) []structs.Command {
// 	result := []structs.Command{}

// 	if port.ExternalPort < 9999 && port.ExternalPort > 65536 {
// 		logger.Log.Error("port must be >9999 and <65536")
// 		return result
// 	}
// 	if port.InternalPort <= 0 && port.InternalPort > 65536 {
// 		logger.Log.Error("port must be >9999 and <65536")
// 		return result
// 	}
// 	if port.PortType != "TCP" && port.PortType != "UDP" {
// 		logger.Log.Error("type musst be TCP or UDP")
// 		return result
// 	}

// 	configMapName := fmt.Sprintf("%s-services", strings.ToLower(port.PortType))
// 	externalPortStr := fmt.Sprintf("%d", port.ExternalPort)
// 	fullServiceName := fmt.Sprintf("%s/%s:%d", namespaceName, serviceName, port.InternalPort)

// 	result = append(result, *mokubernetes.AddKeyToConfigMap(&job, utils.CONFIG.Kubernetes.OwnNamespace, configMapName, externalPortStr, fullServiceName, c, wg))
// 	result = append(result, *mokubernetes.AddPortToService(&job, utils.CONFIG.Kubernetes.OwnNamespace, "nginx-ingress-ingress-nginx-controller", int32(port.ExternalPort), port.PortType, c, wg))

// 	return result
// }

// func UnbindPort(job structs.Job, port dtos.K8sPortsDto, wg *sync.WaitGroup) []structs.Command {
// 	result := []structs.Command{}

// 	if port.ExternalPort < 9999 && port.ExternalPort > 65536 {
// 		logger.Log.Error("port must be >9999 and <65536")
// 		return result
// 	}
// 	if port.PortType != "TCP" && port.PortType != "UDP" {
// 		logger.Log.Error("type musst be TCP or UDP")
// 		return result
// 	}
// 	configMapName := fmt.Sprintf("%s-services", strings.ToLower(port.PortType))
// 	externalPortStr := fmt.Sprintf("%d", port.ExternalPort)
// 	result = append(result, RemoveKeyFromConfigMap(&job, utils.CONFIG.Kubernetes.OwnNamespace, configMapName, externalPortStr, c, wg))
// 	result = append(result, *mokubernetes.RemovePortFromService(&job, utils.CONFIG.Kubernetes.OwnNamespace, "nginx-ingress-ingress-nginx-controller", int32(port.ExternalPort), c, wg))
// 	return result
// }

func UpdateTcpUdpPorts(namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, additive bool) {
	// 1. get configmap and ingress service
	tcpConfigmap := ConfigMapFor(utils.CONFIG.Kubernetes.OwnNamespace, "mogenius-ingress-nginx-tcp")
	udpConfigmap := ConfigMapFor(utils.CONFIG.Kubernetes.OwnNamespace, "mogenius-ingress-nginx-udp")
	ingControllerService := ServiceFor(utils.CONFIG.Kubernetes.OwnNamespace, "mogenius-ingress-nginx-controller")

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

	k8sName := fmt.Sprintf("%s/%s", namespace.Name, service.Name)
	k8sNameIngresss := fmt.Sprintf("%s-%s", namespace.Name, service.Name)

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
				if port.PortType == "TCP" {
					tcpConfigmap.Data[fmt.Sprint(port.ExternalPort)] = fmt.Sprintf("%s:%d", k8sName, port.InternalPort)
					updated = true
				}
				if port.PortType == "UDP" {
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

	// 4. write results to k8s
	tcpResult := UpdateK8sConfigMap(*tcpConfigmap)
	if tcpResult.Error != nil {
		logger.Log.Errorf("UpdateK8sConfigMap: %s", tcpResult)
	}
	udpResult := UpdateK8sConfigMap(*udpConfigmap)
	if udpResult.Error != nil {
		logger.Log.Errorf("UpdateK8sConfigMap: %s", udpResult)
	}
	ingContrResult := UpdateK8sService(*ingControllerService)
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

		service := ServiceFor(namespace, serviceName)
		if service != nil {
			wasModified := false
			for index, aPort := range service.Spec.Ports {
				if aPort.Port == port {
					service.Spec.Ports = utils.Remove(service.Spec.Ports, index)
					wasModified = true
					break
				}
			}

			if wasModified {
				kubeProvider := NewKubeProvider()
				updateOptions := metav1.UpdateOptions{
					FieldManager: DEPLOYMENTNAME,
				}
				serviceClient := kubeProvider.ClientSet.CoreV1().Services(namespace)
				_, err := serviceClient.Update(context.TODO(), service, updateOptions)
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

		service := ServiceFor(namespace, serviceName)
		if service != nil {
			kubeProvider := NewKubeProvider()
			service.Spec.Ports = append(service.Spec.Ports, v1.ServicePort{
				Name:       fmt.Sprintf("%d-%s", port, serviceName),
				Port:       port,
				Protocol:   v1.Protocol(protocol),
				TargetPort: intstr.FromInt(int(port)),
			})

			serviceClient := kubeProvider.ClientSet.CoreV1().Services(namespace)
			_, err := serviceClient.Update(context.TODO(), service, metav1.UpdateOptions{})
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

func ServiceFor(namespace string, serviceName string) *v1.Service {
	kubeProvider := NewKubeProvider()
	serviceClient := kubeProvider.ClientSet.CoreV1().Services(namespace)
	service, err := serviceClient.Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		logger.Log.Errorf("ServiceFor ERROR: %s", err.Error())
		return nil
	}
	return service
}

func AllServices(namespaceName string) []v1.Service {
	result := []v1.Service{}

	provider := NewKubeProvider()
	serviceList, err := provider.ClientSet.CoreV1().Services(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllServices ERROR: %s", err.Error())
		return result
	}

	for _, service := range serviceList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, service.ObjectMeta.Namespace) {
			result = append(result, service)
		}
	}
	return result
}

func AllK8sServices(namespaceName string) K8sWorkloadResult {
	results := AllServices(namespaceName)
	return WorkloadResult(results, nil)
}

func UpdateK8sService(data v1.Service) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	serviceClient := kubeProvider.ClientSet.CoreV1().Services(data.ObjectMeta.Namespace)
	_, err := serviceClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DeleteK8sService(data v1.Service) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	serviceClient := kubeProvider.ClientSet.CoreV1().Services(data.ObjectMeta.Namespace)
	err := serviceClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DescribeK8sService(namespace string, name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "service", name, "-n", namespace)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		logger.Log.Errorf("Error: %s", string(output))
		return WorkloadResult(nil, string(output))
	}
	return WorkloadResult(string(output), nil)
}

func NewK8sService() K8sNewWorkload {
	return NewWorkload(
		RES_SERVICE,
		utils.InitServiceExampleYaml(),
		"A Kubernetes Service is an abstraction which defines a logical set of Pods and a policy by which to access them. The set of Pods targeted by a Service is usually determined by a selector. In this example, the service named 'my-service' listens on port 80, and forwards the requests to port 9376 on the pods which have the label app=MyApp.")
}

func generateService(namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto) v1.Service {
	newService := utils.InitService()
	newService.ObjectMeta.Name = service.Name
	newService.ObjectMeta.Namespace = namespace.Name
	if len(service.Ports) > 0 {
		newService.Spec.Ports = []v1.ServicePort{} // reset before using
		for _, port := range service.Ports {
			if port.PortType == "HTTPS" {
				newService.Spec.Ports = append(newService.Spec.Ports, v1.ServicePort{
					Port: int32(port.InternalPort),
					Name: fmt.Sprintf("%d-%s", port.InternalPort, service.Name),
				})
			} else {
				newService.Spec.Ports = append(newService.Spec.Ports, v1.ServicePort{
					Port:     int32(port.InternalPort),
					Name:     fmt.Sprintf("%d-%s", port.InternalPort, service.Name),
					Protocol: v1.Protocol(port.PortType),
				})
				if port.ExternalPort != 0 {
					newService.Spec.Ports = append(newService.Spec.Ports, v1.ServicePort{
						Port:     int32(port.ExternalPort),
						Name:     fmt.Sprintf("%d-%s", port.ExternalPort, service.Name),
						Protocol: v1.Protocol(port.PortType),
					})
				}
			}
		}
	}
	newService.Spec.Selector["app"] = service.Name

	return newService
}
