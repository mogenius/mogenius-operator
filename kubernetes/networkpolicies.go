package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

	"github.com/gorilla/websocket"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func CreateNetworkPolicyNamespace(job *structs.Job, stage dtos.K8sStageDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create NetworkPolicy namespace", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating NetworkPolicy '%s'.", stage.K8sName), c)

		kubeProvider := NewKubeProvider()
		netPolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(stage.K8sName)
		netpol := utils.InitNetPolNamespace()
		netpol.ObjectMeta.Name = stage.K8sName
		netpol.ObjectMeta.Namespace = stage.K8sName

		netpol.Spec.PodSelector.MatchLabels["ns"] = stage.K8sName
		netpol.Spec.Ingress[0].From[0].PodSelector.MatchLabels["ns"] = stage.K8sName

		_, err := netPolClient.Create(context.TODO(), &netpol, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateNetworkPolicyNamespace ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Created NetworkPolicy '%s'.", stage.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func DeleteNetworkPolicyNamespace(job *structs.Job, stage dtos.K8sStageDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete NetworkPolicy.", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Delete NetworkPolicy '%s'.", stage.K8sName), c)

		kubeProvider := NewKubeProvider()
		netPolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(stage.K8sName)

		err := netPolClient.Delete(context.TODO(), stage.K8sName, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteNetworkPolicyNamespace ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Delete NetworkPolicy '%s'.", stage.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func CreateNetworkPolicyService(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create NetworkPolicy Service", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating NetworkPolicy '%s'.", service.K8sName), c)

		kubeProvider := NewKubeProvider()
		netPolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(stage.K8sName)
		netpol := utils.InitNetPolService()
		netpol.ObjectMeta.Name = service.K8sName
		netpol.ObjectMeta.Namespace = stage.K8sName
		netpol.Spec.Ingress[0].Ports = []v1.NetworkPolicyPort{} //reset before using

		for _, aPort := range service.Ports {
			if aPort.Expose {
				port := intstr.FromInt(aPort.InternalPort)
				netpol.Spec.Ingress[0].Ports = append(netpol.Spec.Ingress[0].Ports, v1.NetworkPolicyPort{
					Port: &port,
				})
			}
		}
		netpol.Spec.PodSelector.MatchLabels["app"] = service.K8sName

		_, err := netPolClient.Create(context.TODO(), &netpol, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateNetworkPolicyService ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Created NetworkPolicy '%s'.", service.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func DeleteNetworkPolicyService(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete NetworkPolicy Service.", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Delete NetworkPolicy '%s'.", service.K8sName), c)

		kubeProvider := NewKubeProvider()
		netPolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(stage.K8sName)

		err := netPolClient.Delete(context.TODO(), service.K8sName, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteNetworkPolicyService ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Delete NetworkPolicy '%s'.", service.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}
