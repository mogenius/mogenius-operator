package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/structs"
	"sync"
	"time"

	"mogenius-k8s-manager/src/utils"

	v1Core "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateNetworkPolicyNamespace(job *structs.Job, namespace dtos.K8sNamespaceDto, name string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("create", "Create NetworkPolicy namespace", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Creating NetworkPolicy")

		netPolClient := GetNetworkingClient().NetworkPolicies(namespace.Name)

		netpol := utils.InitNetPolNamespace()

		netpol.ObjectMeta.Name = name
		// netpol.ObjectMeta.Name = namespace.Name
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
			Name:        fmt.Sprintf("%s.%s.%s", netPol.Name, netPol.Namespace, time.Now().String()),
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
		Reason:    reason,
		Message:   fmt.Sprintf("NetPol %s is being %s", netPol.Name, reason),
		Type:      v1Core.EventTypeNormal,
		EventTime: metav1.MicroTime{Time: time.Now()},
	}

	k8sLogger.Debug("Sending custom network policy event to dispatcher", "event", event)

	processEvent(event)
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
