package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

	"github.com/gorilla/websocket"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func CreateService(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Creating service '%s'.", service.K8sName), job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating service '%s'.", service.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}

		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateService ERROR: %s", err.Error()), c)
			return
		}

		serviceClient := kubeProvider.ClientSet.CoreV1().Services(stage.K8sName)
		newService := generateService(stage, service)

		createOptions := metav1.CreateOptions{
			FieldManager: DEPLOYMENTNAME,
		}

		_, err = serviceClient.Create(context.TODO(), &newService, createOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateService ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Created service '%s'.", stage.K8sName), c)
		}

	}(cmd, wg)
	return cmd
}

func DeleteService(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete Service", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting service '%s'.", stage.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}

		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteService ERROR: %s", err.Error()), c)
			return
		}

		serviceClient := kubeProvider.ClientSet.CoreV1().Services(stage.K8sName)

		err = serviceClient.Delete(context.TODO(), service.K8sName, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteService ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Deleted service '%s'.", stage.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func UpdateService(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Update Service", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Update service '%s'.", stage.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}

		if err != nil {
			cmd.Fail(fmt.Sprintf("UpdateService ERROR: %s", err.Error()), c)
			return
		}

		serviceClient := kubeProvider.ClientSet.CoreV1().Services(stage.K8sName)
		updateService := generateService(stage, service)

		updateOptions := metav1.UpdateOptions{
			FieldManager: DEPLOYMENTNAME,
		}

		_, err = serviceClient.Update(context.TODO(), &updateService, updateOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("UpdateService ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Updated service '%s'.", stage.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func UpdateServiceWith(service *v1.Service) error {
	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}
	if err != nil {
		logger.Log.Errorf("UpdateServiceWith ERROR: %s", err.Error())
		return err
	}

	serviceClient := kubeProvider.ClientSet.CoreV1().Services("")
	_, err = serviceClient.Update(context.TODO(), service, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func RemovePortFromService(job *structs.Job, namespace string, serviceName string, port int32, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Remove Port from Service", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Remove Port '%d'.", port), c)

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
				var kubeProvider *KubeProvider
				var err error
				if !utils.CONFIG.Kubernetes.RunInCluster {
					kubeProvider, err = NewKubeProviderLocal()
				} else {
					kubeProvider, err = NewKubeProviderInCluster()
				}
				if err != nil {
					cmd.Fail(fmt.Sprintf("RemoveKey ERROR: %s", err.Error()), c)
					return
				}
				updateOptions := metav1.UpdateOptions{
					FieldManager: DEPLOYMENTNAME,
				}
				serviceClient := kubeProvider.ClientSet.CoreV1().Services(namespace)
				_, err = serviceClient.Update(context.TODO(), service, updateOptions)
				if err != nil {
					cmd.Fail(fmt.Sprintf("RemoveKey ERROR: %s", err.Error()), c)
					return
				}
				cmd.Success(fmt.Sprintf("Port %d successfully removed.", port), c)
				return
			} else {
				cmd.Success(fmt.Sprintf("Port %d was not contained in list.", port), c)
				return
			}
		}
		cmd.Fail(fmt.Sprintf("Service '%s/%s' not found.", namespace, serviceName), c)
	}(cmd, wg)
	return cmd
}

func AddPortToService(job *structs.Job, namespace string, serviceName string, port int32, protocol string, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Add Port to Service", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Add Port '%d'.", port), c)

		service := ServiceFor(namespace, serviceName)
		if service != nil {
			var kubeProvider *KubeProvider
			var err error
			if !utils.CONFIG.Kubernetes.RunInCluster {
				kubeProvider, err = NewKubeProviderLocal()
			} else {
				kubeProvider, err = NewKubeProviderInCluster()
			}
			if err != nil {
				cmd.Fail(fmt.Sprintf("AddPortToService ERROR: %s", err.Error()), c)
				return
			}

			service.Spec.Ports = append(service.Spec.Ports, v1.ServicePort{
				Name:       fmt.Sprintf("%d-%s", port, serviceName),
				Port:       port,
				Protocol:   v1.Protocol(protocol),
				TargetPort: intstr.FromInt(int(port)),
			})

			serviceClient := kubeProvider.ClientSet.CoreV1().Services(namespace)
			_, err = serviceClient.Update(context.TODO(), service, metav1.UpdateOptions{})
			if err != nil {
				cmd.Fail(fmt.Sprintf("AddPortToService ERROR: %s", err.Error()), c)
				return
			}
			cmd.Success(fmt.Sprintf("Port %d added successfully removed.", port), c)
			return
		}
		cmd.Fail(fmt.Sprintf("Service '%s/%s' not found.", namespace, serviceName), c)
	}(cmd, wg)
	return cmd
}

func ServiceFor(namespace string, serviceName string) *v1.Service {
	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}

	if err != nil {
		logger.Log.Errorf("ServiceFor ERROR: %s", err.Error())
		return nil
	}

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

	var provider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		provider, err = NewKubeProviderLocal()
	} else {
		provider, err = NewKubeProviderInCluster()
	}
	if err != nil {
		logger.Log.Errorf("AllServices ERROR: %s", err.Error())
		return result
	}

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

func UpdateK8sService(data v1.Service) K8sWorkloadResult {
	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}

	if err != nil {
		return WorkloadResult(err.Error())
	}

	serviceClient := kubeProvider.ClientSet.CoreV1().Services(data.ObjectMeta.Namespace)
	_, err = serviceClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func generateService(stage dtos.K8sStageDto, service dtos.K8sServiceDto) v1.Service {
	newService := utils.InitService()
	newService.ObjectMeta.Name = service.K8sName
	newService.ObjectMeta.Namespace = stage.K8sName
	newService.Spec.Ports = []v1.ServicePort{} // reset before using
	for _, port := range service.Ports {
		if port.PortType == "HTTPS" {
			newService.Spec.Ports = append(newService.Spec.Ports, v1.ServicePort{
				Port: int32(port.InternalPort),
				Name: fmt.Sprintf("%d-%s", port.InternalPort, service.K8sName),
			})
		} else {
			newService.Spec.Ports = append(newService.Spec.Ports, v1.ServicePort{
				Port:     int32(port.InternalPort),
				Name:     fmt.Sprintf("%d-%s", port.InternalPort, service.K8sName),
				Protocol: v1.Protocol(port.PortType),
			})
			if port.ExternalPort != 0 {
				newService.Spec.Ports = append(newService.Spec.Ports, v1.ServicePort{
					Port:     int32(port.ExternalPort),
					Name:     fmt.Sprintf("%d-%s", port.ExternalPort, service.K8sName),
					Protocol: v1.Protocol(port.PortType),
				})
			}
		}
	}
	newService.Spec.Selector["app"] = service.K8sName

	return newService
}
