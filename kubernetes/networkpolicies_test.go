package kubernetes

import (
	"mogenius-k8s-manager/dtos"
	"testing"
)

const (
	PolicyName1 = "mogenius-policy-123"
	PolicyName2 = "mogenius-policy-098"
)

func TestCreateNetworkPolicyServiceWithLabel(t *testing.T) {
	var namespace = dtos.K8sNamespaceDto{
		Name:        "mogenius",
		Id:          "mogenius-123",
		DisplayName: "Mogenius 123",
	}

	var ports = []dtos.K8sPortsDto{
		dtos.K8sPortsDtoExampleData(),
		dtos.K8sPortsDtoExternalExampleData(),
	}

	var labelPolicy1 = dtos.K8sLabeledNetworkPolicyParams{
		Name:  PolicyName1,
		Type:  dtos.Ingress,
		Ports: ports,
	}

	ports[0].PortType = dtos.PortTypeUDP
	ports[1].PortType = dtos.PortTypeSCTP

	var labelPolicy2 = dtos.K8sLabeledNetworkPolicyParams{
		Name:  PolicyName2,
		Type:  dtos.Egress,
		Ports: ports,
	}
	err := CreateNetworkPolicyWithLabel(namespace, labelPolicy1)
	if err != nil {
		t.Errorf("Error creating network policy: %s", err.Error())
	}
	err = CreateNetworkPolicyWithLabel(namespace, labelPolicy2)
	if err != nil {
		t.Errorf("Error creating network policy: %s", err.Error())
	}
}

func TestDeleteNetworkPolicy(t *testing.T) {
	var namespace = dtos.K8sNamespaceDto{
		Name:        "mogenius",
		Id:          "mogenius-123",
		DisplayName: "Mogenius 123",
	}

	err := DeleteNetworkPolicy(namespace, PolicyName1)
	if err != nil {
		t.Errorf("Error deleting network policy: %s. %s", PolicyName1, err.Error())
	}
	err = DeleteNetworkPolicy(namespace, PolicyName2)
	if err != nil {
		t.Errorf("Error deleting network policy: %s. %s", PolicyName2, err.Error())
	}
}

func TestInitNetworkPolicyConfigMap(t *testing.T) {
	err := InitNetworkPolicyConfigMap()
	if err != nil {
		t.Errorf("Error initializing network policy config map: %s", err.Error())
	}
}
