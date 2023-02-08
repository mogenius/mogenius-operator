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
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateNetworkPoliciesNamespace(job *structs.Job, stage dtos.K8sStageDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create NetworkPolicies namespace", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating NetworkPolicies '%s'.", stage.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}
		if err != nil {
			logger.Log.Errorf("CreateNetworkPolicies ERROR: %s", err.Error())
		}

		netPolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(stage.K8sName)
		netpol := utils.InitNetPolNamespace()
		netpol.ObjectMeta.Name = stage.K8sName
		netpol.ObjectMeta.Namespace = stage.K8sName

		netpol.Spec.PodSelector.MatchLabels["ns"] = stage.K8sName
		netpol.Spec.Ingress[0].From = []v1.NetworkPolicyPeer{}
		netpol.Spec.Ingress[0].From = append(netpol.Spec.Ingress[0].From, v1.NetworkPolicyPeer{})
		netpol.Spec.Ingress[0].From[0].PodSelector = &metav1.LabelSelector{}
		netpol.Spec.Ingress[0].From[0].PodSelector.MatchLabels = map[string]string{}
		netpol.Spec.Ingress[0].From[0].PodSelector.MatchLabels["ns"] = stage.K8sName

		_, err = netPolClient.Create(context.TODO(), &netpol, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateNetworkPolicies ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Created NetworkPolicies '%s'.", stage.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func DeleteNetworkPoliciesNamespace(job *structs.Job, stage dtos.K8sStageDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete NetworkPolicies.", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Delete NetworkPolicies '%s'.", stage.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}
		if err != nil {
			logger.Log.Errorf("DeleteNetworkPolicies ERROR: %s", err.Error())
		}

		netPolClient := kubeProvider.ClientSet.NetworkingV1().NetworkPolicies(stage.K8sName)

		err = netPolClient.Delete(context.TODO(), stage.K8sName, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteNetworkPolicies ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Delete NetworkPolicies '%s'.", stage.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}
