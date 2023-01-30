package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"sync"

	"github.com/gorilla/websocket"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateService(job *utils.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, redirectTo *string, skipForDelete *dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *utils.Command {
	cmd := utils.CreateCommand(fmt.Sprintf("Creating service '%s'.", stage.K8sName), job, c)
	wg.Add(1)
	go func(cmd *utils.Command, wg *sync.WaitGroup) {
		defer wg.Done()

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}

		if err != nil {
			logger.Log.Errorf("CreateService ERROR: %s", err.Error())
		}

		serviceClient := kubeProvider.ClientSet.CoreV1().Services(stage.K8sName)
		newService := utils.InitService()
		newService.ObjectMeta.Name = service.K8sName
		newService.ObjectMeta.Namespace = stage.K8sName
		newService.Spec.Ports = []v1.ServicePort{}
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

func DeleteService(job *utils.Job, stage dtos.K8sStageDto, c *websocket.Conn, wg *sync.WaitGroup) *utils.Command {
	cmd := utils.CreateCommand("Delete Service", job, c)
	wg.Add(1)
	go func(cmd *utils.Command, wg *sync.WaitGroup) {
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
			logger.Log.Errorf("DeleteService ERROR: %s", err.Error())
		}

		serviceClient := kubeProvider.ClientSet.CoreV1().Services(stage.K8sName)

		err = serviceClient.Delete(context.TODO(), stage.K8sName, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteService ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Deleted service '%s'.", stage.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}
