package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/structs"
	"strings"
	"sync"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	v1Core "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func CreateNetworkPolicyNamespace(job *structs.Job, namespace dtos.K8sNamespaceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create NetworkPolicy namespace", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating NetworkPolicy '%s'.", namespace.Name))

		kubeProvider := punq.NewKubeProvider(nil)
		netPolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(namespace.Name)
		netpol := punqUtils.InitNetPolNamespace()
		netpol.ObjectMeta.Name = namespace.Name
		netpol.ObjectMeta.Namespace = namespace.Name

		netpol.Spec.PodSelector.MatchLabels["ns"] = namespace.Name
		netpol.Spec.Ingress[0].From[0].PodSelector.MatchLabels["ns"] = namespace.Name

		netpol.Labels = MoUpdateLabels(&netpol.Labels, job.ProjectId, &namespace, nil)

		_, err := netPolClient.Create(context.TODO(), &netpol, MoCreateOptions())
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateNetworkPolicyNamespace ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Created NetworkPolicy '%s'.", namespace.Name))
		}
	}(cmd, wg)
	return cmd
}

func DeleteNetworkPolicyNamespace(job *structs.Job, namespace dtos.K8sNamespaceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete NetworkPolicy.", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Delete NetworkPolicy '%s'.", namespace.Name))

		kubeProvider := punq.NewKubeProvider(nil)
		netPolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(namespace.Name)

		err := netPolClient.Delete(context.TODO(), namespace.Name, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteNetworkPolicyNamespace ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Delete NetworkPolicy '%s'.", namespace.Name))
		}
	}(cmd, wg)
	return cmd
}

func CreateNetworkPolicyService(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create NetworkPolicy Service", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating NetworkPolicy '%s'.", service.Name))

		kubeProvider := punq.NewKubeProvider(nil)
		netPolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(namespace.Name)
		netpol := punqUtils.InitNetPolService()
		netpol.ObjectMeta.Name = service.Name
		netpol.ObjectMeta.Namespace = namespace.Name
		netpol.Spec.Ingress[0].Ports = []v1.NetworkPolicyPort{} //reset before using

		for _, aPort := range service.Ports {
			if aPort.Expose {
				port := intstr.FromInt(aPort.InternalPort)
				proto := v1Core.ProtocolTCP // default
				if strings.ToLower(aPort.PortType) == "udp" {
					proto = v1Core.ProtocolUDP
				}
				netpol.Spec.Ingress[0].Ports = append(netpol.Spec.Ingress[0].Ports, v1.NetworkPolicyPort{
					Port: &port, Protocol: &proto,
				})
			}
		}
		netpol.Spec.PodSelector.MatchLabels["app"] = service.Name

		netpol.Labels = MoUpdateLabels(&netpol.Labels, job.ProjectId, &namespace, &service)

		_, err := netPolClient.Create(context.TODO(), &netpol, MoCreateOptions())
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateNetworkPolicyService ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Created NetworkPolicy '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}

func DeleteNetworkPolicyService(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete NetworkPolicy Service.", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Delete NetworkPolicy '%s'.", service.Name))

		kubeProvider := punq.NewKubeProvider(nil)
		netPolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(namespace.Name)

		err := netPolClient.Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteNetworkPolicyService ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Delete NetworkPolicy '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}
