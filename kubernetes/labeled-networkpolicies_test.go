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

	var labelPolicy1 = dtos.K8sLabeledNetworkPolicyDto{
		Name:     PolicyName1,
		Type:     dtos.Ingress,
		Port:     80,
		PortType: dtos.PortTypeHTTPS,
	}
	err := EnsureLabeledNetworkPolicy(namespaceName, labelPolicy1)
	if err != nil {
		t.Errorf("Error creating network policy: %s", err.Error())
	}

	var labelPolicy2 = dtos.K8sLabeledNetworkPolicyDto{
		Name:     PolicyName2,
		Type:     dtos.Egress,
		Port:     59999,
		PortType: dtos.PortTypeUDP,
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
	// check if ports contains a imap named port fo egress
	var found bool
	for _, port := range ports {
		if port.Name == "imap" && port.Type == dtos.Ingress && port.Port == 143 && port.PortType == dtos.PortTypeTCP {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Error reading network policy ports")
	}

}
