package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/shutdown"
	"mogenius-k8s-manager/structs"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"
	scheme "k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	v1Core "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// TODO: this has realy bad performance, need to find a better way to do this
func HandleNetworkPolicyChange(netPol *v1.NetworkPolicy, reason string) {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		k8sLogger.Error("Error creating provider for netpol watcher. Cannot continue because it is vital.", "error", err)
		shutdown.SendShutdownSignalAndBlockForever(true)
		select {}
	}

	// Set up a dynamic event broadcaster for the specific namespace
	broadcaster := record.NewBroadcaster()
	eventInterface := provider.ClientSet.CoreV1().Events(netPol.Namespace)
	broadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: eventInterface})
	netPolRecoderLogger := broadcaster.NewRecorder(scheme.Scheme, v1Core.EventSource{Component: "mogenius.io/WatchNetworkPolicies"})

	annotations := createAnnotations("mogenius.io/created", time.Now().String())

	// Trigger custom event
	k8sLogger.Debug("Netpol is being updated in namespace, triggering event", "netpol", netPol.Name, "namespace", netPol.Namespace)
	netPolRecoderLogger.AnnotatedEventf(netPol, annotations, v1Core.EventTypeNormal, reason, "NetPol %s is being %s", netPol.Name, reason)
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
