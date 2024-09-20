package kubernetes

import (
	"mogenius-k8s-manager/dtos"
	"testing"

	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestCreateNetworkPolicyServiceWithLabel(t *testing.T) {
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
		Type: dtos.Ingress,
		Ports: []v1.NetworkPolicyPort{
			{
				Protocol: coreV1.ProtocolTCP,
				Port:     intstr.FromInt(80),
			},
		},
	}
	err := CreateNetworkPolicyServiceWithLabel(namespace, service, labelPolicy)
	if err != nil {
		t.Errorf("Error creating network policy: %s", err.Error())
	}
}
