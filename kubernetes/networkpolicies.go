package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

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

var netPolRecoderLogger record.EventRecorderLogger

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
		K8sLogger.Errorf("DeleteNetworkPolicy ERROR: %s", err)
		return err
	}

	K8sLogger.Printf("Deleted NetworkPolicy: %s", name)

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

func HandleNetworkPolicyChange(netPol *v1.NetworkPolicy, reason string) {
	if netPolRecoderLogger == nil {
		provider, err := punq.NewKubeProvider(nil)
		if provider == nil || err != nil {
			K8sLogger.Fatalf("Error creating provider for netpol watcher. Cannot continue because it is vital: %s", err.Error())
			return
		}

		// Set up a dynamic event broadcaster for the specific namespace
		broadcaster := record.NewBroadcaster()
		eventInterface := provider.ClientSet.CoreV1().Events(netPol.Namespace)
		broadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: eventInterface})
		netPolRecoderLogger = broadcaster.NewRecorder(scheme.Scheme, v1Core.EventSource{Component: "mogenius.io/WatchNetworkPolicies"})
	}

	// Create a new event and add custom annotations
	event := &v1Core.Event{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: netPol.Namespace,
			Name:      netPol.Name,
		},
		InvolvedObject: v1Core.ObjectReference{
			Kind:       "NetworkPolicy",
			Namespace:  netPol.Namespace,
			Name:       netPol.Name,
			UID:        netPol.UID,
			APIVersion: "networking.k8s.io/v1",
		},
		Message: fmt.Sprintf("NetPol %s is being %s", netPol.Name, reason),
		Type:    v1Core.EventTypeNormal,
		Reason:  reason,
		Source:  v1Core.EventSource{Component: "mogenius.io/WatchNetworkPolicies"},
	}

	// Add custom annotations for auth to the event
	err := addEventAnnotations(event,
		"x-authorization", utils.CONFIG.Kubernetes.ApiKey,
		"x-cluster-mfa-id", utils.CONFIG.Kubernetes.ClusterMfaId)
	if err != nil {
		K8sLogger.Errorf("Failed to add annotations to the event: %v", err)
	}

	// Trigger the custom event
	K8sLogger.Debugf("Netpol %s is being updated in namespace %s, triggering event", netPol.Name, netPol.Namespace)
	netPolRecoderLogger.Event(event, event.Type, event.Reason, event.Message)
}

// AddAnnotations adds key-value pairs as annotations to a Kubernetes event
func addEventAnnotations(event *v1Core.Event, annotations ...string) error {
	if len(annotations)%2 != 0 {
		return fmt.Errorf("annotations must be in key-value pairs")
	}

	if event.ObjectMeta.Annotations == nil {
		event.ObjectMeta.Annotations = make(map[string]string)
	}

	for i := 0; i < len(annotations); i += 2 {
		key := annotations[i]
		value := annotations[i+1]
		event.ObjectMeta.Annotations[key] = value
	}

	return nil
}
