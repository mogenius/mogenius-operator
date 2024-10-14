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
	var namespaceName = "mogenius"

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

	var labelPolicy1 = dtos.K8sLabeledNetworkPolicyDto{
		Name:  PolicyName1,
		Type:  dtos.Ingress,
		Ports: ports1,
	}
	err := EnsureLabeledNetworkPolicy(namespaceName, labelPolicy1)
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

	var labelPolicy2 = dtos.K8sLabeledNetworkPolicyDto{
		Name:  PolicyName2,
		Type:  dtos.Egress,
		Ports: ports2,
	}

	err = EnsureLabeledNetworkPolicy(namespaceName, labelPolicy2)
	if err != nil {
		t.Errorf("Error creating network policy: %s", err.Error())
	}
}

func TestDeleteNetworkPolicy(t *testing.T) {
	var namespaceName = "mogenius"

	err := DeleteNetworkPolicy(namespaceName, PolicyName1)
	if err != nil {
		t.Errorf("Error deleting network policy: %s. %s", PolicyName1, err.Error())
	}
	err = DeleteNetworkPolicy(namespaceName, PolicyName2)
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

func TestReadNetworkPolicyPorts(t *testing.T) {
	ports := ReadNetworkPolicyPorts()
	if len(ports) == 0 {
		t.Errorf("Error reading network policy ports")
	}
	// t.Logf("Ports: %v\n", ports)
}
