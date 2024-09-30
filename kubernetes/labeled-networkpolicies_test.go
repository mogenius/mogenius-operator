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
	var ports1 = []dtos.K8sLabeledPortDto{
		{
			Port:     80,
			PortType: dtos.PortTypeHTTPS,
		},
		{
			Port:     443,
			PortType: dtos.PortTypeTCP,
		},
	}

	var labelPolicy1 = dtos.K8sLabeledNetworkPolicyParams{
		Name:  PolicyName1,
		Type:  dtos.Ingress,
		Ports: ports1,
	}
	err := CreateNetworkPolicyWithLabel(namespace, labelPolicy1)
	if err != nil {
		t.Errorf("Error creating network policy: %s", err.Error())
	}

	var ports2 = []dtos.K8sLabeledPortDto{
		{
			Port:     13333,
			PortType: dtos.PortTypeSCTP,
		},
		{
			Port:     59999,
			PortType: dtos.PortTypeUDP,
		},
	}

	var labelPolicy2 = dtos.K8sLabeledNetworkPolicyParams{
		Name:  PolicyName2,
		Type:  dtos.Egress,
		Ports: ports2,
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
