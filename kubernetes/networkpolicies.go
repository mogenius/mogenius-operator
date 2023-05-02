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
		cmd.Start(fmt.Sprintf("Creating NetworkPolicy '%s'.", stage.Name), c)

		kubeProvider := NewKubeProvider()
		netPolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(stage.Name)
		netpol := utils.InitNetPolNamespace()
		netpol.ObjectMeta.Name = stage.Name
		netpol.ObjectMeta.Namespace = stage.Name

		netpol.Spec.PodSelector.MatchLabels["ns"] = stage.Name
		netpol.Spec.Ingress[0].From[0].PodSelector.MatchLabels["ns"] = stage.Name

		netpol.Labels = MoUpdateLabels(&netpol.Labels, &job.NamespaceId, &stage, nil)

		_, err := netPolClient.Create(context.TODO(), &netpol, MoCreateOptions())
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateNetworkPolicyNamespace ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Created NetworkPolicy '%s'.", stage.Name), c)
		}
	}(cmd, wg)
	return cmd
}

func DeleteNetworkPolicyNamespace(job *structs.Job, stage dtos.K8sStageDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete NetworkPolicy.", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Delete NetworkPolicy '%s'.", stage.Name), c)

		kubeProvider := NewKubeProvider()
		netPolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(stage.Name)

		err := netPolClient.Delete(context.TODO(), stage.Name, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteNetworkPolicyNamespace ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Delete NetworkPolicy '%s'.", stage.Name), c)
		}
	}(cmd, wg)
	return cmd
}

func CreateNetworkPolicyService(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create NetworkPolicy Service", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating NetworkPolicy '%s'.", service.Name), c)

		kubeProvider := NewKubeProvider()
		netPolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(stage.Name)
		netpol := utils.InitNetPolService()
		netpol.ObjectMeta.Name = service.Name
		netpol.ObjectMeta.Namespace = stage.Name
		netpol.Spec.Ingress[0].Ports = []v1.NetworkPolicyPort{} //reset before using

		for _, aPort := range service.Ports {
			if aPort.Expose {
				port := intstr.FromInt(aPort.InternalPort)
				netpol.Spec.Ingress[0].Ports = append(netpol.Spec.Ingress[0].Ports, v1.NetworkPolicyPort{
					Port: &port,
				})
			}
		}
		netpol.Spec.PodSelector.MatchLabels["app"] = service.Name

		netpol.Labels = MoUpdateLabels(&netpol.Labels, &job.NamespaceId, &stage, &service)

		_, err := netPolClient.Create(context.TODO(), &netpol, MoCreateOptions())
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateNetworkPolicyService ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Created NetworkPolicy '%s'.", service.Name), c)
		}
	}(cmd, wg)
	return cmd
}

func DeleteNetworkPolicyService(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete NetworkPolicy Service.", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Delete NetworkPolicy '%s'.", service.Name), c)

		kubeProvider := NewKubeProvider()
		netPolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(stage.Name)

		err := netPolClient.Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteNetworkPolicyService ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Delete NetworkPolicy '%s'.", service.Name), c)
		}
	}(cmd, wg)
	return cmd
}
