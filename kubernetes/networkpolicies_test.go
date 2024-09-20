package kubernetes

import (
	"mogenius-k8s-manager/dtos"
	"testing"
)

func CreateNetworkPolicyServiceWithLabelTest(t *testing.T) {
	var namespace = dtos.K8sNamespaceDto{
		Name:        "mogenius",
		Id:          "mogenius-123",
		DisplayName: "Mogenius 123",
	}

	var service = dtos.K8sServiceDto{
		Id:                 "mogenius-service-123",
		DisplayName:        "Mogenius Service 123",
		ControllerName:     "mo-123",
		Controller:         dtos.DEPLOYMENT,
		ReplicaCount:       1,
		DeploymentStrategy: "recreate",
		Ports: []dtos.K8sPortsDto{
			dtos.K8sPortsDtoExampleData(),
			dtos.K8sPortsDtoExternalExampleData(),
		},
	}
	var labelPolicy = dtos.LabeledNetworkPolicyParams{
		Name: "mogenius-policy-123",
		Type: dtos.Egress,
		Ports: []dtos.Port{
			{
				Protocol: "TCP",
				Port:     80,
			},
			{
				Protocol: "UDP",
				Port:     12345,
			},
		},
	}
	CreateNetworkPolicyServiceWithLabel(namespace, service, labelPolicy)
}
