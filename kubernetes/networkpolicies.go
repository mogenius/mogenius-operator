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

	v1Core "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func CreateNetworkPolicyNamespace(job *structs.Job, stage dtos.K8sStageDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create NetworkPolicy namespace", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating NetworkPolicy '%s'.", stage.Name))

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
			cmd.Fail(fmt.Sprintf("CreateNetworkPolicyNamespace ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Created NetworkPolicy '%s'.", stage.Name))
		}
	}(cmd, wg)
	return cmd
}

func DeleteNetworkPolicyNamespace(job *structs.Job, stage dtos.K8sStageDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete NetworkPolicy.", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Delete NetworkPolicy '%s'.", stage.Name))

		kubeProvider := NewKubeProvider()
		netPolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(stage.Name)

		err := netPolClient.Delete(context.TODO(), stage.Name, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteNetworkPolicyNamespace ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Delete NetworkPolicy '%s'.", stage.Name))
		}
	}(cmd, wg)
	return cmd
}

func CreateNetworkPolicyService(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create NetworkPolicy Service", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating NetworkPolicy '%s'.", service.Name))

		kubeProvider := NewKubeProvider()
		netPolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(stage.Name)
		netpol := utils.InitNetPolService()
		netpol.ObjectMeta.Name = service.Name
		netpol.ObjectMeta.Namespace = stage.Name
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

		netpol.Labels = MoUpdateLabels(&netpol.Labels, &job.NamespaceId, &stage, &service)

		_, err := netPolClient.Create(context.TODO(), &netpol, MoCreateOptions())
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateNetworkPolicyService ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Created NetworkPolicy '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}

func DeleteNetworkPolicyService(job *structs.Job, stage dtos.K8sStageDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete NetworkPolicy Service.", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Delete NetworkPolicy '%s'.", service.Name))

		kubeProvider := NewKubeProvider()
		netPolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(stage.Name)

		err := netPolClient.Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteNetworkPolicyService ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Delete NetworkPolicy '%s'.", service.Name))
		}
	}(cmd, wg)
	return cmd
}

func AllNetworkPolicies(namespaceName string) []v1.NetworkPolicy {
	result := []v1.NetworkPolicy{}

	provider := NewKubeProvider()
	netPolist, err := provider.ClientSet.NetworkingV1().NetworkPolicies(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllNetworkPolicies ERROR: %s", err.Error())
		return result
	}

	for _, netpol := range netPolist.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, netpol.ObjectMeta.Namespace) {
			result = append(result, netpol)
		}
	}
	return result
}

func UpdateK8sNetworkPolicy(data v1.NetworkPolicy) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	netpolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(data.Namespace)
	_, err := netpolClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DeleteK8sNetworkPolicy(data v1.NetworkPolicy) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	netpolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(data.Namespace)
	err := netpolClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DescribeK8sNetworkPolicy(namespace string, name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "netpol", name, "-n", namespace)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		return WorkloadResult(err.Error())
	}
	return WorkloadResult(string(output))
}
