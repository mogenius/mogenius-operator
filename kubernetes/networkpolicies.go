package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	iacmanager "mogenius-k8s-manager/iac-manager"
	"mogenius-k8s-manager/structs"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	v1Core "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
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

func CreateNetworkPolicyWithLabel(namespace dtos.K8sNamespaceDto, labelPolicy dtos.K8sLabeledNetworkPolicyParams) error {
	netpol := punqUtils.InitNetPolService()
	// clean traffic rules
	netpol.Spec.Ingress = []v1.NetworkPolicyIngressRule{}
	netpol.Spec.Egress = []v1.NetworkPolicyEgressRule{}

	netpol.ObjectMeta.Name = labelPolicy.Name
	netpol.ObjectMeta.Namespace = namespace.Name
	label := fmt.Sprintf("mo-netpol-%s-%s", labelPolicy.Name, labelPolicy.Type)
	netpol.Spec.PodSelector.MatchLabels[label] = "true"

	for _, aPort := range labelPolicy.Ports {
		port := intstr.FromInt32(int32(aPort.ExternalPort))
		var proto v1Core.Protocol

		switch aPort.PortType {
		case "UDP":
			proto = v1Core.ProtocolUDP
		case "SCTP":
			proto = v1Core.ProtocolSCTP
		default:
			proto = v1Core.ProtocolTCP
		}

		if labelPolicy.Type == dtos.Ingress {
			var rule v1.NetworkPolicyIngressRule = v1.NetworkPolicyIngressRule{}
			rule.From = append(rule.From, v1.NetworkPolicyPeer{
				IPBlock: &v1.IPBlock{
					CIDR: "0.0.0.0/0",
				},
			})
			rule.Ports = append(rule.Ports, v1.NetworkPolicyPort{
				Port: &port, Protocol: &proto,
			})
			netpol.Spec.Ingress = append(netpol.Spec.Ingress, rule)
		} else {
			var rule v1.NetworkPolicyEgressRule = v1.NetworkPolicyEgressRule{}
			rule.To = append(rule.To, v1.NetworkPolicyPeer{
				IPBlock: &v1.IPBlock{
					CIDR: "0.0.0.0/0",
				},
			})
			rule.Ports = append(rule.Ports, v1.NetworkPolicyPort{
				Port: &port, Protocol: &proto,
			})
			netpol.Spec.Egress = append(netpol.Spec.Egress, rule)
		}
	}

	netPolClient := GetNetworkingClient().NetworkPolicies(namespace.Name)
	_, err := netPolClient.Create(context.TODO(), &netpol, MoCreateOptions())
	if err != nil {
		K8sLogger.Errorf("CreateNetworkPolicyServiceWithLabel ERROR: %s, trying to create labelPolicy %v ", err.Error(), labelPolicy)
		return err
	}
	return nil
}

func DeleteNetworkPolicy(namespace dtos.K8sNamespaceDto, name string) error {
	client := GetNetworkingClient()
	netPolClient := client.NetworkPolicies(namespace.Name)

	err := netPolClient.Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		K8sLogger.Errorf("DeleteNetworkPolicy ERROR: %s", err)
		return err
	}

	K8sLogger.Printf("Deleted NetworkPolicy: %s", name)
	return nil
}

// func DeleteNetworkPolicyNamespace(job *structs.Job, namespace dtos.K8sNamespaceDto, wg *sync.WaitGroup) {
// 	cmd := structs.CreateCommand("delete", "Delete NetworkPolicy.", job)
// 	wg.Add(1)
// 	go func(wg *sync.WaitGroup) {
// 		defer wg.Done()
// 		cmd.Start(job, "Delete NetworkPolicy")

// 		provider, err := punq.NewKubeProvider(nil)
// 		if err != nil {
// 			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
// 			return
// 		}
// 		netPolClient := provider.ClientSet.NetworkingV1().NetworkPolicies(namespace.Name)

// 		err = netPolClient.Delete(context.TODO(), namespace.Name, metav1.DeleteOptions{})
// 		if err != nil {
// 			cmd.Fail(job, fmt.Sprintf("DeleteNetworkPolicyNamespace ERROR: %s", err.Error()))
// 		} else {
// 			cmd.Success(job, "Delete NetworkPolicy")
// 		}
// 	}(wg)
// }

func CreateOrUpdateNetworkPolicyService(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("create", "Create NetworkPolicy Service", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Creating NetworkPolicy")

		client := GetNetworkingClient()
		netPolClient := client.NetworkPolicies(namespace.Name)
		netpol := punqUtils.InitNetPolService()
		netpol.ObjectMeta.Name = service.ControllerName
		netpol.ObjectMeta.Namespace = namespace.Name
		netpol.Spec.Ingress[0].Ports = []v1.NetworkPolicyPort{} //reset before using

		for _, aPort := range service.Ports {
			if aPort.Expose {
				port := intstr.FromInt32(int32(aPort.InternalPort))
				proto := v1Core.ProtocolTCP // default
				if aPort.PortType == dtos.PortTypeUDP {
					proto = v1Core.ProtocolUDP
				}
				netpol.Spec.Ingress[0].Ports = append(netpol.Spec.Ingress[0].Ports, v1.NetworkPolicyPort{
					Port: &port, Protocol: &proto,
				})
			}
		}

		// TODO REMOVE
		//for _, container := range service.Containers {
		//	for _, aPort := range container.Ports {
		//		if aPort.Expose {
		//			port := intstr.FromInt(aPort.InternalPort)
		//			proto := v1Core.ProtocolTCP // default
		//			if aPort.PortType == dtos.PortTypeUDP {
		//				proto = v1Core.ProtocolUDP
		//			}
		//			netpol.Spec.Ingress[0].Ports = append(netpol.Spec.Ingress[0].Ports, v1.NetworkPolicyPort{
		//				Port: &port, Protocol: &proto,
		//			})
		//		}
		//	}
		//}
		netpol.Spec.PodSelector.MatchLabels["app"] = service.ControllerName

		netpol.Labels = MoUpdateLabels(&netpol.Labels, nil, nil, &service)

		_, err := netPolClient.Create(context.TODO(), &netpol, MoCreateOptions())
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				cmd.Success(job, fmt.Sprintf("NetworkPolicy already exists '%s'.", service.ControllerName))
			} else {
				cmd.Fail(job, fmt.Sprintf("CreateNetworkPolicyService ERROR: %s", err.Error()))
			}
		} else {
			cmd.Success(job, "Created NetworkPolicy")
		}
	}(wg)
}

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

func WatchNetworkPolicies() {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		K8sLogger.Fatalf("Error creating provider for watcher. Cannot continue because it is vital: %s", err.Error())
		return
	}

	// Retry watching resources with exponential backoff in case of failures
	err = retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}, apierrors.IsServiceUnavailable, func() error {
		return watchNetworkPolicies(provider, "networkpolicies")
	})
	if err != nil {
		K8sLogger.Fatalf("Error watching networkpolicies: %s", err.Error())
	}

	// Wait forever
	select {}
}

func watchNetworkPolicies(provider *punq.KubeProvider, kindName string) error {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			castedObj := obj.(*v1.NetworkPolicy)
			castedObj.Kind = "NetworkPolicy"
			castedObj.APIVersion = "networking.k8s.io/v1"
			iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			castedObj := newObj.(*v1.NetworkPolicy)
			castedObj.Kind = "NetworkPolicy"
			castedObj.APIVersion = "networking.k8s.io/v1"
			iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		DeleteFunc: func(obj interface{}) {
			castedObj := obj.(*v1.NetworkPolicy)
			castedObj.Kind = "NetworkPolicy"
			castedObj.APIVersion = "networking.k8s.io/v1"
			iacmanager.DeleteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, obj)
		},
	}
	listWatch := cache.NewListWatchFromClient(
		provider.ClientSet.NetworkingV1().RESTClient(),
		kindName,
		v1Core.NamespaceAll,
		fields.Nothing(),
	)
	resourceInformer := cache.NewSharedInformer(listWatch, &v1.NetworkPolicy{}, 0)
	_, err := resourceInformer.AddEventHandler(handler)
	if err != nil {
		return err
	}

	stopCh := make(chan struct{})
	go resourceInformer.Run(stopCh)

	// Wait for the informer to sync and start processing events
	if !cache.WaitForCacheSync(stopCh, resourceInformer.HasSynced) {
		return fmt.Errorf("failed to sync cache")
	}

	// This loop will keep the function alive as long as the stopCh is not closed
	for {
		select {
		case <-stopCh:
			// stopCh closed, return from the function
			return nil
		case <-time.After(30 * time.Second):
			// This is to avoid a tight loop in case stopCh is never closed.
			// You can adjust the time as per your needs.
		}
	}
}
