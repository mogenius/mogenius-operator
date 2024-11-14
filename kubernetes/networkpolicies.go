package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/structs"
	"sync"
	"time"

	punqUtils "github.com/mogenius/punq/utils"
	v1Core "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateNetworkPolicyNamespace(job *structs.Job, namespace dtos.K8sNamespaceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("create", "Create NetworkPolicy namespace", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Creating NetworkPolicy")

		netPolClient := GetNetworkingClient().NetworkPolicies(namespace.Name)

		netpol := punqUtils.InitNetPolNamespace()
		netpol.ObjectMeta.Name = namespace.Name
		netpol.ObjectMeta.Namespace = namespace.Name

		netpol.Spec.PodSelector.MatchLabels["ns"] = namespace.Name
		netpol.Spec.Ingress[0].From[0].PodSelector.MatchLabels["ns"] = namespace.Name

		netpol.Labels = MoUpdateLabels(&netpol.Labels, nil, nil, nil)

		_, err := netPolClient.Create(context.TODO(), &netpol, MoCreateOptions())
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("CreateNetworkPolicyNamespace ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Created NetworkPolicy")
		}
	}(wg)
}

func DeleteNetworkPolicy(namespaceName, name string) error {
	client := GetNetworkingClient()
	netPolClient := client.NetworkPolicies(namespaceName)

	err := netPolClient.Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		k8sLogger.Error("DeleteNetworkPolicy", "networkpolicy", name, "error", err)
		return err
	}

	k8sLogger.Info("Deleted NetworkPolicy", "networkpolicy", name)

	cleanupUnusedDenyAll(namespaceName)
	return nil
}

// func CreateOrUpdateNetworkPolicyService(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
// 	cmd := structs.CreateCommand("create", "Create NetworkPolicy Service", job)
// 	wg.Add(1)
// 	go func(wg *sync.WaitGroup) {
// 		defer wg.Done()
// 		cmd.Start(job, "Creating NetworkPolicy")

// 		client := GetNetworkingClient()
// 		netPolClient := client.NetworkPolicies(namespace.Name)
// 		netpol := punqUtils.InitNetPolService()
// 		netpol.ObjectMeta.Name = service.ControllerName
// 		netpol.ObjectMeta.Namespace = namespace.Name
// 		netpol.Spec.Ingress[0].Ports = []v1.NetworkPolicyPort{} //reset before using

// 		for _, aPort := range service.Ports {
// 			if aPort.Expose {
// 				port := intstr.FromInt32(int32(aPort.InternalPort))
// 				proto := v1Core.ProtocolTCP // default
// 				if aPort.PortType == dtos.PortTypeUDP {
// 					proto = v1Core.ProtocolUDP
// 				}
// 				netpol.Spec.Ingress[0].Ports = append(netpol.Spec.Ingress[0].Ports, v1.NetworkPolicyPort{
// 					Port: &port, Protocol: &proto,
// 				})
// 			}
// 		}

// 		// TODO REMOVE
// 		//for _, container := range service.Containers {
// 		//	for _, aPort := range container.Ports {
// 		//		if aPort.Expose {
// 		//			port := intstr.FromInt(aPort.InternalPort)
// 		//			proto := v1Core.ProtocolTCP // default
// 		//			if aPort.PortType == dtos.PortTypeUDP {
// 		//				proto = v1Core.ProtocolUDP
// 		//			}
// 		//			netpol.Spec.Ingress[0].Ports = append(netpol.Spec.Ingress[0].Ports, v1.NetworkPolicyPort{
// 		//				Port: &port, Protocol: &proto,
// 		//			})
// 		//		}
// 		//	}
// 		//}
// 		netpol.Spec.PodSelector.MatchLabels["app"] = service.ControllerName

// 		netpol.Labels = MoUpdateLabels(&netpol.Labels, nil, nil, &service)

// 		_, err := netPolClient.Create(context.TODO(), &netpol, MoCreateOptions())
// 		if err != nil {
// 			if apierrors.IsAlreadyExists(err) {
// 				cmd.Success(job, fmt.Sprintf("NetworkPolicy already exists '%s'.", service.ControllerName))
// 			} else {
// 				cmd.Fail(job, fmt.Sprintf("CreateNetworkPolicyService ERROR: %s", err.Error()))
// 			}
// 		} else {
// 			cmd.Success(job, "Created NetworkPolicy")
// 		}
// 	}(wg)
// }

func DeleteNetworkPolicyService(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", "Delete NetworkPolicy Service.", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Delete NetworkPolicy")

		client := GetNetworkingClient()
		netPolClient := client.NetworkPolicies(namespace.Name)

		err := netPolClient.Delete(context.TODO(), service.ControllerName, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("DeleteNetworkPolicyService ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Delete NetworkPolicy")
		}
	}(wg)
}

func HandleNetworkPolicyChange(netPol *v1.NetworkPolicy, reason string) {
	annotations := createAnnotations("mogenius.io/created", time.Now().String())
	// create a new event
	event := &v1Core.Event{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   netPol.Namespace,
			Annotations: annotations,
		},
		InvolvedObject: v1Core.ObjectReference{
			Kind:            "NetworkPolicy",
			Namespace:       netPol.Namespace,
			Name:            netPol.Name,
			UID:             netPol.UID,
			ResourceVersion: netPol.ResourceVersion,
		},
		Reason:  reason,
		Message: fmt.Sprintf("NetPol %s is being %s", netPol.Name, reason),
		Type:    v1Core.EventTypeNormal,
	}
	ProcessEvent(event)
}

func createAnnotations(items ...string) map[string]string {
	if len(items)%2 != 0 {
		return nil
	}

	annotations := make(map[string]string)

	for i := 0; i < len(items); i += 2 {
		key := items[i]
		value := items[i+1]
		annotations[key] = value
	}

	return annotations
}
