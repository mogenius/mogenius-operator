package controllers

import (
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/kubernetes"
)

type CreateLabeledNetworkPolicyRequest struct {
	Namespace            dtos.K8sNamespaceDto         `json:"namespace" validate:"required"`
	LabeledNetworkPolicy dtos.K8sLabeledNetworkPolicy `json:"labeledNetworkPolicy" validate:"required"`
}

type CreateLabeledNetworkPolicyResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage"`
}

type LabeledNetworkPoliciesListResponse []dtos.K8sLabeledNetworkPolicy

func ListLabeledNetworkPolicyPortsExample() LabeledNetworkPoliciesListResponse {
	return []dtos.K8sLabeledNetworkPolicy{
		{
			Name: "mogenius-policy-123",
			Type: dtos.Ingress,
			Ports: []dtos.K8sLabeledPortDto{
				{
					Port:     80,
					PortType: dtos.PortTypeHTTPS,
				},
				{
					Port:     443,
					PortType: dtos.PortTypeTCP,
				},
			},
		},
		{
			Name: "mogenius-policy-098",
			Type: dtos.Egress,
			Ports: []dtos.K8sLabeledPortDto{
				{
					Port:     13333,
					PortType: dtos.PortTypeSCTP,
				},
			},
		},
	}
}

func ListLabeledNetworkPolicyPortsRequest() LabeledNetworkPoliciesListResponse {
	return kubernetes.ReadNetworkPolicyPorts()
}
